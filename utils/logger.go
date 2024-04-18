package utils

import (
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// ロガーを初期化
func InitLogger() (*zap.Logger, error) {
	return zap.NewProduction()
}

// Gin のミドルウェア用関数で、リクエストのログを取得します。
func RequestLogger(logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		c.Next()
		latency := time.Since(start)
		logger.Info("request",
			zap.String("path", path),
			zap.Int("status", c.Writer.Status()),
			zap.Duration("latency", latency),
		)
	}
}
