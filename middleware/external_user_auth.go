package middleware

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// ExternalUserConfig 外部用户验证配置
type ExternalUserConfig struct {
	RedisURL      string // Upstash Redis REST URL
	RedisToken    string // Upstash Redis REST Token
	JWTSecret     string // JWT 密钥 (与前端一致)
	MonthlyQuota  int    // 普通用户每月配额
	Enabled       bool   // 是否启用外部用户验证
}

var externalUserConfig = ExternalUserConfig{
	MonthlyQuota: 30,
	Enabled:      false,
}

// InitExternalUserAuth 初始化外部用户验证配置
func InitExternalUserAuth(redisURL, redisToken, jwtSecret string, monthlyQuota int) {
	externalUserConfig.RedisURL = redisURL
	externalUserConfig.RedisToken = redisToken
	externalUserConfig.JWTSecret = jwtSecret
	if monthlyQuota > 0 {
		externalUserConfig.MonthlyQuota = monthlyQuota
	}
	externalUserConfig.Enabled = redisURL != "" && redisToken != ""
	if externalUserConfig.Enabled {
		fmt.Printf("[ExternalUserAuth] 已启用外部用户验证, Redis URL: %s, 每月配额: %d\n", redisURL, externalUserConfig.MonthlyQuota)
	}
}

// ExternalUserData 外部用户数据
type ExternalUserData struct {
	ID           string `json:"id"`
	Email        string `json:"email"`
	Username     string `json:"username"`
	IsVIP        bool   `json:"isVip"`
	VIPExpiresAt int64  `json:"vipExpiresAt"`
}

// UserQuota 用户配额数据
type UserQuota struct {
	UsedCount   int    `json:"usedCount"`
	MonthKey    string `json:"monthKey"` // 格式: "2024-01"
	LastResetAt int64  `json:"lastResetAt"`
}

