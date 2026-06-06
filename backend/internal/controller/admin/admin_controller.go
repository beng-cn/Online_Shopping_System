package admin

import (
	"backend/internal/model/dto/request"
	"backend/internal/pkg/errors"
	"backend/internal/pkg/response"
	"backend/internal/service/category"
	"backend/internal/service/product"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

type AdminController struct {
	productService  product.ProductService
	categoryService category.CategoryService
}

func NewAdminController(
	productService product.ProductService,
	categoryService category.CategoryService,
) *AdminController {
	return &AdminController{
		productService:  productService,
		categoryService: categoryService,
	}
}

// 创建商品
func (c *AdminController) CreateProduct(ctx *gin.Context) {
	var req request.CreateProductRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		response.Error(ctx, errors.NewParamError(err.Error()))
		return
	}

	resp, err := c.productService.CreateProduct(&req)
	if err != nil {
		response.Error(ctx, err)
		return
	}

	response.Success(ctx, resp)
}

// 更新商品
func (c *AdminController) UpdateProduct(ctx *gin.Context) {
	idStr := ctx.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil || id <= 0 {
		response.Error(ctx, errors.NewParamError("商品ID无效"))
		return
	}

	var req request.UpdateProductRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		response.Error(ctx, errors.NewParamError(err.Error()))
		return
	}

	if err := c.productService.UpdateProduct(uint(id), &req); err != nil {
		response.Error(ctx, err)
		return
	}

	response.Success(ctx, nil)
}

// 删除商品
func (c *AdminController) DeleteProduct(ctx *gin.Context) {
	idStr := ctx.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil || id <= 0 {
		response.Error(ctx, errors.NewParamError("商品ID无效"))
		return
	}

	if err := c.productService.DeleteProduct(uint(id)); err != nil {
		response.Error(ctx, err)
		return
	}

	response.Success(ctx, nil)
}

// 创建分类
func (c *AdminController) CreateCategory(ctx *gin.Context) {
	var req request.CreateCategoryRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		response.Error(ctx, errors.NewParamError(err.Error()))
		return
	}

	resp, err := c.categoryService.CreateCategory(&req)
	if err != nil {
		response.Error(ctx, err)
		return
	}

	response.Success(ctx, resp)
}

// 更新分类
func (c *AdminController) UpdateCategory(ctx *gin.Context) {
	idStr := ctx.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil || id <= 0 {
		response.Error(ctx, errors.NewParamError("分类ID无效"))
		return
	}

	var req request.UpdateCategoryRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		response.Error(ctx, errors.NewParamError(err.Error()))
		return
	}

	if err := c.categoryService.UpdateCategory(uint(id), &req); err != nil {
		response.Error(ctx, err)
		return
	}

	response.Success(ctx, nil)
}

// 删除分类
func (c *AdminController) DeleteCategory(ctx *gin.Context) {
	idStr := ctx.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil || id <= 0 {
		response.Error(ctx, errors.NewParamError("分类ID无效"))
		return
	}

	if err := c.categoryService.DeleteCategory(uint(id)); err != nil {
		response.Error(ctx, err)
		return
	}

	response.Success(ctx, nil)
}

// 上传图片
func (c *AdminController) UploadImage(ctx *gin.Context) {
	// 解析上传文件（最大10MB）
	if err := ctx.Request.ParseMultipartForm(10 << 20); err != nil {
		response.Error(ctx, errors.NewParamError("文件大小不能超过10MB"))
		return
	}

	file, handler, err := ctx.Request.FormFile("image")
	if err != nil {
		response.Error(ctx, errors.NewParamError("获取上传文件失败："+err.Error()))
		return
	}
	defer file.Close()

	// 校验文件类型
	allowedTypes := map[string]bool{
		"image/jpeg": true,
		"image/jpg":  true,
		"image/png":  true,
		"image/gif":  true,
	}
	contentType := handler.Header.Get("Content-Type")
	if !allowedTypes[contentType] {
		response.Error(ctx, errors.NewParamError("仅支持上传 jpg、png、gif 格式的图片"))
		return
	}

	// 创建上传目录
	uploadDir := "./uploads"
	if _, err := os.Stat(uploadDir); os.IsNotExist(err) {
		if err := os.MkdirAll(uploadDir, 0755); err != nil {
			response.Error(ctx, errors.Wrap(err, "创建上传目录失败"))
			return
		}
	}

	// 生成唯一文件名
	ext := filepath.Ext(handler.Filename)
	filename := fmt.Sprintf("%d%s", time.Now().UnixNano(), ext)
	filePath := filepath.Join(uploadDir, filename)

	// 保存文件
	dst, err := os.Create(filePath)
	if err != nil {
		response.Error(ctx, errors.Wrap(err, "保存文件失败"))
		return
	}
	defer dst.Close()

	if _, err := dst.ReadFrom(file); err != nil {
		response.Error(ctx, errors.Wrap(err, "保存文件失败"))
		return
	}

	// 生成访问URL
	imageURL := fmt.Sprintf("http://localhost:8080/uploads/%s", filename)
	response.Success(ctx, gin.H{"url": imageURL})
}
