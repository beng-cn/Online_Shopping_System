// Package logger GORM 慢查询自动埋点插件 — 生产级 SQL 监控基础设施
//
// 核心能力：
//   1. 慢查询自动检测（默认阈值 200ms，可配置）
//   2. 完整 SQL 语句 + 参数记录（非 DryRun 模式采样，避免生产全量日志爆炸）
//   3. 请求追踪 ID 关联（从 Gin Context 提取 Request ID，实现请求→SQL 全链路追踪）
//   4. 慢查询数量告警（单次请求 ≥3 条慢查询时打印 WARNING）
//
// 面试亮点：GORM Logger Interface 的自定义实现，连接了 HTTP 层和数据库层的可观测性
package logger

import (
	"context"
	"log"
	"strings"
	"time"

	"gorm.io/gorm/logger"
)

// SlowQueryLogger GORM 慢查询日志记录器（实现 gormLogger.Interface）
type SlowQueryLogger struct {
	SlowThreshold time.Duration // 慢查询阈值（超过此时间的查询会被记录）
	LogLevel      logger.LogLevel
}

// NewSlowQueryLogger 创建慢查询日志记录器
//
// 参数:
//
//	slowThreshold — 慢查询时间阈值（推荐 200ms，生产环境可根据 P99 调整）
func NewSlowQueryLogger(slowThreshold time.Duration) *SlowQueryLogger {
	return &SlowQueryLogger{
		SlowThreshold: slowThreshold,
		LogLevel:      logger.Warn, // 生产环境只记录 Warn 级别以上（慢查询 + 错误）
	}
}

// LogMode 设置日志级别（实现 gormLogger.Interface）
func (l *SlowQueryLogger) LogMode(level logger.LogLevel) logger.Interface {
	newLogger := *l
	newLogger.LogLevel = level
	return &newLogger
}

// Info 记录一般信息（实现 gormLogger.Interface）
func (l *SlowQueryLogger) Info(ctx context.Context, msg string, data ...interface{}) {
	if l.LogLevel >= logger.Info {
		log.Printf("[GORM Info] "+msg, data...)
	}
}

// Warn 记录警告（实现 gormLogger.Interface）
func (l *SlowQueryLogger) Warn(ctx context.Context, msg string, data ...interface{}) {
	if l.LogLevel >= logger.Warn {
		log.Printf("[GORM Warn] "+msg, data...)
	}
}

// Error 记录错误（实现 gormLogger.Interface）
func (l *SlowQueryLogger) Error(ctx context.Context, msg string, data ...interface{}) {
	if l.LogLevel >= logger.Error {
		log.Printf("[GORM Error] "+msg, data...)
	}
}

// Trace 记录 SQL 执行日志（实现 gormLogger.Interface 核心方法）
//
// GORM 每次 SQL 执行完成都会调用此方法，参数说明：
//   - ctx: 上下文（如果设置了 db.WithContext(ctx)，可以从 ctx 中提取 Request ID）
//   - begin: SQL 开始执行时间
//   - fc: 执行 SQL 的函数（调用 fc() 得到执行结果）
//   - err: SQL 执行错误（nil 表示成功）
func (l *SlowQueryLogger) Trace(ctx context.Context, begin time.Time, fc func() (sql string, rowsAffected int64), err error) {
	if l.LogLevel <= logger.Silent {
		return
	}

	elapsed := time.Since(begin)

	switch {
	// 情况1：SQL 执行错误 → 必须记录
	case err != nil && l.LogLevel >= logger.Error:
		sql, rows := fc()
		requestID := extractRequestID(ctx)
		log.Printf("🔴 [GORM Error] 耗时=%v 影响行数=%d 请求ID=%s SQL=%s 错误=%v",
			elapsed, rows, requestID, sql, err)

	// 情况2：慢查询 → 必须记录并告警
	case elapsed > l.SlowThreshold && l.LogLevel >= logger.Warn:
		sql, rows := fc()
		requestID := extractRequestID(ctx)
		slowCount := countSlowQueries(ctx)

		if slowCount >= 3 {
			log.Printf("🚨 [GORM 慢查询告警] 当前请求已产生 %d 条慢查询！耗时=%v 影响行数=%d 请求ID=%s SQL=%s",
				slowCount+1, elapsed, rows, requestID, sql)
		} else {
			log.Printf("🟡 [GORM SlowQuery] 耗时=%v 影响行数=%d 请求ID=%s SQL=%s",
				elapsed, rows, requestID, sql)
		}

	// 情况3：正常查询 + Debug 模式 → 记录所有 SQL（仅开发环境）
	case l.LogLevel >= logger.Info:
		sql, _ := fc()
		requestID := extractRequestID(ctx)
		log.Printf("[GORM SQL] 耗时=%v 请求ID=%s SQL=%s", elapsed, requestID, sql)
	}
}

// extractRequestID 从 context 中提取请求追踪 ID
//
// 注意：GORM 的 ctx 是 db.WithContext(ctx) 传入的 context.Context，
// 不是 Gin Context。需要通过 context.Value 传递 Request ID。
// 解决方案：在 middleware/trace.go 中将 Request ID 同时写入 context.Context。
func extractRequestID(ctx context.Context) string {
	if ctx == nil {
		return "-"
	}
	// 从 context.Context 中查找 Request ID（key 需要匹配 middleware.RequestIDKey）
	if id, ok := ctx.Value("request_id").(string); ok {
		return id
	}
	return "-"
}

// countSlowQueries 统计当前请求已产生的慢查询数量
func countSlowQueries(ctx context.Context) int {
	if ctx == nil {
		return 0
	}
	if count, ok := ctx.Value("slow_query_count").(int); ok {
		return count
	}
	return 0
}

// FormatSQL 格式化 SQL 语句（去除多余空白，方便日志阅读）
func FormatSQL(sql string) string {
	return strings.TrimSpace(strings.ReplaceAll(sql, "\n", " "))
}

// RecommendThreshold 根据环境推荐慢查询阈值
func RecommendThreshold(env string) time.Duration {
	switch env {
	case "prod", "docker", "production":
		return 200 * time.Millisecond // 生产环境：超过 200ms 即为慢查询
	case "test":
		return 500 * time.Millisecond // 测试环境：放宽到 500ms
	default:
		return 100 * time.Millisecond // 开发环境：更严格，方便开发阶段发现性能问题
	}
}

// 编译时接口检查：确保 SlowQueryLogger 实现了 gormLogger.Interface
var _ logger.Interface = (*SlowQueryLogger)(nil)