// ExternalUserAuth 外部用户验证中间件
func ExternalUserAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		// [调试] 记录中间件执行
		fmt.Printf("[ExternalUserAuth] ========== 开始处理请求 ==========\n")
		fmt.Printf("[ExternalUserAuth] 请求路径: %s %s\n", c.Request.Method, c.Request.URL.Path)
		fmt.Printf("[ExternalUserAuth] 配置状态: Enabled=%v, RedisURL=%s\n", externalUserConfig.Enabled, maskString(externalUserConfig.RedisURL, 20))
		
		// Redis 未配置，拒绝所有请求
		if !externalUserConfig.Enabled {
			fmt.Printf("[ExternalUserAuth] ❌ 中间件未启用 (Redis 未配置)\n")
			abortWithOpenAiMessage(c, http.StatusServiceUnavailable, "服务未正确配置，请联系管理员 (Redis 未配置)")
			return
		}

		// 从 Header 获取外部用户 Token
		externalToken := c.Request.Header.Get("X-External-User-Token")
		if externalToken == "" {
			// 没有外部用户 token，拒绝请求
			fmt.Printf("[ExternalUserAuth] ❌ 未收到 X-External-User-Token header\n")
			fmt.Printf("[ExternalUserAuth] 收到的 Headers: %v\n", getHeaderKeys(c.Request.Header))
			abortWithOpenAiMessage(c, http.StatusUnauthorized, "请先登录后再使用 API")
			return
		}
		fmt.Printf("[ExternalUserAuth] ✓ 收到 Token: %s...\n", maskString(externalToken, 30))

		// 验证 JWT Token
		userData, err := verifyExternalJWT(externalToken)
		if err != nil {
			fmt.Printf("[ExternalUserAuth] ❌ JWT 验证失败: %v\n", err)
			abortWithOpenAiMessage(c, http.StatusUnauthorized, "外部用户验证失败: "+err.Error())
			return
		}
		fmt.Printf("[ExternalUserAuth] ✓ 用户验证成功: ID=%s, Username=%s, Email=%s\n", userData.ID, userData.Username, userData.Email)

		// 检查是否是 VIP 用户
		isVIP := userData.IsVIP && userData.VIPExpiresAt > time.Now().Unix()
		
		// 检查是否是管理员
		isAdmin := userData.Username == "admin"
		
		fmt.Printf("[ExternalUserAuth] 用户状态: IsVIP=%v, VIPExpiresAt=%d, IsAdmin=%v\n", userData.IsVIP, userData.VIPExpiresAt, isAdmin)

		// VIP 和管理员不受配额限制
		if isVIP || isAdmin {
			fmt.Printf("[ExternalUserAuth] ✓ VIP/管理员用户，跳过配额检查\n")
			c.Set("external_user_id", userData.ID)
			c.Set("external_user_email", userData.Email)
			c.Set("external_user_vip", true)
			// 设置响应头，告知前端配额状态
			c.Header("X-Quota-Status", "vip")
			c.Header("X-Quota-Used", "0")
			c.Header("X-Quota-Total", "-1")
			c.Header("X-Quota-Remaining", "-1")
			c.Next()
			return
		}

		// 普通用户检查配额
		quota, err := getUserQuota(userData.ID)
		if err != nil {
			fmt.Printf("[ExternalUserAuth] ❌ 获取配额失败: %v\n", err)
			abortWithOpenAiMessage(c, http.StatusInternalServerError, "获取用户配额失败: "+err.Error())
			return
		}
		fmt.Printf("[ExternalUserAuth] 当前配额: UsedCount=%d, MonthKey=%s\n", quota.UsedCount, quota.MonthKey)

		// 检查是否需要重置配额 (新的月份)
		currentMonthKey := time.Now().Format("2006-01")
		if quota.MonthKey != currentMonthKey {
			fmt.Printf("[ExternalUserAuth] 月份变更，重置配额: %s -> %s\n", quota.MonthKey, currentMonthKey)
			quota.UsedCount = 0
			quota.MonthKey = currentMonthKey
			quota.LastResetAt = time.Now().Unix()
		}

		// 检查配额
		if quota.UsedCount >= externalUserConfig.MonthlyQuota {
			fmt.Printf("[ExternalUserAuth] ❌ 配额已用完: %d/%d\n", quota.UsedCount, externalUserConfig.MonthlyQuota)
			abortWithOpenAiMessage(c, http.StatusTooManyRequests, 
				fmt.Sprintf("本月调用次数已用完 (%d/%d)，请升级 VIP 或等待下月重置", 
					quota.UsedCount, externalUserConfig.MonthlyQuota))
			return
		}

		// 扣减配额
		oldCount := quota.UsedCount
		quota.UsedCount++
		if err := saveUserQuota(userData.ID, quota); err != nil {
			// 保存失败不阻止请求，只记录日志
			fmt.Printf("[ExternalUserAuth] ⚠️ 保存用户配额失败: %v\n", err)
		} else {
			fmt.Printf("[ExternalUserAuth] ✓ 配额扣减成功: %d -> %d (总配额: %d)\n", oldCount, quota.UsedCount, externalUserConfig.MonthlyQuota)
		}

		c.Set("external_user_id", userData.ID)
		c.Set("external_user_email", userData.Email)
		c.Set("external_user_vip", false)
		c.Set("external_user_quota_used", quota.UsedCount)
		c.Set("external_user_quota_total", externalUserConfig.MonthlyQuota)
		
		// 设置响应头，告知前端配额状态
		c.Header("X-Quota-Status", "active")
		c.Header("X-Quota-Used", strconv.Itoa(quota.UsedCount))
		c.Header("X-Quota-Total", strconv.Itoa(externalUserConfig.MonthlyQuota))
		c.Header("X-Quota-Remaining", strconv.Itoa(externalUserConfig.MonthlyQuota - quota.UsedCount))
		
		fmt.Printf("[ExternalUserAuth] ========== 请求处理完成 ==========\n")

		c.Next()
	}
}

// maskString 遮蔽字符串，只显示前 n 个字符
func maskString(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

// getHeaderKeys 获取所有 header 的 key 列表
func getHeaderKeys(h http.Header) []string {
	keys := make([]string, 0, len(h))
	for k := range h {
		keys = append(keys, k)
	}
	return keys
}

// verifyExternalJWT 验证外部 JWT Token
func verifyExternalJWT(tokenString string) (*ExternalUserData, error) {
	parts := strings.Split(tokenString, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("无效的 token 格式")
	}

	// 解码 payload
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		// 尝试标准 base64
		payload, err = base64.StdEncoding.DecodeString(parts[1])
		if err != nil {
			return nil, fmt.Errorf("无法解码 token payload")
		}
	}

	var claims map[string]interface{}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return nil, fmt.Errorf("无法解析 token payload")
	}

	// 检查过期时间
	if exp, ok := claims["exp"].(float64); ok {
		if int64(exp) < time.Now().Unix() {
			return nil, fmt.Errorf("token 已过期")
		}
	}

	// 从 Redis 获取用户数据验证
	userId, _ := claims["userId"].(string)
	email, _ := claims["email"].(string)

	if userId == "" && email == "" {
		return nil, fmt.Errorf("token 中缺少用户信息")
	}

	// 从 Redis 获取用户详细信息
	userData, err := getUserFromRedis(userId)
	if err != nil {
		// 如果 Redis 获取失败，使用 token 中的基本信息
		userData = &ExternalUserData{
			ID:    userId,
			Email: email,
		}
		if email != "" {
			userData.Username = strings.Split(email, "@")[0]
		}
	}

	return userData, nil
}

