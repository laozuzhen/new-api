package controller

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/QuantumNous/new-api/constant"
	"github.com/gin-gonic/gin"
)

// ExternalUserInfo 外部用户信息
type ExternalUserInfo struct {
	ID           string `json:"id"`
	Email        string `json:"email"`
	Username     string `json:"username"`
	IsVIP        bool   `json:"isVip"`
	VIPExpiresAt int64  `json:"vipExpiresAt"`
	QuotaUsed    int    `json:"quotaUsed"`
	QuotaTotal   int    `json:"quotaTotal"`
	MonthKey     string `json:"monthKey"`
}

// UserQuotaData 用户配额数据
type UserQuotaData struct {
	UsedCount   int    `json:"usedCount"`
	MonthKey    string `json:"monthKey"`
	LastResetAt int64  `json:"lastResetAt"`
}

// GetExternalUsers 获取所有外部用户列表
func GetExternalUsers(c *gin.Context) {
	if constant.ExternalUserRedisURL == "" || constant.ExternalUserRedisToken == "" {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "Redis 未配置",
		})
		return
	}

	// 使用 SCAN 命令获取所有 user:* 键
	users := []ExternalUserInfo{}
	cursor := "0"
	
	for {
		// Upstash REST API: POST with ["SCAN", cursor, "MATCH", "user:*", "COUNT", "100"]
		cmdBody, _ := json.Marshal([]interface{}{"SCAN", cursor, "MATCH", "user:*", "COUNT", "100"})
		
		req, err := http.NewRequest("POST", constant.ExternalUserRedisURL, bytes.NewReader(cmdBody))
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
			return
		}
		req.Header.Set("Authorization", "Bearer "+constant.ExternalUserRedisToken)
		req.Header.Set("Content-Type", "application/json")

		client := &http.Client{Timeout: 10 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
			return
		}
		
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		var result struct {
			Result []interface{} `json:"result"`
		}
		if err := json.Unmarshal(body, &result); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "解析 Redis 响应失败"})
			return
		}

		if len(result.Result) < 2 {
			break
		}

		// 获取下一个 cursor
		cursor = fmt.Sprintf("%v", result.Result[0])
		
		// 获取 keys
		keys, ok := result.Result[1].([]interface{})
		if !ok {
			break
		}

		// 获取每个用户的详细信息
		for _, k := range keys {
			key := fmt.Sprintf("%v", k)
			if len(key) <= 5 { // "user:" 长度
				continue
			}
			userId := key[5:] // 去掉 "user:" 前缀
			
			userInfo, err := getExternalUserInfo(userId)
			if err == nil && userInfo != nil {
				users = append(users, *userInfo)
			}
		}

		// cursor 为 "0" 表示扫描完成
		if cursor == "0" {
			break
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    users,
		"total":   len(users),
	})
}

// getExternalUserInfo 获取单个用户的完整信息
func getExternalUserInfo(userId string) (*ExternalUserInfo, error) {
	// 获取用户基本信息
	userKey := "user:" + userId
	userData, err := redisGet(userKey)
	if err != nil {
		return nil, err
	}

	var user ExternalUserInfo
	if err := json.Unmarshal([]byte(userData), &user); err != nil {
		return nil, err
	}
	user.ID = userId

	// 获取用户配额
	quotaKey := "quota:" + userId
	quotaData, err := redisGet(quotaKey)
	if err == nil && quotaData != "" {
		var quota UserQuotaData
		if json.Unmarshal([]byte(quotaData), &quota) == nil {
			// 检查月份是否需要重置
			currentMonth := time.Now().Format("2006-01")
			if quota.MonthKey == currentMonth {
				user.QuotaUsed = quota.UsedCount
			} else {
				user.QuotaUsed = 0
			}
			user.MonthKey = quota.MonthKey
		}
	}
	
	// VIP 用户显示无限配额，普通用户显示月度配额
	if user.IsVIP && user.VIPExpiresAt > time.Now().Unix() {
		user.QuotaTotal = -1 // -1 表示无限
	} else {
		user.QuotaTotal = constant.ExternalUserMonthlyQuota
	}

	return &user, nil
}

