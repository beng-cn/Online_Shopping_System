package jwt

import (
	"backend/internal/config"
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type Claims struct {
	UserID   uint   `json:"user_id"`
	RoleID   uint   `json:"role_id"`
	Username string `json:"username"`
	jwt.RegisteredClaims
}

type JWTUtil struct {
	secret      []byte
	expireHours int
}

// 创建JWT工具实例（接收AppConfig参数）
func NewJWTUtil(cfg *config.AppConfig) *JWTUtil {
	return &JWTUtil{
		secret:      []byte(cfg.JWT.Secret),
		expireHours: cfg.JWT.ExpireHours,
	}
}

// 生成JWT令牌
func (j *JWTUtil) GenerateToken(userID uint, roleID uint, username string) (string, error) {
	claims := &Claims{
		UserID:   userID,
		RoleID:   roleID,
		Username: username,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Duration(j.expireHours) * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "mall-system",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(j.secret)
}

// 解析JWT令牌
func (j *JWTUtil) ParseToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		return j.secret, nil
	})
	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}

	return nil, errors.New("无效的令牌")
}
