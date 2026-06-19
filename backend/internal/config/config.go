package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/viper"
)

var App *AppConfig

type AppConfig struct {
	Server    ServerConfig    `mapstructure:"server"`
	MySQL     MySQLConfig     `mapstructure:"mysql"`
	Redis     RedisConfig     `mapstructure:"redis"`
	JWT       JWTConfig       `mapstructure:"jwt"`
	Alipay    AlipayConfig    `mapstructure:"alipay"`
	Cache     CacheConfig     `mapstructure:"cache"`
	FlashSale FlashSaleConfig `mapstructure:"flash_sale"` // 秒杀配置
}

type CacheConfig struct {
	HotProductWarmUpLimit int `mapstructure:"hot_product_warm_up_limit"` // 预热热门商品数量
}

// FlashSaleConfig 秒杀模块配置（可通过yaml或环境变量覆盖）
type FlashSaleConfig struct {
	PaymentTimeoutHours int `mapstructure:"payment_timeout_hours"` // 支付超时时间（小时），默认2
	CoolDownMinutes     int `mapstructure:"cooldown_minutes"`     // 冷却时间（分钟），默认2
	RandomDelayMaxMs    int `mapstructure:"random_delay_max_ms"` // 排队入场随机延迟上限（毫秒），0=关闭，建议300-500
}

type ServerConfig struct {
	Port    int    `mapstructure:"port"`
	RunMode string `mapstructure:"run_mode"`
}

type MySQLConfig struct {
	Host      string `mapstructure:"host"`
	Port      int    `mapstructure:"port"`
	Username  string `mapstructure:"username"`
	Password  string `mapstructure:"password"`
	Database  string `mapstructure:"database"`
	Charset   string `mapstructure:"charset"`
	ParseTime bool   `mapstructure:"parse_time"`
	Loc       string `mapstructure:"loc"`
}

type RedisConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
}

type JWTConfig struct {
	Secret      string `mapstructure:"secret"`
	ExpireHours int    `mapstructure:"expire_hours"`
}

type AlipayConfig struct {
	AppID      string `mapstructure:"app_id"`
	PrivateKey string `mapstructure:"private_key"`
	PublicKey  string `mapstructure:"public_key"`
	NotifyURL  string `mapstructure:"notify_url"`
	ReturnURL  string `mapstructure:"return_url"`
}

// DSN 生成 MySQL 连接字符串（格式: user:password@tcp(host:port)/db?charset=utf8mb4&parseTime=true）
func (c *MySQLConfig) DSN() string {
	return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=%s&parseTime=%t&loc=%s",
		c.Username, c.Password, c.Host, c.Port, c.Database, c.Charset, c.ParseTime, c.Loc)
}

// Load 加载配置文件并返回AppConfig实例
//
// 生产环境安全实践：
//   - 敏感字段(密码/密钥)通过环境变量注入，不写入 yaml 文件
//   - viper.AutomaticEnv() 自动将 MYSQL_PASSWORD 映射到 mysql.password
//   - Docker:    docker compose 的 env_file 或 environment 注入
//   - K8s:       Secret 资源 → Pod 环境变量 → Viper 自动读取
//   - 本地开发:   dev.yaml 不在 Git 中（已在 .gitignore），从 example 复制后填本地密码
func Load() (*AppConfig, error) {
	env := os.Getenv("GO_ENV")
	if env == "" {
		env = "dev"
	}

	// 自动将环境变量映射到配置字段（MYSQL_PASSWORD → mysql.password）
	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	viper.SetConfigName(env)
	viper.SetConfigType("yaml")
	viper.AddConfigPath("./configs")

	if err := viper.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("读取配置文件失败: %w", err)
	}

	var appConfig AppConfig
	if err := viper.Unmarshal(&appConfig); err != nil {
		return nil, fmt.Errorf("解析配置文件失败: %w", err)
	}

	App = &appConfig

	return &appConfig, nil
}
