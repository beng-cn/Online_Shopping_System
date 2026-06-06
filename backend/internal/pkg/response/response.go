package response

import (
	"backend/internal/pkg/errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// 统一响应体
type Response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// 成功响应
func Success(ctx *gin.Context, data interface{}) {
	ctx.JSON(http.StatusOK, Response{
		Code:    errors.CodeSuccess,
		Message: "success",
		Data:    data,
	})
}

// 错误处理
func Error(ctx *gin.Context, err error) {
	code := errors.CodeServerError
	msg := "服务器内部错误"

	// 优先处理自定义业务错误
	if e, ok := err.(*errors.Error); ok {
		code = e.Code
		msg = e.Message
	} else if err == gorm.ErrRecordNotFound {
		// 统一处理GORM记录不存在错误
		code = errors.CodeNotFound
		msg = "资源不存在"
	}

	ctx.JSON(http.StatusOK, Response{
		Code:    code,
		Message: msg,
		Data:    nil,
	})
}
