package middleware

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/constant"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
)

// ExternalUserConfig 外部用户验证配置
type ExternalUserConfig struct {
	RedisURL      string        // Redis 连接 URL (支持本地 redis:// 和 Upstash)
	RedisToken    string        // Upstash Redis REST Token (本地 Redis 不需
	JWTSecret     string        // JWT 密钥 (与前端一致)
	MonthlyQuota  int           // 普通用户每月配额
	Enabled       bool          // 是否启用外部用户验证
	redisClient   *redis.Client // go-redis 客户端 (本地 Redis)
	useLocalRedis bool          // 是否使用本地 Redis
}

var externalUserConfig = ExternalUserConfig{
	MonthlyQuota: 30,
	Enabled:      false,
}

var ctx = context.Background()

// InitExternalUserAuth 初始化外部用户验证配置
func InitExternalUserAuth(redisURL, redisToken, jwtSecret string, monthlyQuota int) {
	fmt.Printf("[ExternalUserAuth] 初始化开始: URL=%s, Token长度=%d, Quota=%d\n", redisURL, len(redisToken), monthlyQuota)
	
	externalUserConfig.RedisURL = redisURL
	externalUserConfig.RedisToken = redisToken
	externalUserConfig.JWTSecret = jwtSecret
	if monthlyQuota > 0 {
		externalUserConfig.MonthlyQuota = monthlyQuota
	}

	// 检测是否是本地 Redis (redis:// 开头)
	if strings.HasPrefix(redisURL, "redis://") {
		externalUserConfig.useLocalRedis = true
		opt, err := redis.ParseURL(redisURL)
		if err != nil {
			fmt.Printf("[ExternalUserAuth] ❌ 解析 Redis URL 失败: %v\n", err)
			externalUserConfig.Enabled = false
			constant.ExternalUserAuthEnabled = false
			return
		}
		externalUserConfig.redisClient = redis.NewClient(opt)
		// 测试连接
		_, err = externalUserConfig.redisClient.Ping(ctx).Result()
		if err != nil {
			fmt.Printf("[ExternalUserAuth] ❌ Redis 连接失败: %v\n", err)
			externalUserConfig.Enabled = false
			constant.ExternalUserAuthEnabled = false
			return
		}
		externalUserConfig.Enabled = true
		constant.ExternalUserAuthEnabled = true
		fmt.Printf("[ExternalUserAuth] ✓ 已启用外部用户验证 (本地 Redis), URL: %s, 每月配额: %d\n", redisURL, externalUserConfig.MonthlyQuota)
	} else if redisURL != "" && redisToken != "" {
		// Upstash REST API
		externalUserConfig.useLocalRedis = false
		externalUserConfig.Enabled = true
		constant.ExternalUserAuthEnabled = true
		fmt.Printf("[ExternalUserAuth] ✓ 已启用外部用户验证 (Upstash), URL: %s, 每月配额: %d\n", redisURL, externalUserConfig.MonthlyQuota)
	} else {
		externalUserConfig.Enabled = false
		constant.ExternalUserAuthEnabled = false
		fmt.Printf("[ExternalUserAuth] ⚠️ 外部用户验证未启用 (Redis 未配置)\n")
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
	MonthKey    string `json:"monthKey"`
	LastResetAt int64  `json:"lastResetAt"`
}


// ChannelQuotaConfig 渠道配额配置 (从前端传递)
type ChannelQuotaConfig struct {
	ChannelId    string `json:"channelId"`
	ChannelName  string `json:"channelName"`
	QuotaEnabled bool   `json:"quotaEnabled"`
	QuotaLimit   int    `json:"quotaLimit"` // -1 表示无限制
}

// ExternalUserAuth 外部用户验证中间件
func ExternalUserAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		fmt.Printf("[ExternalUserAuth] ========== 开始处理请求 ==========\n")
		fmt.Printf("[ExternalUserAuth] 请求路径: %s %s\n", c.Request.Method, c.Request.URL.Path)
		fmt.Printf("[ExternalUserAuth] 配置状态: Enabled=%v, UseLocalRedis=%v, RedisURL=%s\n", externalUserConfig.Enabled, externalUserConfig.useLocalRedis, externalUserConfig.RedisURL)

		if !externalUserConfig.Enabled {
			fmt.Printf("[ExternalUserAuth] ❌ 中间件未启用, constant.ExternalUserRedisURL=%s\n", constant.ExternalUserRedisURL)
			abortWithOpenAiMessage(c, http.StatusServiceUnavailable, "服务未正确配置，请联系管理员 (Redis 未配置)")
			return
		}

		externalToken := c.Request.Header.Get("X-External-User-Token")
		if externalToken == "" {
			fmt.Printf("[ExternalUserAuth] ❌ 未收到 X-External-User-Token header\n")
			abortWithOpenAiMessage(c, http.StatusUnauthorized, "请先登录后再使用 API")
			return
		}
		fmt.Printf("[ExternalUserAuth] ✓ 收到 Token: %s...\n", maskString(externalToken, 30))

		// 获取渠道配额配置 (从 header 传递)
		channelId := c.Request.Header.Get("X-Channel-Id")
		channelName := c.Request.Header.Get("X-Channel-Name")
		quotaEnabledStr := c.Request.Header.Get("X-Channel-Quota-Enabled")
		quotaLimitStr := c.Request.Header.Get("X-Channel-Quota-Limit")
		
		// 解析渠道配额配置
		quotaEnabled := quotaEnabledStr != "false" // 默认启用
		quotaLimit := externalUserConfig.MonthlyQuota // 默认使用全局配额
		if quotaLimitStr != "" {
			if parsed, err := strconv.Atoi(quotaLimitStr); err == nil {
				quotaLimit = parsed
			}
		}
		
		fmt.Printf("[ExternalUserAuth] 渠道配置: ID=%s, Name=%s, QuotaEnabled=%v, QuotaLimit=%d\n", 
			channelId, channelName, quotaEnabled, quotaLimit)

		userData, err := verifyExternalJWT(externalToken)
		if err != nil {
			fmt.Printf("[ExternalUserAuth] ❌ JWT 验证失败: %v\n", err)
			abortWithOpenAiMessage(c, http.StatusUnauthorized, "外部用户验证失败: "+err.Error())
			return
		}
		fmt.Printf("[ExternalUserAuth] ✓ 用户验证成功: ID=%s, Email=%s\n", userData.ID, userData.Email)

		isVIP := userData.IsVIP && userData.VIPExpiresAt > time.Now().Unix()
		isAdmin := userData.Username == "admin"

		if isVIP || isAdmin {
			fmt.Printf("[ExternalUserAuth] ✓ VIP/管理员用户，跳过配额检查\n")
			c.Set("external_user_id", userData.ID)
			c.Set("external_user_email", userData.Email)
			c.Set("external_user_vip", true)
			c.Header("X-Quota-Status", "vip")
			c.Header("X-Quota-Used", "0")
			c.Header("X-Quota-Total", "-1")
			c.Header("X-Quota-Remaining", "-1")
			c.Header("X-Channel-Id", channelId)
			c.Next()
			return
		}

		// 如果渠道禁用了配额，直接放行
		if !quotaEnabled {
			fmt.Printf("[ExternalUserAuth] ✓ 渠道 %s 禁用了配额限制，直接放行\n", channelName)
			c.Set("external_user_id", userData.ID)
			c.Set("external_user_email", userData.Email)
			c.Set("external_user_vip", false)
			c.Header("X-Quota-Status", "disabled")
			c.Header("X-Quota-Used", "0")
			c.Header("X-Quota-Total", "-1")
			c.Header("X-Quota-Remaining", "-1")
			c.Header("X-Channel-Id", channelId)
			c.Next()
			return
		}

		// 如果渠道配额无限制
		if quotaLimit == -1 {
			fmt.Printf("[ExternalUserAuth] ✓ 渠道 %s 配额无限制，直接放行\n", channelName)
			c.Set("external_user_id", userData.ID)
			c.Set("external_user_email", userData.Email)
			c.Set("external_user_vip", false)
			c.Header("X-Quota-Status", "unlimited")
			c.Header("X-Quota-Used", "0")
			c.Header("X-Quota-Total", "-1")
			c.Header("X-Quota-Remaining", "-1")
			c.Header("X-Channel-Id", channelId)
			c.Next()
			return
		}

		// 获取用户在该渠道的配额 (per-user-per-channel)
		quota, err := getUserChannelQuota(userData.ID, channelId)
		if err != nil {
			fmt.Printf("[ExternalUserAuth] ❌ 获取配额失败: %v\n", err)
			abortWithOpenAiMessage(c, http.StatusInternalServerError, "获取用户配额失败: "+err.Error())
			return
		}

		currentMonthKey := time.Now().Format("2006-01")
		if quota.MonthKey != currentMonthKey {
			quota.UsedCount = 0
			quota.MonthKey = currentMonthKey
			quota.LastResetAt = time.Now().Unix()
		}

		if quota.UsedCount >= quotaLimit {
			fmt.Printf("[ExternalUserAuth] ❌ 渠道 %s 配额已用完: %d/%d\n", channelName, quota.UsedCount, quotaLimit)
			abortWithOpenAiMessage(c, http.StatusTooManyRequests,
				fmt.Sprintf("渠道「%s」本月调用次数已用完 (%d/%d)，请升级 VIP 或切换其他渠道",
					channelName, quota.UsedCount, quotaLimit))
			return
		}

		quota.UsedCount++
		if err := saveUserChannelQuota(userData.ID, channelId, quota); err != nil {
			fmt.Printf("[ExternalUserAuth] ⚠️ 保存配额失败: %v\n", err)
		}

		c.Set("external_user_id", userData.ID)
		c.Set("external_user_email", userData.Email)
		c.Set("external_user_vip", false)
		c.Header("X-Quota-Status", "active")
		c.Header("X-Quota-Used", strconv.Itoa(quota.UsedCount))
		c.Header("X-Quota-Total", strconv.Itoa(quotaLimit))
		c.Header("X-Quota-Remaining", strconv.Itoa(quotaLimit-quota.UsedCount))
		c.Header("X-Channel-Id", channelId)

		c.Next()
	}
}

