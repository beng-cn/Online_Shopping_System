//go:build wireinject
// +build wireinject

package main

import (
	"backend/internal/config"
	"backend/internal/controller/admin"

	// 给Controller层的包加 Ctrl 后缀别名
	cartCtrl "backend/internal/controller/cart"
	categoryCtrl "backend/internal/controller/category"
	flashCtrl "backend/internal/controller/flash"
	orderCtrl "backend/internal/controller/order"
	productCtrl "backend/internal/controller/product"
	userCtrl "backend/internal/controller/user"
	"backend/internal/repository/mysql"
	"backend/internal/repository/redis"
	"backend/internal/router"

	// 给Service层的包加 Svc 后缀别名
	"backend/internal/pkg/bloom"
	"backend/internal/pkg/breaker"
	"backend/internal/pkg/jwt"
	"backend/internal/pkg/localcache"
	"backend/internal/pkg/semaphore"
	cartSvc "backend/internal/service/cart"
	categorySvc "backend/internal/service/category"
	flashSvc "backend/internal/service/flash"
	orderSvc "backend/internal/service/order"
	"backend/internal/service/payment"
	productSvc "backend/internal/service/product"
	userSvc "backend/internal/service/user"

	"github.com/google/wire"
)

// InitApp Wire 编译时依赖注入入口，按序组装 Config→DB→Cache→Repo→Service→Controller→Router 全链路
func InitApp() (*router.Router, error) {
	wire.Build(
		// 配置加载
		config.Load,

		// 数据库连接
		mysql.InitDB,
		redis.InitRedis,

		// 工具类
			jwt.NewJWTUtil,
			jwt.NewRedisBlacklist,  // JWT 黑名单
			bloom.NewFilter,        // 布隆过滤器（防缓存穿透）
			localcache.NewDefault,  // L1 本地缓存
			breaker.NewDefault,     // 数据库熔断器
			semaphore.NewDefault,   // 数据库并发信号量

		// MySQL Repository
		mysql.NewUserRepository,
		mysql.NewProductRepository,
		mysql.NewCartRepository,
		mysql.NewOrderRepository,
		mysql.NewOrderItemRepository,
		mysql.NewCategoryRepository,
		mysql.NewFlashRepository, // 秒杀

		// Redis Cache
		redis.NewProductCache,
		redis.NewCategoryCache,
		redis.NewFlashCache, // 秒杀

		// Service
		userSvc.NewUserService,
		productSvc.NewProductService,
		cartSvc.NewCartService,
		orderSvc.NewOrderService,
		categorySvc.NewCategoryService,
		payment.NewAlipayService,
		flashSvc.NewFlashService, // 秒杀

		// Controller
		userCtrl.NewUserController,
		productCtrl.NewProductController,
		cartCtrl.NewCartController,
		orderCtrl.NewOrderController,
		categoryCtrl.NewCategoryController,
		admin.NewAdminController,
		flashCtrl.NewFlashController,       // 秒杀用户端
		flashCtrl.NewAdminFlashController,  // 秒杀管理端

		// Router
		router.NewRouter,
	)
	return nil, nil
}
