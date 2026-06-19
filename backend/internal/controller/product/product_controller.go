package product

import (
	"backend/internal/model/dto/request"
	"backend/internal/pkg/errors"
	"backend/internal/pkg/response"
	"backend/internal/service/product"
	"log"
	"strconv"

	"github.com/gin-gonic/gin"
)

type ProductController struct {
	productService product.ProductService
}

// NewProductController 创建商品控制器实例
func NewProductController(productService product.ProductService) *ProductController {
	return &ProductController{productService: productService}
}

// GetProductList 分页获取商品列表
func (c *ProductController) GetProductList(ctx *gin.Context) {
	var req request.ProductListRequest

	// 从 Query String 解析参数（符合 GET 语义）
	if err := ctx.ShouldBindQuery(&req); err != nil {
		log.Printf("❌ 参数绑定失败: %v", err)
		response.Error(ctx, errors.NewParamError(err.Error()))
		return
	}

	log.Printf("✅ 商品列表查询: keyword=%s, category_id=%s, page=%d, size=%d, sort=%s",
		req.Keyword, req.CategoryID, req.PageNum, req.PageSize, req.Sort)

	// 传递完整的req对象给Service层
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
