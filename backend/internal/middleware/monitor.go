package middleware

import (
	"backend/internal/config"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"runtime"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"gorm.io/gorm"
)

// Monitor 监控中间件：收集请求性能、资源使用、数据库和Redis指标
func Monitor(cfg *config.AppConfig, db *gorm.DB, redisClient *redis.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		method := c.Request.Method

		// 请求体大小统计
		var requestBodySize int64 = 0
		if c.Request.Body != nil {
			bodyBytes, _ := io.ReadAll(c.Request.Body)
			requestBodySize = int64(len(bodyBytes))
			c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
		}

		// 响应写入器，用于捕获响应状态码和大小
		w := &responseWriter{ResponseWriter: c.Writer}
		c.Writer = w

		// 处理请求
		c.Next()

		// 计算耗时
		duration := time.Since(start)

		// 获取内存使用情况
		var memStats runtime.MemStats
		runtime.ReadMemStats(&memStats)

		// 获取数据库连接池状态
		var dbStats struct {
			OpenConnections int
			IdleConnections int
			InUseConnections int
		}
		if sqlDB, err := db.DB(); err == nil {
			stats := sqlDB.Stats()
			dbStats.OpenConnections = stats.OpenConnections
			dbStats.IdleConnections = stats.Idle
						dbStats.InUseConnections = stats.InUse
		}

		// 获取Redis连接池状态
		var redisStats struct {
			ClientConnections int
			PoolStats         redis.PoolStats
		}
		if redisClient != nil {
						redisStats.PoolStats = *redisClient.PoolStats()
		}

		// 构建监控数据
		monitorData := map[string]interface{}{
			"timestamp":        time.Now().Format(time.RFC3339),
			"method":          method,
			"path":            path,
			"status":          w.statusCode,
			"duration_ms":     duration.Milliseconds(),
			"request_size":    requestBodySize,
			"response_size":   w.size,
			"user_agent":      c.Request.UserAgent(),
			"ip":             c.ClientIP(),
			"memory_heap":    memStats.HeapAlloc,
			"memory_sys":     memStats.Sys,
			"gc_pause_ns":    memStats.PauseNs[0],
			"gc_cycles":     memStats.NumGC,
			"db_open":        dbStats.OpenConnections,
			"db_idle":        dbStats.IdleConnections,
			"db_in_use":      dbStats.InUseConnections,
			"redis_clients":  redisStats.ClientConnections,
			"redis_pool_idle": redisStats.PoolStats.IdleConns,
			"redis_pool_in_use": redisStats.PoolStats.TotalConns - redisStats.PoolStats.IdleConns,
		}

		// 记录监控日志
		if duration > 1000*time.Millisecond { // 超过1秒的请求
			log.Printf("🐢 慢请求: 耗时=%dms 方法=%s 路径=%s IP=%s",
				duration.Milliseconds(), method, path, c.ClientIP())
		}

		// 异步发送监控数据到外部系统（如果有）
		go sendMetrics(monitorData)
	}
}

// responseWriter 用于捕获响应状态码和大小
type responseWriter struct {
	gin.ResponseWriter
	statusCode int
	size       int
}

func (w *responseWriter) WriteHeader(code int) {
	w.statusCode = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *responseWriter) Write(data []byte) (int, error) {
	size, err := w.ResponseWriter.Write(data)
	w.size += size
	return size, err
}

// sendMetrics 发送监控指标到外部系统
func sendMetrics(data map[string]interface{}) {
	// 这里可以集成到 Prometheus、Grafana、InfluxDB 等监控系统
	// 目前只记录日志
	jsonData, _ := json.Marshal(data)
	log.Printf("Metrics collected: %s", jsonData)
}

// HealthCheck 健康检查处理器
func HealthCheck(cfg *config.AppConfig, db *gorm.DB, redisClient *redis.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		health := map[string]interface{}{
			"status":    "ok",
			"timestamp": time.Now().Format(time.RFC3339),
			"version":   "1.0.0",
			"checks": map[string]interface{}{},
		}

		// 检查数据库连接
		if db != nil {
			if sqlDB, err := db.DB(); err == nil {
				if err := sqlDB.Ping(); err == nil {
					health["checks"].(map[string]interface{})["database"] = "healthy"
				} else {
					health["checks"].(map[string]interface{})["database"] = fmt.Sprintf("error: %v", err)
					health["status"] = "degraded"
				}
			}
		} else {
			health["checks"].(map[string]interface{})["database"] = "not configured"
		}

		// 检查Redis连接
		if redisClient != nil {
			if _, err := redisClient.Ping(redisClient.Context()).Result(); err == nil {
				health["checks"].(map[string]interface{})["redis"] = "healthy"
			} else {
				health["checks"].(map[string]interface{})["redis"] = fmt.Sprintf("error: %v", err)
				health["status"] = "degraded"
			}
		} else {
			health["checks"].(map[string]interface{})["redis"] = "not configured"
		}

		// 检查系统资源
		var memStats runtime.MemStats
		runtime.ReadMemStats(&memStats)

		// 内存使用率超过80%标记为警告
		memUsagePercent := float64(memStats.HeapAlloc) / float64(memStats.Sys) * 100
		if memUsagePercent > 80 {
			health["status"] = "warning"
		}
		health["checks"].(map[string]interface{})["memory_usage_percent"] = memUsagePercent

		c.JSON(http.StatusOK, health)
	}
}

// MetricsHandler 指标处理器
func MetricsHandler(db *gorm.DB, redisClient *redis.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		metrics := map[string]interface{}{}

		// 数据库指标
		if db != nil {
			if sqlDB, err := db.DB(); err == nil {
				stats := sqlDB.Stats()
				metrics["database"] = map[string]interface{}{
					"open_connections":    stats.OpenConnections,
					"in_use_connections":   stats.InUse,
					"idle_connections":    stats.Idle,
					"max_open_conns":      stats.OpenConnections,
					"wait_count":         stats.WaitCount,
					"wait_duration":     stats.WaitDuration.String(),
					"max_idle_closed":    stats.MaxIdleClosed,
					"max_lifetime_closed": stats.MaxLifetimeClosed,
				}
			}
		}

		// Redis指标
		if redisClient != nil {
			stats := redisClient.PoolStats()
			metrics["redis"] = map[string]interface{}{
				"active_connections": stats.TotalConns - stats.IdleConns,
				"idle_connections":   stats.IdleConns,
				"hits":             0,
				"misses":           0,
				"commands":         0,
			}
		}

		// 系统指标
		var memStats runtime.MemStats
		runtime.ReadMemStats(&memStats)
		metrics["system"] = map[string]interface{}{
			"memory_heap_alloc":  memStats.HeapAlloc,
			"memory_sys":         memStats.Sys,
			"memory_gc_pause":    memStats.PauseNs,
			"gc_cycles":         memStats.NumGC,
			"goroutines":        runtime.NumGoroutine(),
		}

		c.JSON(http.StatusOK, metrics)
	}
}

// LogErrorTracker 错误跟踪中间件
func LogErrorTracker() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		// 记录所有错误
		if len(c.Errors) > 0 {
			for _, e := range c.Errors {
				log.Printf("API Error: 路径=%s 方法=%s IP=%s 错误=%s",
					c.Request.URL.Path, c.Request.Method, c.ClientIP(), e.Error())
			}
		}
	}
}