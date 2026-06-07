package router

import (
	"backend/internal/config"
	"backend/internal/controller/admin"
	cartCtrl "backend/internal/controller/cart"
	categoryCtrl "backend/internal/controller/category"
	orderCtrl "backend/internal/controller/order"
	productCtrl "backend/internal/controller/product"
	userCtrl "backend/internal/controller/user"
	"backend/internal/middleware"
	"backend/internal/pkg/errors"
	"backend/internal/pkg/jwt"
	"backend/internal/service/category"
	"backend/internal/service/product"
	"log"
	"net/http"
	"runtime/debug"
	"time"

	"github.com/gin-gonic/gin"
)

// 路由管理器
type Router struct {
	Config             *config.AppConfig // 配置实例（公开字段）
	JWTUtil            *jwt.JWTUtil      // 添加JWT工具实例（公开字段）
	UserController     *userCtrl.UserController
	ProductController  *productCtrl.ProductController
	CartController     *cartCtrl.CartController
	OrderController    *orderCtrl.OrderController
	CategoryController *categoryCtrl.CategoryController
	AdminController    *admin.AdminController
	ProductService     product.ProductService   // 新增服务引用
	CategoryService    category.CategoryService // 新增服务引用
}

// 创建路由管理器实例
func NewRouter(
	cfg *config.AppConfig,
	jwtUtil *jwt.JWTUtil, // 添加JWTUtil参数
	userController *userCtrl.UserController,
	productController *productCtrl.ProductController,
	cartController *cartCtrl.CartController,
	orderController *orderCtrl.OrderController,
	categoryController *categoryCtrl.CategoryController,
	adminController *admin.AdminController,
	productService product.ProductService, // 新增参数
	categoryService category.CategoryService, // 新增参数
) *Router {
	return &Router{
		Config:             cfg,
		JWTUtil:            jwtUtil, // 赋值JWTUtil
		UserController:     userController,
		ProductController:  productController,
		CartController:     cartController,
		OrderController:    orderController,
		CategoryController: categoryController,
		AdminController:    adminController,
		ProductService:     productService,  // 赋值
		CategoryService:    categoryService, // 赋值
	}
}

// 配置所有路由
func (r *Router) Setup() *gin.Engine {
	engine := gin.New()
	engine.SetTrustedProxies([]string{"127.0.0.1", "::1"})

	// 全局中间件
	engine.Use(gin.Logger())
	engine.Use(middleware.Cors())
	engine.Use(CustomRecovery())

	// 全局限流：每秒200请求，突发上限300（可根据业务调整）
	limiter := middleware.NewRateLimiter(200, 300)
	engine.Use(limiter.Handler())

	// 健康检查端点（用于 K8s / 负载均衡器探活）
	engine.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status": "ok",
			"time":   time.Now().Format("2006-01-02 15:04:05"),
		})
	})

	// 静态文件服务
	engine.Static("/uploads", "./uploads")

	// 公开接口组（无需登录）
	public := engine.Group("/api")
	{
		// 用户相关
		userGroup := public.Group("/user")
		{
			userGroup.POST("/register", r.UserController.Register)
			userGroup.POST("/login", r.UserController.Login)
		}

		// 商品相关
		productGroup := public.Group("/product")
		{
			productGroup.POST("/list", r.ProductController.GetProductList)
			productGroup.GET("/:id", r.ProductController.GetProductDetail)
			productGroup.GET("/category/parents", r.CategoryController.GetParentCategories)
			productGroup.GET("/category/children", r.CategoryController.GetChildCategories)
		}

		// 支付宝回调
		alipayGroup := public.Group("/alipay")
		{
			alipayGroup.POST("/notify", r.OrderController.AliPayNotify)
			alipayGroup.GET("/success", r.OrderController.AliPayReturn)
		}
	}

	// 需要普通用户登录的接口组
	auth := engine.Group("/api/auth")
	auth.Use(middleware.Auth(r.JWTUtil)) // 传入注入的JWTUtil
	{
		// 用户信息
		auth.PUT("/user/info", r.UserController.UpdateUserInfo)
		auth.GET("/user/info", r.UserController.GetUserInfo)

		// 购物车
		cartGroup := auth.Group("/cart")
		{
			cartGroup.GET("/list", r.CartController.GetCartList)
			cartGroup.POST("/add", r.CartController.AddToCart)
			cartGroup.PUT("/:id", r.CartController.UpdateCartQuantity)
			cartGroup.DELETE("/:id", r.CartController.DeleteCartItem)
		}

		// 订单
		orderGroup := auth.Group("/order")
		{
			orderGroup.POST("/create", r.OrderController.CreateOrder)
			orderGroup.GET("/list", r.OrderController.GetOrderList)
			orderGroup.POST("/alipay", r.OrderController.AliPayUnifiedOrder)
			orderGroup.GET("/items/:id", r.OrderController.GetOrderItems)
			orderGroup.DELETE("/delete/:id", r.OrderController.DeleteOrder)
		}
	}

	// 需要管理员权限的接口组
	adminGroup := engine.Group("/api/admin")
	adminGroup.Use(middleware.AdminAuth(r.JWTUtil)) // 传入注入的JWTUtil
	{
		// 商品管理
		adminGroup.POST("/product", r.AdminController.CreateProduct)
		adminGroup.PUT("/product/:id", r.AdminController.UpdateProduct)
		adminGroup.DELETE("/product/:id", r.AdminController.DeleteProduct)

		// 分类管理
		adminGroup.POST("/category/add", r.AdminController.CreateCategory)
		adminGroup.PUT("/category/:id", r.AdminController.UpdateCategory)
		adminGroup.DELETE("/category/:id", r.AdminController.DeleteCategory)

		// 图片上传
		adminGroup.POST("/upload", r.AdminController.UploadImage)

		// 搜索优化
		adminGroup.POST("/product/batch-keywords", r.AdminController.BatchGenerateKeywords)

		// 用户管理
		adminGroup.GET("/user/list", r.UserController.ListUsers)
		adminGroup.PUT("/user/:id/status", r.UserController.UpdateUserStatus)
		adminGroup.PUT("/user/:id/reset-password", r.UserController.ResetUserPassword)
		adminGroup.DELETE("/user/:id", r.UserController.DeleteUser)
	}

	return engine
}

// 错误处理
func CustomRecovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				// 生产环境必须打印完整堆栈，便于问题排查
				log.Printf("\n\n【=== 系统异常 ===】\n")
				log.Printf("异常类型: %T\n", err)
				log.Printf("异常信息: %v\n", err)
				log.Printf("异常堆栈:\n%s\n", debug.Stack())
				log.Printf("【===============】\n\n")

				// 只在未写入响应头时返回错误
				if !c.Writer.Written() {
					c.JSON(http.StatusOK, gin.H{
						"code":    errors.CodeServerError,
						"message": "服务器内部错误",
						"data":    nil,
					})
				}
				c.Abort()
			}
		}()
		c.Next()
	}
}
