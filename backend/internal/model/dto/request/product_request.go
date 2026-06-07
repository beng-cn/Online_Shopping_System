package request

// 创建商品请求
type CreateProductRequest struct {
	CategoryID uint   `json:"category_id" binding:"required,min=1" msg:"请选择商品分类"`
	Name       string `json:"name" binding:"required" msg:"商品名称不能为空"`
	Keywords   string `json:"keywords" binding:"omitempty,max=500" msg:"关键词长度不能超过500个字符"`
	Price      float64 `json:"price" binding:"required,gt=0" msg:"商品价格必须大于0"`
	Stock      int    `json:"stock" binding:"required,min=0" msg:"库存数量不能为负数"`
	Image      string `json:"image"`
	Status     int    `json:"status" binding:"oneof=0 1" msg:"状态只能是0或1"`
}

// 更新商品请求
type UpdateProductRequest struct {
	CategoryID uint   `json:"category_id" binding:"required,min=1" msg:"请选择商品分类"`
	Name       string `json:"name" binding:"required" msg:"商品名称不能为空"`
	Keywords   string `json:"keywords" binding:"omitempty,max=500" msg:"关键词长度不能超过500个字符"`
	Price      float64 `json:"price" binding:"required,gt=0" msg:"商品价格必须大于0"`
	Stock      int    `json:"stock" binding:"required,min=0" msg:"库存数量不能为负数"`
	Image      string `json:"image"`
	Status     int    `json:"status" binding:"oneof=0 1" msg:"状态只能是0或1"`
}

// 商品列表查询请求
type ProductListRequest struct {
	Keyword    string  `json:"keyword" binding:"omitempty,max=50" msg:"关键词长度不能超过50个字符"`
	CategoryID string  `json:"category_id" binding:"omitempty,numeric" msg:"分类ID必须是数字"`
	PageNum    int     `json:"page_num" binding:"omitempty,min=1" msg:"页码必须大于0"`
	PageSize   int     `json:"page_size" binding:"omitempty,min=1,max=100" msg:"每页条数必须在1-100之间"`
	Sort       string  `json:"sort" binding:"omitempty,oneof=created_at sales price_asc price_desc" msg:"排序方式只能是created_at、sales、price_asc、price_desc"`
	MinPrice   float64 `json:"min_price" binding:"omitempty,min=0" msg:"最低价格不能为负数"`
	MaxPrice   float64 `json:"max_price" binding:"omitempty,min=0" msg:"最高价格不能为负数"`
}
