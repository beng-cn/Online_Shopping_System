package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
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
		// 延迟1秒确保数据库和Redis连接完全就绪
		time.Sleep(1 * time.Second)

		// 预热分类缓存（全量）
		if err := router.CategoryService.WarmUpAllCategories(); err != nil {
			log.Printf("⚠️ 分类缓存预热失败: %v", err)
		}

		// 使用配置的数量预热热门商品
		hotLimit := router.Config.Cache.HotProductWarmUpLimit
		if hotLimit <= 0 {
			hotLimit = 100 // 默认值
		}
		if err := router.ProductService.WarmUpHotProducts(hotLimit); err != nil {
			log.Printf("⚠️ 热门商品缓存预热失败: %v", err)
		}

		// 秒杀崩溃恢复：处理上次非正常退出遗留的防丢失记录
		router.FlashService.RecoverPendingOrders()
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
