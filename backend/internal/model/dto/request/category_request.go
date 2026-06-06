package request

// 创建分类请求
type CreateCategoryRequest struct {
	Name     string `json:"name" binding:"required" msg:"分类名称不能为空"`
	ParentID uint   `json:"parent_id" binding:"min=0" msg:"父分类ID不能为负数"`
	Status   int    `json:"status" binding:"oneof=0 1" msg:"状态只能是0或1"`
}

// 更新分类请求
type UpdateCategoryRequest struct {
	Name     string `json:"name" binding:"required" msg:"分类名称不能为空"`
	ParentID uint   `json:"parent_id" binding:"min=0" msg:"父分类ID不能为负数"`
	Status   int    `json:"status" binding:"oneof=0 1" msg:"状态只能是0或1"`
}
