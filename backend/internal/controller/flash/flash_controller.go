package flash

import (
	"backend/internal/model/dto/request"
	"backend/internal/pkg/errors"
	"backend/internal/pkg/response"
	"backend/internal/service/flash"
	"strconv"
	"time"

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

// GenerateCaptcha 生成验证码（人机验证）
// GET /api/v1/auth/flash/captcha
func (c *FlashController) GenerateCaptcha(ctx *gin.Context) {
	captcha, err := c.flashService.GenerateCaptcha()
	if err != nil {
		response.Error(ctx, err)
		return
	}
	response.Success(ctx, gin.H{
		"captcha_id":    captcha.ID,
		"captcha_image": captcha.ImageB64,
		"expires_in":    120,
	})
	_ = time.Now // 预留给未来时间戳校验
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
