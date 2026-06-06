// backend/internal/model/dto/response/page_response.go
package response

// PageResponse 统一分页响应结构体
type PageResponse struct {
	List       interface{} `json:"list"`        // 数据列表
	Total      int64       `json:"total"`       // 总条数
	PageNum    int         `json:"page_num"`    // 当前页码
	PageSize   int         `json:"page_size"`   // 每页条数
	TotalPages int         `json:"total_pages"` // 总页数
}

// NewPageResponse 生成分页响应
func NewPageResponse(list interface{}, total int64, pageNum int, pageSize int) *PageResponse {
	totalPages := int(total) / pageSize
	if int(total)%pageSize != 0 {
		totalPages++
	}
	return &PageResponse{
		List:       list,
		Total:      total,
		PageNum:    pageNum,
		PageSize:   pageSize,
		TotalPages: totalPages,
	}
}
