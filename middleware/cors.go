package middleware

import (
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func CORS() gin.HandlerFunc {
	config := cors.DefaultConfig()
	config.AllowAllOrigins = true
	config.AllowCredentials = true
	config.AllowMethods = []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"}
	config.AllowHeaders = []string{"*"}
	// 暴露自定义响应头，让前端可以读取配额信息
	config.ExposeHeaders = []string{
		"X-Quota-Status",
		"X-Quota-Used",
		"X-Quota-Total",
		"X-Quota-Remaining",
		"X-Quota-Reason",
		"X-Channel-Id",
	}
	return cors.New(config)
}
