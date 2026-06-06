package middleware

import (
	"backend/internal/pkg/errors"
	"backend/internal/pkg/jwt"
	"backend/internal/pkg/response"
	"strings"

	"github.com/gin-gonic/gin"
)

// 普通用户登录校验中间件
func Auth(jwtUtil *jwt.JWTUtil) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			response.Error(c, errors.New(errors.CodeUnauthorized, "未登录，请先登录"))
			c.Abort()
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if !(len(parts) == 2 && parts[0] == "Bearer") {
			response.Error(c, errors.New(errors.CodeUnauthorized, "请求头格式错误"))
			c.Abort()
			return
		}

		claims, err := jwtUtil.ParseToken(parts[1])
		if err != nil {
			response.Error(c, errors.New(errors.CodeUnauthorized, "登录已过期，请重新登录"))
			c.Abort()
			return
		}

		if claims == nil {
			response.Error(c, errors.New(errors.CodeUnauthorized, "登录已过期，请重新登录"))
			c.Abort()
			return
		}

		c.Set("user_id", claims.UserID)
		c.Set("role_id", claims.RoleID)
		c.Set("username", claims.Username)

		c.Next()
	}
}

// 管理员权限校验中间件
func AdminAuth(jwtUtil *jwt.JWTUtil) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			response.Error(c, errors.New(errors.CodeUnauthorized, "未登录"))
			c.Abort()
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if !(len(parts) == 2 && parts[0] == "Bearer") {
			response.Error(c, errors.New(errors.CodeUnauthorized, "格式错误"))
			c.Abort()
			return
		}

		claims, err := jwtUtil.ParseToken(parts[1])
		if err != nil {
			response.Error(c, errors.New(errors.CodeUnauthorized, "登录过期"))
			c.Abort()
			return
		}

		if claims == nil {
			response.Error(c, errors.New(errors.CodeUnauthorized, "登录过期"))
			c.Abort()
			return
		}

		if claims.RoleID != 1 {
			response.Error(c, errors.New(errors.CodeForbidden, "仅管理员可访问"))
			c.Abort()
			return
		}

		c.Set("user_id", claims.UserID)
		c.Set("role_id", claims.RoleID)
		c.Set("username", claims.Username)

		c.Next()
	}
}
