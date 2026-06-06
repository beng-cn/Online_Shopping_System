package request

// 创建商品请求
type CreateProductRequest struct {
	CategoryID uint    `json:"category_id" binding:"required,min=1" msg:"请选择商品分类"`
	Name       string  `json:"name" binding:"required" msg:"商品名称不能为空"`
	Price      float64 `json:"price" binding:"required,gt=0" msg:"商品价格必须大于0"`
	Stock      int     `json:"stock" binding:"required,min=0" msg:"库存数量不能为负数"`
	Image      string  `json:"image"`
	Status     int     `json:"status" binding:"oneof=0 1" msg:"状态只能是0或1"`
}

// 更新商品请求
type UpdateProductRequest struct {
	CategoryID uint    `json:"category_id" binding:"required,min=1" msg:"请选择商品分类"`
	Name       string  `json:"name" binding:"required" msg:"商品名称不能为空"`
	Price      float64 `json:"price" binding:"required,gt=0" msg:"商品价格必须大于0"`
	Stock      int     `json:"stock" binding:"required,min=0" msg:"库存数量不能为负数"`
	Image      string  `json:"image"`
	Status     int     `json:"status" binding:"oneof=0 1" msg:"状态只能是0或1"`
}
