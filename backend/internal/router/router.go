package router

import (
	"backend/internal/config"
	"backend/internal/controller/admin"
	cartCtrl "backend/internal/controller/cart"
	categoryCtrl "backend/internal/controller/category"
	flashCtrl "backend/internal/controller/flash"
	orderCtrl "backend/internal/controller/order"
	productCtrl "backend/internal/controller/product"
	userCtrl "backend/internal/controller/user"
	"backend/internal/middleware"
	"backend/internal/pkg/errors"
	"backend/internal/pkg/flashlimiter"
	"backend/internal/pkg/jwt"
	"backend/internal/service/category"
	"backend/internal/service/flash"
	"backend/internal/service/product"
	"log"
	"net/http"
	"runtime/debug"
	"time"

	"github.com/gin-gonic/gin"
)

// 路由管理器
type Router struct {
	Config               *config.AppConfig
	JWTUtil              *jwt.JWTUtil
	UserController       *userCtrl.UserController
	ProductController    *productCtrl.ProductController
	CartController       *cartCtrl.CartController
	OrderController      *orderCtrl.OrderController
	CategoryController   *categoryCtrl.CategoryController
	AdminController      *admin.AdminController
	FlashController      *flashCtrl.FlashController
	AdminFlashController *flashCtrl.AdminFlashController
	ProductService       product.ProductService
	CategoryService      category.CategoryService
	FlashService         flash.FlashService
	UserSnatchLimiter    *flashlimiter.UserRateLimiter
}

func NewRouter(
	cfg *config.AppConfig,
	jwtUtil *jwt.JWTUtil,
	userController *userCtrl.UserController,
	productController *productCtrl.ProductController,
	cartController *cartCtrl.CartController,
	orderController *orderCtrl.OrderController,
	categoryController *categoryCtrl.CategoryController,
	adminController *admin.AdminController,
	flashController *flashCtrl.FlashController,
	adminFlashController *flashCtrl.AdminFlashController,
	productService product.ProductService,
	categoryService category.CategoryService,
	flashService flash.FlashService,
) *Router {
	snatchLimiter := flashlimiter.NewUserRateLimiter(1, 1)
	flashService.SetUserLimiter(snatchLimiter)

	return &Router{
		Config:               cfg,
		JWTUtil:              jwtUtil,
		UserController:       userController,
		ProductController:    productController,
		CartController:       cartController,
		OrderController:      orderController,
		CategoryController:   categoryController,
		AdminController:      adminController,
		FlashController:      flashController,
		AdminFlashController: adminFlashController,
		ProductService:       productService,
		CategoryService:      categoryService,
		FlashService:         flashService,
		UserSnatchLimiter:    snatchLimiter,
	}
}