func maskString(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

func verifyExternalJWT(tokenString string) (*ExternalUserData, error) {
	parts := strings.Split(tokenString, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("无效的 token 格式")
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		payload, err = base64.StdEncoding.DecodeString(parts[1])
		if err != nil {
			return nil, fmt.Errorf("无法解码 token payload")
		}
	}

	var claims map[string]interface{}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return nil, fmt.Errorf("无法解析 token payload")
	}

	if exp, ok := claims["exp"].(float64); ok {
		if int64(exp) < time.Now().Unix() {
			return nil, fmt.Errorf("token 已过期")
		}
	}

	userId, _ := claims["userId"].(string)
	email, _ := claims["email"].(string)

	if userId == "" && email == "" {
		return nil, fmt.Errorf("token 中缺少用户信息")
	}

	userData, err := getUserFromRedis(userId)
	if err != nil {
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
	if !externalUserConfig.Enabled {
		return nil, fmt.Errorf("Redis 未配置")
	}

	key := "user:" + userId

	if externalUserConfig.useLocalRedis {
		// 本地 Redis
		val, err := externalUserConfig.redisClient.Get(ctx, key).Result()
		if err == redis.Nil {
			return nil, fmt.Errorf("用户不存在")
		}
		if err != nil {
			return nil, err
		}
		var userData ExternalUserData
		if err := json.Unmarshal([]byte(val), &userData); err != nil {
			return nil, err
		}
		return &userData, nil
	}

	// Upstash REST API (保持原有逻辑)
	return getUserFromUpstash(userId)
}

// getUserQuota 获取用户配额 (旧版，保留兼容)
func getUserQuota(userId string) (*UserQuota, error) {
	if !externalUserConfig.Enabled {
		return &UserQuota{MonthKey: time.Now().Format("2006-01")}, nil
	}

	key := "quota:" + userId

	if externalUserConfig.useLocalRedis {
		val, err := externalUserConfig.redisClient.Get(ctx, key).Result()
		if err == redis.Nil {
			return &UserQuota{MonthKey: time.Now().Format("2006-01")}, nil
		}
		if err != nil {
			return nil, err
		}
		var quota UserQuota
		if err := json.Unmarshal([]byte(val), &quota); err != nil {
			return &UserQuota{MonthKey: time.Now().Format("2006-01")}, nil
		}
		return &quota, nil
	}

	// Upstash REST API
	return getQuotaFromUpstash(userId)
}

// getUserChannelQuota 获取用户在特定渠道的配额 (per-user-per-channel)
func getUserChannelQuota(userId string, channelId string) (*UserQuota, error) {
	if !externalUserConfig.Enabled {
		return &UserQuota{MonthKey: time.Now().Format("2006-01")}, nil
	}

	// 如果没有指定渠道，使用旧的 key 格式
	var key string
	if channelId == "" {
		key = "quota:" + userId
	} else {
		key = "quota:" + userId + ":channel:" + channelId
	}

	if externalUserConfig.useLocalRedis {
		val, err := externalUserConfig.redisClient.Get(ctx, key).Result()
		if err == redis.Nil {
			return &UserQuota{MonthKey: time.Now().Format("2006-01")}, nil
		}
		if err != nil {
			return nil, err
		}
		var quota UserQuota
		if err := json.Unmarshal([]byte(val), &quota); err != nil {
			return &UserQuota{MonthKey: time.Now().Format("2006-01")}, nil
		}
		return &quota, nil
	}

	// Upstash REST API
	return getChannelQuotaFromUpstash(userId, channelId)
}

// saveUserQuota 保存用户配额 (旧版，保留兼容)
func saveUserQuota(userId string, quota *UserQuota) error {
	if !externalUserConfig.Enabled {
		return fmt.Errorf("Redis 未配置")
	}

	key := "quota:" + userId
	quotaJSON, err := json.Marshal(quota)
	if err != nil {
		return err
	}

	if externalUserConfig.useLocalRedis {
		return externalUserConfig.redisClient.Set(ctx, key, string(quotaJSON), 0).Err()
	}

	// Upstash REST API
	return saveQuotaToUpstash(userId, quota)
}

// saveUserChannelQuota 保存用户在特定渠道的配额 (per-user-per-channel)
func saveUserChannelQuota(userId string, channelId string, quota *UserQuota) error {
	if !externalUserConfig.Enabled {
		return fmt.Errorf("Redis 未配置")
	}

	// 如果没有指定渠道，使用旧的 key 格式
	var key string
	if channelId == "" {
		key = "quota:" + userId
	} else {
		key = "quota:" + userId + ":channel:" + channelId
	}
	
	quotaJSON, err := json.Marshal(quota)
	if err != nil {
		return err
	}

	if externalUserConfig.useLocalRedis {
		return externalUserConfig.redisClient.Set(ctx, key, string(quotaJSON), 0).Err()
	}

	// Upstash REST API
	return saveChannelQuotaToUpstash(userId, channelId, quota)
}

// GetExternalUserQuotaInfo 获取外部用户配额信息
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
		return 0, -1, true, nil
	}

	quota, err := getUserQuota(userId)
	if err != nil {
		return 0, 0, false, err
	}

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

	userJSON, err := json.Marshal(userData)
	if err != nil {
		return err
	}

	key := "user:" + userId

	if externalUserConfig.useLocalRedis {
		return externalUserConfig.redisClient.Set(ctx, key, string(userJSON), 0).Err()
	}

	// Upstash REST API
	return setUserToUpstash(userId, userData)
}

