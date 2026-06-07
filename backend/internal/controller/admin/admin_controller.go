package admin

import (
	"backend/internal/model/dto/request"
	"backend/internal/pkg/errors"
	"backend/internal/pkg/response"
	"backend/internal/service/category"
	"backend/internal/service/product"
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
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

// 上传图片（安全增强版）
func (c *AdminController) UploadImage(ctx *gin.Context) {
	// 1. 解析上传文件（最大10MB）
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

	// 2. 通过文件魔数检测真实类型（不信任 Content-Type 头）
	// 读取文件头部 512 字节用于类型检测，与 http.DetectContentType 保持一致
	head := make([]byte, 512)
	n, _ := io.ReadFull(file, head)
	if n == 0 {
		response.Error(ctx, errors.NewParamError("无法读取文件内容"))
		return
	}
	head = head[:n]

	detectedType := http.DetectContentType(head)
	allowedTypes := map[string]bool{
		"image/jpeg": true,
		"image/png":  true,
		"image/gif":  true,
	}
	if !allowedTypes[detectedType] {
		response.Error(ctx, errors.NewParamError("仅支持上传 jpg、png、gif 格式的图片"))
		return
	}

	// 重置文件指针到开头，以便后续完整写入
	var fileReader io.Reader
	if seeker, ok := file.(io.Seeker); ok {
		seeker.Seek(0, io.SeekStart)
		fileReader = file
	} else {
		// 无法 seek 时，将已读的头部 + 剩余内容拼接
		fileReader = io.MultiReader(bytes.NewReader(head), file)
	}

	// 3. 安全的文件扩展名处理
	ext := strings.ToLower(filepath.Ext(handler.Filename))
	detectedExt := map[string]string{
		"image/jpeg": ".jpg",
		"image/png":  ".png",
		"image/gif":  ".gif",
	}[detectedType]
	// 以魔数检测结果为准，忽略用户传入的扩展名
	if detectedExt != "" {
		ext = detectedExt
	}

	// 4. 创建上传目录
	uploadDir := "./uploads"
	if _, err := os.Stat(uploadDir); os.IsNotExist(err) {
		if err := os.MkdirAll(uploadDir, 0755); err != nil {
			response.Error(ctx, errors.Wrap(err, "创建上传目录失败"))
			return
		}
	}

	// 5. 生成并发安全的唯一文件名（时间戳 + 16位加密随机十六进制）
	randomBytes := make([]byte, 8)
	_, _ = rand.Read(randomBytes)
	filename := fmt.Sprintf("%d%s%s", time.Now().UnixNano(), hex.EncodeToString(randomBytes), ext)
	filePath := filepath.Join(uploadDir, filename)

	// 6. 保存文件
	dst, err := os.Create(filePath)
	if err != nil {
		response.Error(ctx, errors.Wrap(err, "保存文件失败"))
		return
	}
	defer dst.Close()

	if _, err := io.Copy(dst, fileReader); err != nil {
		response.Error(ctx, errors.Wrap(err, "保存文件失败"))
		return
	}

	// 7. 生成访问URL
	imageURL := fmt.Sprintf("http://localhost:8080/uploads/%s", filename)
	response.Success(ctx, gin.H{"url": imageURL})
}

// 批量生成商品搜索关键词（管理员操作，为存量商品补充关键词）
func (c *AdminController) BatchGenerateKeywords(ctx *gin.Context) {
	count, err := c.productService.BatchGenerateKeywords()
	if err != nil {
		response.Error(ctx, err)
		return
	}
	response.Success(ctx, gin.H{"generated_count": count, "message": fmt.Sprintf("成功为 %d 个商品生成关键词", count)})
}
