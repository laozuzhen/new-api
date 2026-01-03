package controller

import (
	"net/http"
	"os"

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
	// 诊断信息
	DiagRedisURLSet   bool   `json:"diagRedisURLSet"`
	DiagRedisTokenSet bool   `json:"diagRedisTokenSet"`
	DiagJWTSecretSet  bool   `json:"diagJWTSecretSet"`
	DiagEnvVars       map[string]bool `json:"diagEnvVars"`
}

// GetExternalUserAuthStatus 获取外部用户验证系统状态
func GetExternalUserAuthStatus(c *gin.Context) {
	status := ExternalUserAuthStatus{
		MonthlyQuota: constant.ExternalUserMonthlyQuota,
		DiagEnvVars:  make(map[string]bool),
	}

	// 检查 Redis 配置
	status.RedisConfigured = constant.ExternalUserRedisURL != "" && constant.ExternalUserRedisToken != ""
	status.DiagRedisURLSet = constant.ExternalUserRedisURL != ""
	status.DiagRedisTokenSet = constant.ExternalUserRedisToken != ""
	
	// 检查 JWT 配置 (可选，用于未来的签名验证)
	status.JWTConfigured = constant.ExternalUserJWTSecret != ""
	status.DiagJWTSecretSet = constant.ExternalUserJWTSecret != ""
	
	// 检查环境变量是否设置 (不暴露值，只检查是否存在)
	envVarsToCheck := []string{
		"UPSTASH_REDIS_REST_URL",
		"UPSTASH_REDIS_REST_TOKEN",
		"EXTERNAL_USER_REDIS_URL",
		"EXTERNAL_USER_REDIS_TOKEN",
		"EXTERNAL_USER_JWT_SECRET",
		"EXTERNAL_USER_MONTHLY_QUOTA",
	}
	for _, envVar := range envVarsToCheck {
		status.DiagEnvVars[envVar] = os.Getenv(envVar) != ""
	}
	
	// 只需要 Redis 配置即可启用
	status.Enabled = status.RedisConfigured

	// 设置禁用原因
	if !status.Enabled {
		reasons := []string{}
		if !status.DiagRedisURLSet {
			reasons = append(reasons, "UPSTASH_REDIS_REST_URL 未设置")
		}
		if !status.DiagRedisTokenSet {
			reasons = append(reasons, "UPSTASH_REDIS_REST_TOKEN 未设置")
		}
		if len(reasons) > 0 {
			status.DisabledReason = "Redis 未配置: " + reasons[0]
			if len(reasons) > 1 {
				status.DisabledReason += " 和 " + reasons[1]
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    status,
		"message": getStatusMessage(status),
	})
}

// getStatusMessage 根据状态生成诊断消息
func getStatusMessage(status ExternalUserAuthStatus) string {
	if status.Enabled {
		if status.JWTConfigured {
			return "外部用户验证已启用，配置完整"
		}
		return "外部用户验证已启用，但 JWT_SECRET 未配置 (可选)"
	}
	return "外部用户验证未启用，请检查 Redis 配置"
}