// IsExternalUserEnabled 检查外部用户验证是否启用
func IsExternalUserEnabled() bool {
	return externalUserConfig.Enabled
}

// GetRedisClient 获取 Redis 客户端 (供外部使用)
func GetRedisClient() *redis.Client {
	return externalUserConfig.redisClient
}

// IsUsingLocalRedis 是否使用本地 Redis
func IsUsingLocalRedis() bool {
	return externalUserConfig.useLocalRedis
}


// ========== Upstash REST API 兼容函数 ==========

func getUserFromUpstash(userId string) (*ExternalUserData, error) {
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
		jsonBytes, _ := json.Marshal(v)
		if err := json.Unmarshal(jsonBytes, &userData); err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("用户数据格式错误")
	}

	return &userData, nil
}

func getQuotaFromUpstash(userId string) (*UserQuota, error) {
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

	quota := &UserQuota{MonthKey: time.Now().Format("2006-01")}
	if result.Result != nil {
		if v, ok := result.Result.(string); ok && v != "" {
			json.Unmarshal([]byte(v), quota)
		}
	}

	return quota, nil
}

func saveQuotaToUpstash(userId string, quota *UserQuota) error {
	quotaJSON, _ := json.Marshal(quota)
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

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Redis 返回错误: %s", string(body))
	}

	return nil
}

// getChannelQuotaFromUpstash 从 Upstash 获取用户渠道配额
func getChannelQuotaFromUpstash(userId string, channelId string) (*UserQuota, error) {
	var key string
	if channelId == "" {
		key = "quota:" + userId
	} else {
		key = "quota:" + userId + ":channel:" + channelId
	}
	
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

	quota := &UserQuota{MonthKey: time.Now().Format("2006-01")}
	if result.Result != nil {
		if v, ok := result.Result.(string); ok && v != "" {
			json.Unmarshal([]byte(v), quota)
		}
	}

	return quota, nil
}

// saveChannelQuotaToUpstash 保存用户渠道配额到 Upstash
func saveChannelQuotaToUpstash(userId string, channelId string, quota *UserQuota) error {
	quotaJSON, _ := json.Marshal(quota)
	var key string
	if channelId == "" {
		key = "quota:" + userId
	} else {
		key = "quota:" + userId + ":channel:" + channelId
	}
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

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Redis 返回错误: %s", string(body))
	}

	return nil
}

func setUserToUpstash(userId string, userData *ExternalUserData) error {
	userJSON, _ := json.Marshal(userData)
	key := "user:" + userId
	cmdBody, _ := json.Marshal([]string{"SET", key, string(userJSON)})

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
