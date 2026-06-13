package flash

import (
	"backend/internal/model/dto/request"
	"backend/internal/pkg/errors"
	"backend/internal/pkg/response"
	"backend/internal/service/flash"
	"strconv"

	"github.com/gin-gonic/gin"
)

// FlashController 秒杀用户端控制器
type FlashController struct {
	flashService flash.FlashService
}

// NewFlashController 创建秒杀用户端控制器（供 Wire 注入）
func NewFlashController(flashService flash.FlashService) *FlashController {
	return &FlashController{flashService: flashService}
}

// ListActiveFlashSales 获取进行中的秒杀活动列表
func (c *FlashController) ListActiveFlashSales(ctx *gin.Context) {
	resp, err := c.flashService.ListActiveFlashSales()
	if err != nil {
		response.Error(ctx, err)
		return
	}
	response.Success(ctx, resp)
}

// GetFlashSaleDetail 获取秒杀活动详情
func (c *FlashController) GetFlashSaleDetail(ctx *gin.Context) {
	idStr := ctx.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil || id <= 0 {
		response.Error(ctx, errors.NewParamError("活动ID无效"))
		return
	}

	resp, err := c.flashService.GetFlashSaleDetail(uint(id))
	if err != nil {
		response.Error(ctx, err)
		return
	}
	response.Success(ctx, resp)
}

// EnterFlashSale 排队入场
func (c *FlashController) EnterFlashSale(ctx *gin.Context) {
	var req request.FlashEnterRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		response.Error(ctx, errors.NewParamError(err.Error()))
		return
	}

	userID := ctx.GetUint("user_id")
	resp, err := c.flashService.EnterFlashSale(userID, &req)
	if err != nil {
		response.Error(ctx, err)
		return
	}
	response.Success(ctx, resp)
}

// SnatchFlashSale 秒杀抢购（核心接口）
func (c *FlashController) SnatchFlashSale(ctx *gin.Context) {
	var req request.FlashSnatchRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		response.Error(ctx, errors.NewParamError(err.Error()))
		return
	}

	userID := ctx.GetUint("user_id")
	resp, err := c.flashService.SnatchFlashSale(userID, &req)
	if err != nil {
		response.Error(ctx, err)
		return
	}
	response.Success(ctx, resp)
}

// GetUserFlashOrders 获取用户秒杀订单列表
func (c *FlashController) GetUserFlashOrders(ctx *gin.Context) {
	userID := ctx.GetUint("user_id")
	resp, err := c.flashService.GetUserFlashOrders(userID)
	if err != nil {
		response.Error(ctx, err)
		return
	}
	response.Success(ctx, resp)
}
