package flash

import (
	"backend/internal/model/dto/request"
	"backend/internal/pkg/errors"
	"backend/internal/pkg/response"
	"backend/internal/service/flash"
	"strconv"

	"github.com/gin-gonic/gin"
)

// AdminFlashController 秒杀管理端控制器
type AdminFlashController struct {
	flashService flash.FlashService
}

// NewAdminFlashController 创建秒杀管理端控制器（供 Wire 注入）
func NewAdminFlashController(flashService flash.FlashService) *AdminFlashController {
	return &AdminFlashController{flashService: flashService}
}

// CreateFlashSale 创建秒杀活动
func (c *AdminFlashController) CreateFlashSale(ctx *gin.Context) {
	var req request.CreateFlashSaleRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		response.Error(ctx, errors.NewParamError(err.Error()))
		return
	}

	flash, err := c.flashService.CreateFlashSale(&req)
	if err != nil {
		response.Error(ctx, err)
		return
	}
	response.Success(ctx, flash)
}

// UpdateFlashSale 修改秒杀活动
func (c *AdminFlashController) UpdateFlashSale(ctx *gin.Context) {
	idStr := ctx.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil || id <= 0 {
		response.Error(ctx, errors.NewParamError("活动ID无效"))
		return
	}

	var req request.UpdateFlashSaleRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		response.Error(ctx, errors.NewParamError(err.Error()))
		return
	}

	if err := c.flashService.UpdateFlashSale(uint(id), &req); err != nil {
		response.Error(ctx, err)
		return
	}
	response.Success(ctx, nil)
}

// WarmUpFlashSale 预热秒杀库存到Redis
func (c *AdminFlashController) WarmUpFlashSale(ctx *gin.Context) {
	idStr := ctx.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil || id <= 0 {
		response.Error(ctx, errors.NewParamError("活动ID无效"))
		return
	}

	if err := c.flashService.WarmUpFlashSale(uint(id)); err != nil {
		response.Error(ctx, err)
		return
	}
	response.Success(ctx, gin.H{"message": "预热成功"})
}

// EndFlashSale 强制结束秒杀
func (c *AdminFlashController) EndFlashSale(ctx *gin.Context) {
	idStr := ctx.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil || id <= 0 {
		response.Error(ctx, errors.NewParamError("活动ID无效"))
		return
	}

	if err := c.flashService.EndFlashSale(uint(id)); err != nil {
		response.Error(ctx, err)
		return
	}
	response.Success(ctx, gin.H{"message": "秒杀已结束"})
}

// ListAllFlashSales 查看所有秒杀活动
func (c *AdminFlashController) ListAllFlashSales(ctx *gin.Context) {
	resp, err := c.flashService.ListAllFlashSales()
	if err != nil {
		response.Error(ctx, err)
		return
	}
	response.Success(ctx, resp)
}
