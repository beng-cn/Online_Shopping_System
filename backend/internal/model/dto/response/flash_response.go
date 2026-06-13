package response

import "time"

// 秒杀活动列表响应（公开页面展示）
type FlashSaleListResponse struct {
	ID          uint    `json:"id"`
	ProductID   uint    `json:"product_id"`
	ProductName string  `json:"product_name"`
	FlashPrice  float64 `json:"flash_price"`
	OriginPrice float64 `json:"origin_price"` // 原价，用于展示折扣
	FlashStock  int     `json:"flash_stock"`
	Remaining   int     `json:"remaining"`    // Redis中的实时剩余库存
	QueueCount  int     `json:"queue_count"`  // 当前排队人数
	StartTime   string  `json:"start_time"`
	EndTime     string  `json:"end_time"`
	Status      int     `json:"status"`
	Image       string  `json:"image"`
}

// 秒杀详情响应（含倒计时信息）
type FlashSaleDetailResponse struct {
	ID          uint   `json:"id"`
	ProductID   uint   `json:"product_id"`
	ProductName string `json:"product_name"`
	FlashPrice  float64 `json:"flash_price"`
	OriginPrice float64 `json:"origin_price"`
	FlashStock  int    `json:"flash_stock"`
	Remaining   int    `json:"remaining"`
	QueueCount  int    `json:"queue_count"`
	QueueCap    int    `json:"queue_cap"`
	StartTime   string `json:"start_time"`
	EndTime     string `json:"end_time"`
	Status      int    `json:"status"`
	Image       string `json:"image"`
	ServerTime  string `json:"server_time"` // 服务器当前时间，用于前端计算倒计时
}

// 排队入场响应
type FlashEnterResponse struct {
	Admitted    bool   `json:"admitted"`     // 是否获准入场
	QueueNumber int64  `json:"queue_number"` // 排队序号
	Message     string `json:"message"`      // 提示信息
}

// 秒杀抢购响应
type FlashSnatchResponse struct {
	Success  bool   `json:"success"`
	OrderNo  string `json:"order_no,omitempty"`  // 成功时返回订单号
	Message  string `json:"message"`              // 提示信息
}

// 秒杀订单列表响应
type FlashOrderResponse struct {
	ID         uint      `json:"id"`
	OrderNo    string    `json:"order_no"`
	FlashPrice float64   `json:"flash_price"`
	ProductName string   `json:"product_name"`
	Status     int       `json:"status"`      // 0=待支付 1=已支付 2=已取消 3=待释放
	CreatedAt  time.Time `json:"created_at"`
}

// 管理员秒杀活动列表响应
type AdminFlashSaleResponse struct {
	ID         uint      `json:"id"`
	ProductID  uint      `json:"product_id"`
	FlashPrice float64   `json:"flash_price"`
	FlashStock int       `json:"flash_stock"`
	QueueCap   int       `json:"queue_cap"`
	StartTime  time.Time `json:"start_time"`
	EndTime    time.Time `json:"end_time"`
	Status     int       `json:"status"`
	CreatedAt  time.Time `json:"created_at"`
}
