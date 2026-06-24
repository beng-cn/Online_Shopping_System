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
// @Summary 创建秒杀活动
// @Description 管理员创建新的秒杀活动，设置秒杀商品、价格、库存和时间范围
// @Tags 秒杀管理
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param body body request.CreateFlashSaleRequest true "创建秒杀活动请求体"
// @Success 200 {object} response.Response "创建成功"
// @Failure 400 {object} response.Response "参数错误"
// @Router /admin/flash [post]
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
// @Summary 修改秒杀活动
// @Description 管理员修改已有的秒杀活动信息，可更新价格、库存、时间范围和状态
// @Tags 秒杀管理
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param id path int true "秒杀活动ID"
// @Param body body request.UpdateFlashSaleRequest true "修改秒杀活动请求体"
// @Success 200 {object} response.Response "修改成功"
// @Failure 400 {object} response.Response "参数错误"
// @Router /admin/flash/{id} [put]
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
// @Summary 预热秒杀库存
// @Description 管理员手动将秒杀活动的库存数据预热到Redis缓存中，用于应对高并发抢购场景
// @Tags 秒杀管理
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param id path int true "秒杀活动ID"
// @Success 200 {object} response.Response "预热成功"
// @Failure 400 {object} response.Response "活动ID无效"
// @Router /admin/flash/{id}/warmup [post]
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
// @Summary 强制结束秒杀
// @Description 管理员强制提前结束正在进行的秒杀活动
// @Tags 秒杀管理
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param id path int true "秒杀活动ID"
// @Success 200 {object} response.Response "秒杀已结束"
// @Failure 400 {object} response.Response "活动ID无效"
// @Router /admin/flash/{id}/end [post]
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
// @Summary 查看所有秒杀活动
// @Description 管理员查看所有秒杀活动列表（包含未开始、进行中、已结束的全部活动）
// @Tags 秒杀管理
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Success 200 {object} response.Response "查询成功"
// @Router /admin/flash/list [get]
func (c *AdminFlashController) ListAllFlashSales(ctx *gin.Context) {
	resp, err := c.flashService.ListAllFlashSales()
	if err != nil {
		response.Error(ctx, err)
		return
	}
	response.Success(ctx, resp)
}
