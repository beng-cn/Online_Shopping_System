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
// @Summary 获取进行中的秒杀活动列表
// @Description 无需登录，获取当前所有进行中和即将开始的秒杀活动
// @Tags 秒杀
// @Accept json
// @Produce json
// @Success 200 {object} response.Response "查询成功"
// @Router /flash/list [get]
func (c *FlashController) ListActiveFlashSales(ctx *gin.Context) {
	resp, err := c.flashService.ListActiveFlashSales()
	if err != nil {
		response.Error(ctx, err)
		return
	}
	response.Success(ctx, resp)
}

// GetFlashSaleDetail 获取秒杀活动详情
// @Summary 获取秒杀活动详情
// @Description 无需登录，获取指定秒杀活动的详细信息，包含商品详情和秒杀进度
// @Tags 秒杀
// @Accept json
// @Produce json
// @Param id path int true "秒杀活动ID"
// @Success 200 {object} response.Response "查询成功"
// @Failure 400 {object} response.Response "活动ID无效"
// @Failure 404 {object} response.Response "活动不存在"
// @Router /flash/{id} [get]
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
// @Summary 排队入场
// @Description 用户加入秒杀活动的排队队列，获取入场令牌。需要先登录
// @Tags 秒杀
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param body body request.FlashEnterRequest true "排队入场请求体"
// @Success 200 {object} response.Response "入场成功"
// @Failure 400 {object} response.Response "参数错误"
// @Failure 403 {object} response.Response "活动未开始或已结束"
// @Router /auth/flash/enter [post]
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
// @Summary 秒杀抢购
// @Description 用户提交秒杀抢购请求，需先通过排队入场并完成验证码验证。需要先登录
// @Tags 秒杀
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param body body request.FlashSnatchRequest true "秒杀抢购请求体"
// @Success 200 {object} response.Response "抢购成功"
// @Failure 400 {object} response.Response "参数错误"
// @Failure 403 {object} response.Response "验证码错误或库存不足"
// @Router /auth/flash/snatch [post]
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
// @Summary 生成验证码
// @Description 生成秒杀场景下的图形验证码，用于防刷。需要先登录，验证码有效期120秒
// @Tags 秒杀
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Success 200 {object} response.Response "生成成功，返回captcha_id、captcha_image(base64)和expires_in"
// @Router /auth/flash/captcha [get]
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
// @Summary 获取用户秒杀订单
// @Description 获取当前登录用户的所有秒杀订单记录。需要先登录
// @Tags 秒杀
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Success 200 {object} response.Response "查询成功"
// @Router /auth/flash/orders [get]
func (c *FlashController) GetUserFlashOrders(ctx *gin.Context) {
	userID := ctx.GetUint("user_id")
	resp, err := c.flashService.GetUserFlashOrders(userID)
	if err != nil {
		response.Error(ctx, err)
		return
	}
	response.Success(ctx, resp)
}
