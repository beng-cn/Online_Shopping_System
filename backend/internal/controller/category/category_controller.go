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

// 获取所有父分类
func (c *CategoryController) GetParentCategories(ctx *gin.Context) {
	resp, err := c.categoryService.GetParentCategories()
	if err != nil {
		response.Error(ctx, err)
		return
	}

	response.Success(ctx, resp)
}

// 获取子分类
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
