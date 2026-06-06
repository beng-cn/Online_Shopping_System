package config

import (
	"fmt"
	"os"

	"github.com/spf13/viper"
)

var App *AppConfig

type AppConfig struct {
	Server ServerConfig `mapstructure:"server"`
	MySQL  MySQLConfig  `mapstructure:"mysql"`
	Redis  RedisConfig  `mapstructure:"redis"`
	JWT    JWTConfig    `mapstructure:"jwt"`
	Alipay AlipayConfig `mapstructure:"alipay"`
	Cache  CacheConfig  `mapstructure:"cache"`
}

type CacheConfig struct {
	HotProductWarmUpLimit int `mapstructure:"hot_product_warm_up_limit"` // 预热热门商品数量
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

// DSN 生成MySQL连接字符串
func (c *MySQLConfig) DSN() string {
	return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=%s&parseTime=%t&loc=%s",
		c.Username, c.Password, c.Host, c.Port, c.Database, c.Charset, c.ParseTime, c.Loc)
}

// Load 加载配置文件并返回AppConfig实例
func Load() (*AppConfig, error) {
	env := os.Getenv("GO_ENV")
	if env == "" {
		env = "dev"
	}

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
