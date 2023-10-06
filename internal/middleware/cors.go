package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/peergoim/signaling-server/internal/config"
	"strconv"
	"strings"
)

func Cors(config config.CorsConfig) gin.HandlerFunc {
	if !config.Enabled {
		return func(c *gin.Context) {
			c.Next()
		}
	}
	return func(c *gin.Context) {
		method := c.Request.Method
		c.Header("Access-Control-Allow-Origin", strings.Join(config.AllowOrigins, ","))
		c.Header("Access-Control-Allow-Headers", strings.Join(config.AllowHeaders, ","))
		c.Header("Access-Control-Allow-Methods", strings.Join(config.AllowMethods, ","))
		c.Header("Access-Control-Expose-Headers", strings.Join(config.ExposeHeaders, ","))
		c.Header("Access-Control-Allow-Credentials", strconv.FormatBool(config.AllowCredentials))
		if method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	}
}
