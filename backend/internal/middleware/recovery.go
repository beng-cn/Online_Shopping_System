package middleware

import (
	"backend/internal/pkg/errors"
	"backend/internal/pkg/response"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
)

// 全局异常恢复中间件
func Recovery() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				log.Printf("panic recovered: %v", err)
				response.Error(ctx, errors.New(errors.CodeServerError, "服务器内部错误"))
				ctx.AbortWithStatus(http.StatusInternalServerError)
			}
		}()
		ctx.Next()
	}
}
