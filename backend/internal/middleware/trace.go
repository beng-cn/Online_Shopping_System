// Package middleware 请求追踪中间件 — 为每个 HTTP 请求生成唯一 Request ID 实现全链路追踪
//
// 功能：
//   - 解析客户端传入的 X-Request-ID 头（存在则复用，不存在则生成新的 UUID）
//   - 将 Request ID 注入 Gin Context 和 Response Header
//   - 与 GORM Logger 配合，将 Request ID 写入慢 SQL 日志（实现请求→SQL 关联）
//
// 面试亮点：分布式追踪的基础设施 — 在微服务架构中，Request ID 跨服务传递实现全链路追踪
package middleware

import (
	"context"
	"crypto/rand"
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
)

// RequestIDKey 是 Gin Context 中存储 Request ID 的键名
// 导出以便其他包（如 GORM Logger）从 Context 中提取 Request ID
const RequestIDKey = "request_id"

// Trace 请求追踪中间件 — 为每个请求注入唯一追踪 ID
//
// 使用方式：
//
//	router.Use(middleware.Trace()) // 建议在所有中间件之前注册
//
// 后续可通过 ctx.GetString(middleware.RequestIDKey) 获取 Request ID
func Trace() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 优先使用客户端传入的 X-Request-ID（支持跨服务传递）
		requestID := c.GetHeader("X-Request-ID")
		if requestID == "" {
			requestID = generateRequestID()
		}

		// 注入 Gin Context（供后续 Handler/Service 使用）
		c.Set(RequestIDKey, requestID)

		// 注入 context.Context（供 GORM db.WithContext(ctx) 链路追踪）
		ctx := context.WithValue(c.Request.Context(), RequestIDKey, requestID)
		c.Request = c.Request.WithContext(ctx)

		// 注入 Response Header（方便前端/客户端定位问题）
		c.Header("X-Request-ID", requestID)

		c.Next()
	}
}

// GetRequestID 从 Gin Context 中提取 Request ID（供其他包使用）
func GetRequestID(c *gin.Context) string {
	if id, exists := c.Get(RequestIDKey); exists {
		if s, ok := id.(string); ok {
			return s
		}
	}
	return ""
}

// generateRequestID 生成 16 字符的十六进制随机 Request ID（类似 Git commit hash 前8位 x2）
//
// 为什么不用 UUID？UUID 36 字符太长，日志中占空间且肉眼扫描困难
// 16 字符十六进制 = 64 bit 随机数，碰撞概率极低（10^9 请求中碰撞概率 ~2.7×10^-11）
func generateRequestID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		// 极端情况：随机数生成失败，用时间戳兜底
		return fmt.Sprintf("fallback-%d", time.Now().UnixNano())
	}
	return fmt.Sprintf("%x", b)
}
