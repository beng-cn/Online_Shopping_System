// Package env 环境变量门禁校验 — 启动时强制检查所有敏感凭据已通过环境变量注入
//
// 设计原则：
//   - 所有密码/密钥/Token 禁止硬编码在 yaml 配置文件中
//   - 启动时逐项检查，缺失任何一项立即 panic 阻止启动（fail-fast）
//   - 生产环境(GO_ENV=prod/docker)执行严格模式，开发环境(dev)执行警告模式
//   - 错误消息清晰列出所有缺失项，方便运维人员一次补齐
package env

import (
	"fmt"
	"log"
	"os"
	"strings"
)

// RequiredSecret 定义一项必须通过环境变量注入的敏感配置
type RequiredSecret struct {
	EnvKey      string // 环境变量名，如 "MYSQL_PASSWORD"
	Description string // 中文描述，如 "MySQL 数据库密码"
	AllowEmpty  bool   // 是否允许空值（如本地开发 Redis 无密码）
}

// ValidateSecrets 启动门禁：逐一检查所有必需环境变量是否已设置
//
// 严格模式（prod/docker）：任何缺失 → panic 阻止启动
// 宽松模式（dev/test）：缺失 → 打印警告继续运行
//
// 返回值：所有检查项的状态信息（用于启动日志输出）
func ValidateSecrets() []string {
	env := os.Getenv("GO_ENV")
	if env == "" {
		env = "dev"
	}
	isStrict := env == "prod" || env == "docker" || env == "production"

	// 定义所有需要检查的敏感配置项（集中管理，方便审计）
	required := []RequiredSecret{
		{EnvKey: "MYSQL_PASSWORD", Description: "MySQL 数据库密码", AllowEmpty: false},
		{EnvKey: "MYSQL_HOST", Description: "MySQL 主机地址", AllowEmpty: false},
		{EnvKey: "REDIS_HOST", Description: "Redis 主机地址", AllowEmpty: false},
		{EnvKey: "REDIS_PASSWORD", Description: "Redis 密码（本地开发可为空）", AllowEmpty: true},
		{EnvKey: "JWT_SECRET", Description: "JWT 签名密钥", AllowEmpty: false},
	}

	var missing []string
	var status []string

	for _, secret := range required {
		val := os.Getenv(secret.EnvKey)
		if val == "" && !secret.AllowEmpty {
			missing = append(missing, fmt.Sprintf("  ❌ %s — 环境变量 %s 未设置或为空", secret.Description, secret.EnvKey))
		} else if val == "" && secret.AllowEmpty {
			status = append(status, fmt.Sprintf("  ⚠️  %s — 环境变量 %s 为空（已允许）", secret.Description, secret.EnvKey))
		} else {
			status = append(status, fmt.Sprintf("  ✅ %s — 已从环境变量 %s 注入（长度=%d）", secret.Description, secret.EnvKey, len(val)))
		}
	}

	// 额外检查：生产环境 JWT Secret 不能使用默认值
	if jwtVal := os.Getenv("JWT_SECRET"); jwtVal != "" {
		if isWeakJWTSecret(jwtVal) {
			msg := fmt.Sprintf("  🔴 JWT 签名密钥使用了弱密码，生产环境必须更换！请设置强随机字符串到 JWT_SECRET 环境变量")
			if isStrict {
				missing = append(missing, msg)
			} else {
				status = append(status, msg)
			}
		}
	}

	if len(missing) > 0 {
		errMsg := fmt.Sprintf(
			"\n========================================\n"+
				"🔴 环境变量门禁校验失败！以下必需配置未设置：\n\n%s\n\n"+
				"请通过以下方式之一注入：\n"+
				"  1. 命令行: export MYSQL_PASSWORD=xxx && go run ./cmd/server/\n"+
				"  2. .env 文件: 在项目根目录创建 .env 文件，格式 KEY=VALUE\n"+
				"  3. Docker: docker compose 的 environment 或 env_file 字段\n"+
				"  4. K8s: kubectl create secret generic ... --from-literal=KEY=VALUE\n"+
				"========================================\n",
			strings.Join(missing, "\n"),
		)

		if isStrict {
			// 生产环境严格模式：直接 panic 阻止启动
			panic(errMsg)
		} else {
			// 开发环境宽松模式：打印警告（方便本地调试）
			log.Printf("⚠️  [环境变量门禁] 发现缺失配置（开发环境不阻止启动）:\n%s", errMsg)
		}
	}

	// 打印门禁通过信息
	log.Println("🏁 [环境变量门禁] 敏感配置检查完成:")
	for _, s := range status {
		log.Println(s)
	}

	return status
}

// isWeakJWTSecret 检测是否使用了示例/默认的弱 JWT 密钥
func isWeakJWTSecret(secret string) bool {
	weakPatterns := []string{
		"your-jwt-secret",
		"change-me",
		"changeme",
		"your-secret",
		"your_secret",
		"default",
		"test",
		"123456",
		"password",
	}
	lower := strings.ToLower(secret)
	for _, pattern := range weakPatterns {
		if strings.Contains(lower, pattern) {
			return true
		}
	}
	// JWT secret 长度至少 32 字符才算强密钥
	return len(secret) < 16
}
