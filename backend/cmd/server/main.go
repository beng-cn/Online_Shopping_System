package main

import (
	"fmt"
	"log"
	"time"
)

func main() {
	router, err := InitApp()
	if err != nil {
		panic(fmt.Sprintf("应用初始化失败: %v", err))
	}

	// 异步执行缓存预热（不阻塞服务启动）
	go func() {
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
	}()

	// 启动HTTP服务
	fmt.Printf("✅ 服务器启动成功，监听端口: %d\n", router.Config.Server.Port)
	fmt.Printf("🌐 访问地址: http://localhost:%d\n", router.Config.Server.Port)
	if err := router.Setup().Run(fmt.Sprintf(":%d", router.Config.Server.Port)); err != nil {
		panic(fmt.Sprintf("服务器启动失败: %v", err))
	}
}
