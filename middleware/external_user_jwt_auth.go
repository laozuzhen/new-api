package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// ExternalUserJWTAuth 轻量 JWT 中间件，用于验证 Editor 用户的 JWT。
// 验证成功后将 external_user_id 注入 gin.Context。
func ExternalUserJWTAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
			c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "未提供有效的认证信息"})
			c.Abort()
			return
		}
		token := strings.TrimPrefix(authHeader, "Bearer ")
		userData, err := verifyExternalJWT(token)
		if err != nil || userData == nil || userData.ID == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "认证失败: token 无效或已过期"})
			c.Abort()
			return
		}
		c.Set("external_user_id", userData.ID)
		c.Next()
	}
}
