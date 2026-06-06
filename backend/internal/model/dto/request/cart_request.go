package request

// 添加到购物车请求
type AddToCartRequest struct {
	ProductID uint `json:"product_id" binding:"required,min=1" msg:"商品ID不能为空"`
	Quantity  int  `json:"quantity" binding:"required,min=1" msg:"添加数量必须大于0"`
}

// 修改购物车数量请求
type UpdateCartQuantityRequest struct {
	Quantity int `json:"quantity" binding:"required,min=1" msg:"数量必须大于0"`
}
