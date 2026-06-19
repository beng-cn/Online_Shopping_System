package middleware

import (
	"backend/internal/pkg/errors"
	"backend/internal/pkg/jwt"
	"backend/internal/pkg/response"
	"strings"

	"github.com/gin-gonic/gin"
)

// Auth 普通用户登录校验中间件，解析 Bearer Token 并验证 JWT 有效性及黑名单状态
// blacklist 可选，nil 表示不启用黑名单检查
func Auth(jwtUtil *jwt.JWTUtil, blacklist jwt.TokenBlacklist) gin.HandlerFunc {
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

		// JWT 黑名单检查：Token 已被主动失效（修改密码、管理员禁用等场景）
		// JTI 为空时直接拒绝——所有新签发的 Token 都包含 JTI，空 JTI 说明 Token 异常
		if blacklist != nil {
			if claims.JTI == "" {
				response.Error(c, errors.New(errors.CodeUnauthorized, "Token 异常，请重新登录"))
				c.Abort()
				return
			}
			if blacklist.IsBlacklisted(claims.JTI) {
				response.Error(c, errors.New(errors.CodeUnauthorized, "Token 已失效，请重新登录"))
				c.Abort()
				return
			}
		}

		c.Set("user_id", claims.UserID)
		c.Set("role_id", claims.RoleID)
		c.Set("username", claims.Username)

		c.Next()
	}
}

// AdminAuth 管理员权限校验中间件，在 Auth 基础上额外校验 role_id=1
// blacklist 可选，nil 表示不启用黑名单检查
func AdminAuth(jwtUtil *jwt.JWTUtil, blacklist jwt.TokenBlacklist) gin.HandlerFunc {
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

		// JWT 黑名单检查
		if blacklist != nil {
			if claims.JTI == "" {
				response.Error(c, errors.New(errors.CodeUnauthorized, "Token 异常，请重新登录"))
				c.Abort()
				return
			}
			if blacklist.IsBlacklisted(claims.JTI) {
				response.Error(c, errors.New(errors.CodeUnauthorized, "Token 已失效，请重新登录"))
				c.Abort()
				return
			}
		}

		c.Set("user_id", claims.UserID)
		c.Set("role_id", claims.RoleID)
		c.Set("username", claims.Username)

		c.Next()
	}
}
