package main

// @title           在线商城 API
// @version         1.0
// @description     基于 Go/Gin 的电商系统接口文档，包含用户、商品、购物车、订单、秒杀模块
// @host            localhost:8080
// @BasePath        /api/v1
// @schemes         http
// @securityDefinitions.apikey Bearer
// @in              header
// @name            Authorization
// @description     在输入框中输入 "Bearer {token}" 进行认证

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"backend/internal/pkg/env"

	"github.com/joho/godotenv"
)

// 全局context，用于控制所有goroutine的优雅退出
var appContext = context.Background()

func main() {
	// 🔴 第〇步：加载 .env 文件到进程环境变量
	// godotenv 将 .env 中的 KEY=VALUE 注入到 os.Environ()，
	// 之后 Viper 的 AutomaticEnv() 和 env.ValidateSecrets() 都能读取到
	if err := godotenv.Load(); err != nil {
		// .env 文件不存在时不报错，兼容 Docker/K8s 直接注入环境变量的场景
		log.Println("⚠️  未找到 .env 文件，将使用系统环境变量或 YAML 配置中的值")
	}

	// 🔴 第一步：环境变量门禁校验（敏感信息零硬编码）
	// 在加载任何配置之前，先检查所有密码/密钥是否已通过环境变量注入
	env.ValidateSecrets()

	router, err := InitApp()
	if err != nil {
		panic(fmt.Sprintf("应用初始化失败: %v", err))
	}

	// 异步执行缓存预热和秒杀崩溃恢复（不阻塞服务启动）
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("🚨 启动后初始化发生严重panic: %v", r)
				// 关键初始化失败，添加重试机制
				time.Sleep(5 * time.Second)
				if r := recover(); r != nil {
					log.Printf("🚨 初始化重试仍然失败，请检查系统状态: %v", r)
				}
			}
		}()
		// 延迟3秒确保数据库和Redis连接完全就绪
		time.Sleep(3 * time.Second)

		// 预热分类缓存（全量）
		if err := router.CategoryService.WarmUpAllCategories(context.Background()); err != nil {
			log.Printf("⚠️ 分类缓存预热失败: %v", err)
		}

		// 初始化布隆过滤器（从数据库加载所有产品ID）
		if err := router.ProductService.InitBloomFilter(context.Background()); err != nil {
			log.Printf("⚠️ 布隆过滤器初始化失败: %v", err)
		}

		// 使用配置的数量预热热门商品
		hotLimit := router.Config.Cache.HotProductWarmUpLimit
		if hotLimit <= 0 {
			hotLimit = 100 // 默认值
		}
		if err := router.ProductService.WarmUpHotProducts(context.Background(), hotLimit); err != nil {
			log.Printf("⚠️ 热门商品缓存预热失败: %v", err)
		}

		// 秒杀崩溃恢复
		router.FlashService.RecoverPendingOrders(context.Background())
	}()

	// 秒杀自动预热扫描器（每30秒检查一次即将开始的活动）
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("🚨 秒杀自动预热发生panic: %v", r)
				// 尝试恢复
				time.Sleep(10 * time.Second)
				if r := recover(); r != nil {
					log.Printf("🚨 秒杀预热无法恢复，影响系统功能: %v", r)
				}
			}
		}()
		time.Sleep(5 * time.Second)
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		done := make(chan os.Signal, 1)
		signal.Notify(done, syscall.SIGINT, syscall.SIGTERM)

		for {
			select {
			case <-ticker.C:
				router.FlashService.AutoWarmUp(context.Background())
			case <-done:
				log.Println("秒杀预热扫描器已停止")
				return
			}
		}
	}()

	// 用户限流器定期清理（每5分钟清理超过30分钟未活动的用户桶，防止内存泄漏）
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("🚨 限流器清理发生panic: %v", r)
				// 尝试恢复
				time.Sleep(5 * time.Minute)
				if r := recover(); r != nil {
					log.Printf("🚨 限流器清理无法恢复，可能导致内存泄漏: %v", r)
				}
			}
		}()
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		done := make(chan os.Signal, 1)
		signal.Notify(done, syscall.SIGINT, syscall.SIGTERM)

		for {
			select {
			case <-ticker.C:
				router.UserSnatchLimiter.CleanupExpired(30 * time.Minute)
			case <-done:
				log.Println("限流器清理已停止")
				return
			}
		}
	}()

	// 秒杀一致性守护器：崩溃恢复 + 库存对账，每5秒执行一次
	// 合并维护，两套逻辑共用同一扫描周期
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("🚨 秒杀一致性守护器发生panic，可能导致数据不一致: %v", r)
				// 关键守护器panic，立即重试
				time.Sleep(5 * time.Second)
				if r := recover(); r != nil {
					log.Printf("🚨 一致性守护器无法恢复，数据一致性风险！: %v", r)
				}
			}
		}()
		time.Sleep(5 * time.Second) // 延迟5秒，确保服务完全就绪

		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		done := make(chan os.Signal, 1)
		signal.Notify(done, syscall.SIGINT, syscall.SIGTERM)

		for {
			select {
			case <-ticker.C:
				router.FlashService.GuardConsistency(context.Background())
			case <-done:
				log.Println("一致性守护器已停止")
				return
			}
		}
	}()

	// 布隆过滤器定时重建（每1小时全量重建，清理已删除产品的残留ID）
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("🚨 布隆过滤器定时重建发生panic: %v", r)
				// 布隆过滤器影响缓存穿透防护，需要尽快恢复
				time.Sleep(1 * time.Hour)
				if r := recover(); r != nil {
					log.Printf("🚨 布隆过滤器无法恢复，可能导致缓存穿透: %v", r)
				}
			}
		}()
		time.Sleep(2 * time.Hour) // 首次重建延迟2小时（等启动时的首次加载完成后再周期性重建）
		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()
		done := make(chan os.Signal, 1)
		signal.Notify(done, syscall.SIGINT, syscall.SIGTERM)

		for {
			select {
			case <-ticker.C:
				if err := router.ProductService.InitBloomFilter(context.Background()); err != nil {
					log.Printf("⚠️ 布隆过滤器定时重建失败: %v", err)
				}
			case <-done:
				log.Println("布隆过滤器重建已停止")
				return
			}
		}
	}()

	// 秒杀超时订单扫描器（每30秒执行一次）
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("🚨 秒杀超时扫描器发生panic，可能导致订单状态异常: %v", r)
				// 超时扫描器影响订单清理，需要尽快恢复
				time.Sleep(30 * time.Second)
				if r := recover(); r != nil {
					log.Printf("🚨 超时扫描器无法恢复，可能造成订单堆积: %v", r)
				}
			}
		}()
		// 延迟5秒启动，确保服务已完全就绪
		time.Sleep(5 * time.Second)

		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		done := make(chan os.Signal, 1)
		signal.Notify(done, syscall.SIGINT, syscall.SIGTERM)

		for {
			select {
			case <-ticker.C:
				router.FlashService.ScanExpiredOrders(context.Background())
			case <-done:
				log.Println("超时扫描器已停止")
				return
			}
		}
	}()

	// 创建 HTTP Server 实例（支持优雅关闭）
	addr := fmt.Sprintf(":%d", router.Config.Server.Port)
	srv := &http.Server{
		Addr:    addr,
		Handler: router.Setup(),
	}

	// 在独立 goroutine 中启动服务器
	go func() {
		fmt.Printf("✅ 服务器启动成功，监听端口: %d\n", router.Config.Server.Port)
		fmt.Printf("🌐 访问地址: http://localhost:%d\n", router.Config.Server.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			panic(fmt.Sprintf("服务器启动失败: %v", err))
		}
	}()

	// 监听操作系统退出信号（实现优雅关闭）
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit
	fmt.Printf("\n🛑 收到 %v 信号，开始优雅关闭...\n", sig)

	// 设置 10 秒超时等待现有请求处理完毕
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("⚠️ 服务器强制关闭: %v", err)
	}

	fmt.Println("✅ 服务器已安全关闭")
}