func (r *Router) Setup() *gin.Engine {
	engine := gin.New()
	engine.SetTrustedProxies([]string{"127.0.0.1", "::1"})

	engine.Use(gin.Logger())
	engine.Use(middleware.Cors())
	engine.Use(CustomRecovery())

	limiter := middleware.NewRateLimiter(200, 300)
	engine.Use(limiter.Handler())

	engine.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status": "ok",
			"time":   time.Now().Format("2006-01-02 15:04:05"),
		})
	})

	engine.Static("/uploads", "./uploads")

	// === 公开接口（无需登录）===
	public := engine.Group("/api")
	{
		public.POST("/user/register", r.UserController.Register)
		public.POST("/user/login", r.UserController.Login)
		public.POST("/user/forgot-password", r.UserController.ForgotPassword)

		productGroup := public.Group("/product")
		{
			productGroup.POST("/list", r.ProductController.GetProductList)
			productGroup.GET("/:id", r.ProductController.GetProductDetail)
			productGroup.GET("/category/parents", r.CategoryController.GetParentCategories)
			productGroup.GET("/category/children", r.CategoryController.GetChildCategories)
		}

		public.GET("/flash/list", r.FlashController.ListActiveFlashSales)
		public.GET("/flash/:id", r.FlashController.GetFlashSaleDetail)

		public.POST("/alipay/notify", r.OrderController.AliPayNotify)
		public.GET("/alipay/success", r.OrderController.AliPayReturn)
	}

	// === 需登录接口 ===
	auth := engine.Group("/api/auth")
	auth.Use(middleware.Auth(r.JWTUtil))
	{
		auth.PUT("/user/info", r.UserController.UpdateUserInfo)
		auth.GET("/user/info", r.UserController.GetUserInfo)

		cartGroup := auth.Group("/cart")
		{
			cartGroup.GET("/list", r.CartController.GetCartList)
			cartGroup.POST("/add", r.CartController.AddToCart)
			cartGroup.PUT("/:id", r.CartController.UpdateCartQuantity)
			cartGroup.DELETE("/:id", r.CartController.DeleteCartItem)
		}

		orderGroup := auth.Group("/order")
		{
			orderGroup.POST("/create", r.OrderController.CreateOrder)
			orderGroup.GET("/list", r.OrderController.GetOrderList)
			orderGroup.POST("/alipay", r.OrderController.AliPayUnifiedOrder)
			orderGroup.GET("/items/:id", r.OrderController.GetOrderItems)
			orderGroup.DELETE("/delete/:id", r.OrderController.DeleteOrder)
		}

		flashAuthGroup := auth.Group("/flash")
		{
			flashAuthGroup.POST("/enter", r.FlashController.EnterFlashSale)
			flashAuthGroup.POST("/snatch", r.FlashController.SnatchFlashSale)
			flashAuthGroup.GET("/orders", r.FlashController.GetUserFlashOrders)
		}
	}

	// === 管理员接口 ===
	adminGroup := engine.Group("/api/admin")
	adminGroup.Use(middleware.AdminAuth(r.JWTUtil))
	{
		adminGroup.POST("/product", r.AdminController.CreateProduct)
		adminGroup.PUT("/product/:id", r.AdminController.UpdateProduct)
		adminGroup.DELETE("/product/:id", r.AdminController.DeleteProduct)

		adminGroup.POST("/category/add", r.AdminController.CreateCategory)
		adminGroup.PUT("/category/:id", r.AdminController.UpdateCategory)
		adminGroup.DELETE("/category/:id", r.AdminController.DeleteCategory)

		adminGroup.POST("/upload", r.AdminController.UploadImage)
		adminGroup.POST("/product/batch-keywords", r.AdminController.BatchGenerateKeywords)

		adminGroup.GET("/user/list", r.UserController.ListUsers)
		adminGroup.PUT("/user/:id/status", r.UserController.UpdateUserStatus)
		adminGroup.PUT("/user/:id/reset-password", r.UserController.ResetUserPassword)
		adminGroup.DELETE("/user/:id", r.UserController.DeleteUser)

		// 管理员安全验证（PIN二次验证）
		adminGroup.POST("/verify-pin", r.UserController.VerifyAdminPin)
		adminGroup.POST("/set-pin", r.UserController.SetAdminPin)

		flashAdminGroup := adminGroup.Group("/flash")
		{
			flashAdminGroup.POST("", r.AdminFlashController.CreateFlashSale)
			flashAdminGroup.PUT("/:id", r.AdminFlashController.UpdateFlashSale)
			flashAdminGroup.POST("/:id/warmup", r.AdminFlashController.WarmUpFlashSale)
			flashAdminGroup.POST("/:id/end", r.AdminFlashController.EndFlashSale)
			flashAdminGroup.GET("/list", r.AdminFlashController.ListAllFlashSales)
		}
	}

	return engine
}

func CustomRecovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				log.Printf("\n\n【=== 系统异常 ===】\n")
				log.Printf("异常类型: %T\n", err)
				log.Printf("异常信息: %v\n", err)
				log.Printf("异常堆栈:\n%s\n", debug.Stack())
				log.Printf("【===============】\n\n")

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
