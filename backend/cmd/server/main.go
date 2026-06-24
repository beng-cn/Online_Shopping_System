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
)

func main() {
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
				log.Printf("⚠️ 启动后初始化发生panic: %v", r)
			}
		}()
		// 延迟3秒确保数据库和Redis连接完全就绪
		time.Sleep(3 * time.Second)

		// 预热分类缓存（全量）
		if err := router.CategoryService.WarmUpAllCategories(); err != nil {
			log.Printf("⚠️ 分类缓存预热失败: %v", err)
		}

		// 初始化布隆过滤器（从数据库加载所有产品ID）
		if err := router.ProductService.InitBloomFilter(); err != nil {
			log.Printf("⚠️ 布隆过滤器初始化失败: %v", err)
		}

		// 使用配置的数量预热热门商品
		hotLimit := router.Config.Cache.HotProductWarmUpLimit
		if hotLimit <= 0 {
			hotLimit = 100 // 默认值
		}
		if err := router.ProductService.WarmUpHotProducts(hotLimit); err != nil {
			log.Printf("⚠️ 热门商品缓存预热失败: %v", err)
		}

		// 秒杀崩溃恢复
		router.FlashService.RecoverPendingOrders()
	}()

	// 秒杀自动预热扫描器（每30秒检查一次即将开始的活动）
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("⚠️ 秒杀自动预热发生panic: %v", r)
			}
		}()
		time.Sleep(5 * time.Second)
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			router.FlashService.AutoWarmUp()
		}
	}()

	// 用户限流器定期清理（每5分钟清理超过30分钟未活动的用户桶，防止内存泄漏）
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("⚠️ 限流器清理发生panic: %v", r)
			}
		}()
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			router.UserSnatchLimiter.CleanupExpired(30 * time.Minute)
		}
	}()

	// 秒杀一致性守护器：崩溃恢复 + 库存对账，每5秒执行一次
	// 合并维护，两套逻辑共用同一扫描周期
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("⚠️ 秒杀一致性守护器发生panic: %v", r)
			}
		}()
		time.Sleep(5 * time.Second) // 延迟5秒，确保服务完全就绪

		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		for range ticker.C {
			router.FlashService.GuardConsistency()
		}
	}()

	// 布隆过滤器定时重建（每1小时全量重建，清理已删除产品的残留ID）
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("⚠️ 布隆过滤器定时重建发生panic: %v", r)
			}
		}()
		time.Sleep(2 * time.Hour) // 首次重建延迟2小时（等启动时的首次加载完成后再周期性重建）
		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()
		for range ticker.C {
			if err := router.ProductService.InitBloomFilter(); err != nil {
				log.Printf("⚠️ 布隆过滤器定时重建失败: %v", err)
			}
		}
	}()

	// 秒杀超时订单扫描器（每30秒执行一次）
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("⚠️ 秒杀超时扫描器发生panic: %v", r)
			}
		}()
		// 延迟5秒启动，确保服务已完全就绪
		time.Sleep(5 * time.Second)

		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for range ticker.C {
			router.FlashService.ScanExpiredOrders()
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