// UpdateExternalUserQuota 更新用户配额
func UpdateExternalUserQuota(c *gin.Context) {
	userId := c.Param("userId")
	if userId == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "缺少用户 ID"})
		return
	}

	var req struct {
		UsedCount int  `json:"usedCount"`
		Reset     bool `json:"reset"` // 是否重置为 0
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "参数错误"})
		return
	}

	// 获取当前配额
	quotaKey := "quota:" + userId
	currentMonth := time.Now().Format("2006-01")
	
	quota := UserQuotaData{
		MonthKey:    currentMonth,
		LastResetAt: time.Now().Unix(),
	}

	if req.Reset {
		quota.UsedCount = 0
	} else {
		quota.UsedCount = req.UsedCount
	}

	// 保存配额
	quotaJSON, _ := json.Marshal(quota)
	if err := redisSet(quotaKey, string(quotaJSON)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "保存配额失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "配额更新成功",
		"data": gin.H{
			"userId":    userId,
			"usedCount": quota.UsedCount,
			"monthKey":  quota.MonthKey,
		},
	})
}

// UpdateExternalUserVIP 更新用户 VIP 状态
func UpdateExternalUserVIP(c *gin.Context) {
	userId := c.Param("userId")
	if userId == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "缺少用户 ID"})
		return
	}

	var req struct {
		IsVIP        bool  `json:"isVip"`
		VIPExpiresAt int64 `json:"vipExpiresAt"` // Unix 时间戳
		VIPDays      int   `json:"vipDays"`      // 或者指定天数
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "参数错误"})
		return
	}

	// 获取用户数据
	userKey := "user:" + userId
	userData, err := redisGet(userKey)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "用户不存在"})
		return
	}

	var user map[string]interface{}
	if err := json.Unmarshal([]byte(userData), &user); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "解析用户数据失败"})
		return
	}

	// 更新 VIP 状态
	user["isVip"] = req.IsVIP
	if req.VIPDays > 0 {
		user["vipExpiresAt"] = time.Now().Add(time.Duration(req.VIPDays) * 24 * time.Hour).Unix()
	} else if req.VIPExpiresAt > 0 {
		user["vipExpiresAt"] = req.VIPExpiresAt
	} else if !req.IsVIP {
		user["vipExpiresAt"] = 0
	}

	// 保存用户数据
	userJSON, _ := json.Marshal(user)
	if err := redisSet(userKey, string(userJSON)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "保存用户数据失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "VIP 状态更新成功",
		"data":    user,
	})
}

// redisGet 从 Redis 获取值
func redisGet(key string) (string, error) {
	cmdBody, _ := json.Marshal([]string{"GET", key})
	
	req, err := http.NewRequest("POST", constant.ExternalUserRedisURL, bytes.NewReader(cmdBody))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+constant.ExternalUserRedisToken)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	
	var result struct {
		Result interface{} `json:"result"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", err
	}

	if result.Result == nil {
		return "", fmt.Errorf("key not found")
	}

	return fmt.Sprintf("%v", result.Result), nil
}

// redisSet 设置 Redis 值
func redisSet(key, value string) error {
	cmdBody, _ := json.Marshal([]string{"SET", key, value})
	
	req, err := http.NewRequest("POST", constant.ExternalUserRedisURL, bytes.NewReader(cmdBody))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+constant.ExternalUserRedisToken)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Redis error: %s", string(body))
	}

	return nil
}

// GetExternalUserDetail 获取单个用户详情
func GetExternalUserDetail(c *gin.Context) {
	userId := c.Param("userId")
	if userId == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "缺少用户 ID"})
		return
	}

	userInfo, err := getExternalUserInfo(userId)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "用户不存在: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    userInfo,
	})
}

// BatchUpdateQuota 批量更新配额
func BatchUpdateQuota(c *gin.Context) {
	var req struct {
		UserIds   []string `json:"userIds"`
		UsedCount int      `json:"usedCount"`
		Reset     bool     `json:"reset"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "参数错误"})
		return
	}

	currentMonth := time.Now().Format("2006-01")
	successCount := 0
	failedUsers := []string{}

	for _, userId := range req.UserIds {
		quota := UserQuotaData{
			MonthKey:    currentMonth,
			LastResetAt: time.Now().Unix(),
		}
		if req.Reset {
			quota.UsedCount = 0
		} else {
			quota.UsedCount = req.UsedCount
		}

		quotaJSON, _ := json.Marshal(quota)
		if err := redisSet("quota:"+userId, string(quotaJSON)); err != nil {
			failedUsers = append(failedUsers, userId)
		} else {
			successCount++
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success":      len(failedUsers) == 0,
		"message":      fmt.Sprintf("成功更新 %d 个用户，失败 %d 个", successCount, len(failedUsers)),
		"successCount": successCount,
		"failedUsers":  failedUsers,
	})
}

// parseIntParam 解析整数参数
func parseIntParam(s string, defaultVal int) int {
	if s == "" {
		return defaultVal
	}
	val, err := strconv.Atoi(s)
	if err != nil {
		return defaultVal
	}
	return val
}
