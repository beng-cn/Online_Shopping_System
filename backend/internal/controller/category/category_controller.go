package category

import (
	"backend/internal/pkg/errors"
	"backend/internal/pkg/response"
	"backend/internal/service/category"
	"strconv"

	"github.com/gin-gonic/gin"
)

type CategoryController struct {
	categoryService category.CategoryService
}

// NewCategoryController 创建分类控制器实例
func NewCategoryController(categoryService category.CategoryService) *CategoryController {
	return &CategoryController{categoryService: categoryService}
}

// GetParentCategories 获取所有父分类
// @Summary 获取所有父分类
// @Description 获取所有顶级商品分类列表（parent_id 为 0 的分类）
// @Tags 分类
// @Accept json
// @Produce json
// @Success 200 {object} response.Response "查询成功"
// @Router /product/category/parents [get]
func (c *CategoryController) GetParentCategories(ctx *gin.Context) {
	resp, err := c.categoryService.GetParentCategories()
	if err != nil {
		response.Error(ctx, err)
		return
	}

	response.Success(ctx, resp)
}

// GetChildCategories 获取子分类
// @Summary 获取子分类
// @Description 根据父分类ID获取其下所有子分类列表
// @Tags 分类
// @Accept json
// @Produce json
// @Param parent_id query int true "父分类ID"
// @Success 200 {object} response.Response "查询成功"
// @Failure 400 {object} response.Response "父分类ID无效"
// @Router /product/category/children [get]
func (c *CategoryController) GetChildCategories(ctx *gin.Context) {
	parentIDStr := ctx.Query("parent_id")
	parentID, err := strconv.Atoi(parentIDStr)
	if err != nil || parentID < 0 {
		response.Error(ctx, errors.NewParamError("父分类ID无效"))
		return
	}

	resp, err := c.categoryService.GetChildCategories(uint(parentID))
	if err != nil {
		response.Error(ctx, err)
		return
	}

	response.Success(ctx, resp)
}
