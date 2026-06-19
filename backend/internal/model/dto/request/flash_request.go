package request

// 创建秒杀活动请求（管理员用）
type CreateFlashSaleRequest struct {
	ProductID  uint    `json:"product_id" binding:"required,min=1" msg:"请选择商品"`
	FlashPrice float64 `json:"flash_price" binding:"required,gt=0" msg:"秒杀价格必须大于0"`
	FlashStock int     `json:"flash_stock" binding:"required,min=1" msg:"秒杀库存至少为1"`
	QueueCap   int     `json:"queue_cap" binding:"omitempty,min=0" msg:"排队上限不能为负数"` // 0表示自动计算
	StartTime  string  `json:"start_time" binding:"required" msg:"请设置开始时间"`
	EndTime    string  `json:"end_time" binding:"required" msg:"请设置结束时间"`
}

// 修改秒杀活动请求（管理员用）
type UpdateFlashSaleRequest struct {
	FlashPrice float64 `json:"flash_price" binding:"omitempty,gt=0" msg:"秒杀价格必须大于0"`
	FlashStock int     `json:"flash_stock" binding:"omitempty,min=1" msg:"秒杀库存至少为1"`
	QueueCap   int     `json:"queue_cap" binding:"omitempty,min=0" msg:"排队上限不能为负数"`
	StartTime  string  `json:"start_time"`
	EndTime    string  `json:"end_time"`
	Status     *int    `json:"status" binding:"omitempty,oneof=0 1 2 3" msg:"状态值无效"` // 指针类型，避免零值问题
}

// 排队入场请求（用户端）
type FlashEnterRequest struct {
	FlashSaleID uint `json:"flash_sale_id" binding:"required,min=1" msg:"请指定秒杀活动"`
}

// 秒杀抢购请求（用户端）
type FlashSnatchRequest struct {
	FlashSaleID   uint   `json:"flash_sale_id" binding:"required,min=1" msg:"请指定秒杀活动"`
	CaptchaID     string `json:"captcha_id" binding:"required" msg:"请提供验证码ID"`
	CaptchaAnswer string `json:"captcha_answer" binding:"required" msg:"请输入验证码"`
}
