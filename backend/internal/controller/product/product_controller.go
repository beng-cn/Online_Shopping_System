package product

import (
	"backend/internal/model/dto/request" // ✅ 新增：导入request包
	"backend/internal/pkg/errors"
	"backend/internal/pkg/response"
	"backend/internal/service/product"
	"log" // ✅ 新增：导入log包用于调试
	"strconv"

	"github.com/gin-gonic/gin"
)

type ProductController struct {
	productService product.ProductService
}

func NewProductController(productService product.ProductService) *ProductController {
	return &ProductController{productService: productService}
}

// GetProductList 分页获取商品列表（POST版本，支持JSON请求体）
func (c *ProductController) GetProductList(ctx *gin.Context) {
	var req request.ProductListRequest

	// ✅ 从JSON请求体中解析参数
	if err := ctx.ShouldBindJSON(&req); err != nil {
		log.Printf("❌ 参数绑定失败: %v", err)
		response.Error(ctx, errors.NewParamError(err.Error()))
		return
	}

	// ✅ 打印调试日志，确认参数是否正确接收
	log.Printf("✅ 收到商品列表查询请求: keyword=%s, category_id=%s, page_num=%d, page_size=%d",
		req.Keyword, req.CategoryID, req.PageNum, req.PageSize)

	// ✅ 传递完整的req对象给Service层
	resp, err := c.productService.GetProductList(&req)
	if err != nil {
		response.Error(ctx, err)
		return
	}
	response.Success(ctx, resp)
}

// 获取商品详情（保持不变）
func (c *ProductController) GetProductDetail(ctx *gin.Context) {
	idStr := ctx.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil || id <= 0 {
		response.Error(ctx, errors.NewParamError("商品ID无效"))
		return
	}

	resp, err := c.productService.GetProductByID(uint(id))
	if err != nil {
		response.Error(ctx, err)
		return
	}

	response.Success(ctx, resp)
}
