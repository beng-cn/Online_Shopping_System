package response

import (
	"backend/internal/pkg/errors"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

// 统一响应结构体
type Response struct {
	Code int         `json:"code"`
	Msg  string      `json:"msg"`
	Data interface{} `json:"data,omitempty"`
}

// 成功响应（绝对不会 panic）
func Success(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, Response{
		Code: 0,
		Msg:  "success",
		Data: data,
	})
}

func Error(ctx *gin.Context, err error) {
	var code int = 500
	var msg string = "服务器内部错误"

	if e, ok := err.(*errors.Error); ok {
		code = e.Code
		msg = e.Message
	} else {
		// 打印未知错误的详细信息，方便调试
		fmt.Printf("【未知错误类型】%T, 错误内容: %v\n", err, err)
	}

	ctx.JSON(http.StatusOK, Response{
		Code: code,
		Msg:  msg,
		Data: nil,
	})
}
