package product

import (
	"backend/internal/pkg/errors"
	"backend/internal/pkg/response"
	"backend/internal/service/product"
	"strconv"

	"github.com/gin-gonic/gin"
)

type ProductController struct {
	productService product.ProductService
}

func NewProductController(productService product.ProductService) *ProductController {
	return &ProductController{productService: productService}
}

// 获取商品列表
func (c *ProductController) GetProductList(ctx *gin.Context) {
	keyword := ctx.Query("keyword")
	categoryID := ctx.Query("category_id")

	resp, err := c.productService.GetProductList(keyword, categoryID)
	if err != nil {
		response.Error(ctx, err)
		return
	}

	response.Success(ctx, resp)
}

// 获取商品详情
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