// getUserFromRedis 从 Redis 获取用户数据
func getUserFromRedis(userId string) (*ExternalUserData, error) {
	if externalUserConfig.RedisURL == "" {
		return nil, fmt.Errorf("Redis 未配置")
	}

	key := "user:" + userId
	url := fmt.Sprintf("%s/get/%s", externalUserConfig.RedisURL, key)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+externalUserConfig.RedisToken)

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result struct {
		Result interface{} `json:"result"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	if result.Result == nil {
		return nil, fmt.Errorf("用户不存在")
	}

	// Upstash 可能返回 string 或 object
	var userData ExternalUserData
	switch v := result.Result.(type) {
	case string:
		if v == "" {
			return nil, fmt.Errorf("用户不存在")
		}
		if err := json.Unmarshal([]byte(v), &userData); err != nil {
			return nil, err
		}
	case map[string]interface{}:
		// 直接是 JSON object
		jsonBytes, _ := json.Marshal(v)
		if err := json.Unmarshal(jsonBytes, &userData); err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("用户数据格式错误")
	}

	return &userData, nil
}

// getUserQuota 获取用户配额
func getUserQuota(userId string) (*UserQuota, error) {
	if externalUserConfig.RedisURL == "" {
		return &UserQuota{}, nil
	}

	// Upstash REST API: GET /get/:key (key 需要 URL 编码)
	key := "quota:" + userId
	url := fmt.Sprintf("%s/get/%s", externalUserConfig.RedisURL, key)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+externalUserConfig.RedisToken)

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result struct {
		Result interface{} `json:"result"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	quota := &UserQuota{
		MonthKey: time.Now().Format("2006-01"),
	}

	// Upstash 可能返回 string 或 null
	if result.Result != nil {
		switch v := result.Result.(type) {
		case string:
			if v != "" {
				json.Unmarshal([]byte(v), quota)
			}
		}
	}

	return quota, nil
}

// saveUserQuota 保存用户配额
func saveUserQuota(userId string, quota *UserQuota) error {
	if externalUserConfig.RedisURL == "" {
		return nil
	}

	quotaJSON, err := json.Marshal(quota)
	if err != nil {
		return err
	}

	// Upstash Redis REST API: POST with body [command, args...]
	// 使用 pipeline 方式: POST / with body ["SET", "key", "value"]
	key := "quota:" + userId
	cmdBody, _ := json.Marshal([]string{"SET", key, string(quotaJSON)})
	
	req, err := http.NewRequest("POST", externalUserConfig.RedisURL, bytes.NewReader(cmdBody))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+externalUserConfig.RedisToken)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return nil
}

// GetExternalUserQuotaInfo 获取外部用户配额信息 (供 API 调用)
func GetExternalUserQuotaInfo(userId string) (used int, total int, isVIP bool, err error) {
	if !externalUserConfig.Enabled {
		return 0, 0, false, fmt.Errorf("外部用户验证未启用")
	}

	userData, err := getUserFromRedis(userId)
	if err != nil {
		return 0, 0, false, err
	}

	isVIP = userData.IsVIP && userData.VIPExpiresAt > time.Now().Unix()
	if isVIP || userData.Username == "admin" {
		return 0, -1, true, nil // -1 表示无限
	}

	quota, err := getUserQuota(userId)
	if err != nil {
		return 0, 0, false, err
	}

	// 检查月份重置
	currentMonthKey := time.Now().Format("2006-01")
	if quota.MonthKey != currentMonthKey {
		quota.UsedCount = 0
	}

	return quota.UsedCount, externalUserConfig.MonthlyQuota, false, nil
}

// SetUserVIP 设置用户 VIP 状态
func SetUserVIP(userId string, isVIP bool, expiresAt int64) error {
	userData, err := getUserFromRedis(userId)
	if err != nil {
		return err
	}

	userData.IsVIP = isVIP
	userData.VIPExpiresAt = expiresAt

	// 保存回 Redis
	userJSON, err := json.Marshal(userData)
	if err != nil {
		return err
	}

	url := fmt.Sprintf("%s/set/user:%s", externalUserConfig.RedisURL, userId)
	req, err := http.NewRequest("POST", url, bytes.NewReader(userJSON))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+externalUserConfig.RedisToken)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return nil
}

// parseIntOrDefault 解析整数，失败返回默认值
func parseIntOrDefault(s string, defaultVal int) int {
	if s == "" {
		return defaultVal
	}
	val, err := strconv.Atoi(s)
	if err != nil {
		return defaultVal
	}
	return val
}
