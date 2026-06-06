//go:build wireinject
// +build wireinject

package main

import (
	"backend/internal/config"
	"backend/internal/controller/admin"

	// 给Controller层的包加 Ctrl 后缀别名
	cartCtrl "backend/internal/controller/cart"
	categoryCtrl "backend/internal/controller/category"
	orderCtrl "backend/internal/controller/order"
	productCtrl "backend/internal/controller/product"
	userCtrl "backend/internal/controller/user"
	"backend/internal/repository/mysql"
	"backend/internal/repository/redis"
	"backend/internal/router"

	// 给Service层的包加 Svc 后缀别名
	"backend/internal/pkg/jwt"
	cartSvc "backend/internal/service/cart"
	categorySvc "backend/internal/service/category"
	orderSvc "backend/internal/service/order"
	"backend/internal/service/payment"
	productSvc "backend/internal/service/product"
	userSvc "backend/internal/service/user"

	"github.com/google/wire"
)

func InitApp() (*router.Router, error) {
	wire.Build(
		// 配置加载
		config.Load,

		// 数据库连接
		mysql.InitDB,
		redis.InitRedis,

		// 工具类
		jwt.NewJWTUtil,

		// MySQL Repository
		mysql.NewUserRepository,
		mysql.NewProductRepository,
		mysql.NewCartRepository,
		mysql.NewOrderRepository,
		mysql.NewOrderItemRepository,
		mysql.NewCategoryRepository,

		// Redis Cache
		redis.NewProductCache,
		redis.NewCategoryCache,

		// Service
		userSvc.NewUserService,
		productSvc.NewProductService,
		cartSvc.NewCartService,
		orderSvc.NewOrderService,
		categorySvc.NewCategoryService,
		payment.NewAlipayService,

		// Controller
		userCtrl.NewUserController,
		productCtrl.NewProductController,
		cartCtrl.NewCartController,
		orderCtrl.NewOrderController,
		categoryCtrl.NewCategoryController,
		admin.NewAdminController,

		// Router
		router.NewRouter,
	)
	return nil, nil
}
