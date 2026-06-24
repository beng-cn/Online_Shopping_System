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
// @Summary 分页获取商品列表
// @Description 支持关键词搜索、分类筛选、价格区间过滤和多种排序方式的分页商品列表查询
// @Tags 商品
// @Accept json
// @Produce json
// @Param keyword query string false "搜索关键词（最长50字符）"
// @Param category_id query string false "分类ID（数字）"
// @Param page_num query int false "页码（默认1）" default(1)
// @Param page_size query int false "每页条数（1-100，默认20）" default(20)
// @Param sort query string false "排序方式：created_at | sales | price_asc | price_desc" Enums(created_at, sales, price_asc, price_desc)
// @Param min_price query number false "最低价格"
// @Param max_price query number false "最高价格"
// @Success 200 {object} response.Response "查询成功"
// @Failure 400 {object} response.Response "参数错误"
// @Router /product/list [get]
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

// GetProductDetail 获取商品详情
// @Summary 获取商品详情
// @Description 根据商品ID获取商品详细信息，包含分类、价格、库存、关键词等
// @Tags 商品
// @Accept json
// @Produce json
// @Param id path int true "商品ID"
// @Success 200 {object} response.Response "查询成功"
// @Failure 400 {object} response.Response "商品ID无效"
// @Failure 404 {object} response.Response "商品不存在"
// @Router /product/{id} [get]
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
