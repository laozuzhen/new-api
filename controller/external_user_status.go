package controller

import (
	"net/http"

	"github.com/QuantumNous/new-api/constant"
	"github.com/gin-gonic/gin"
)

// ExternalUserAuthStatus 外部用户验证系统状态
type ExternalUserAuthStatus struct {
	Enabled           bool   `json:"enabled"`
	RedisConfigured   bool   `json:"redisConfigured"`
	JWTConfigured     bool   `json:"jwtConfigured"`
	MonthlyQuota      int    `json:"monthlyQuota"`
	DisabledReason    string `json:"disabledReason,omitempty"`
}

// GetExternalUserAuthStatus 获取外部用户验证系统状态
func GetExternalUserAuthStatus(c *gin.Context) {
	status := ExternalUserAuthStatus{
		MonthlyQuota: constant.ExternalUserMonthlyQuota,
	}

	// 检查 Redis 配置
	status.RedisConfigured = constant.ExternalUserRedisURL != "" && constant.ExternalUserRedisToken != ""
	
	// 检查 JWT 配置 (可选，用于未来的签名验证)
	status.JWTConfigured = constant.ExternalUserJWTSecret != ""
	
	// 只需要 Redis 配置即可启用
	status.Enabled = status.RedisConfigured

	// 设置禁用原因
	if !status.Enabled {
		if !status.RedisConfigured {
			status.DisabledReason = "Redis 未配置 (UPSTASH_REDIS_REST_URL 或 UPSTASH_REDIS_REST_TOKEN)"
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    status,
	})
}
