// Package config 提供应用配置管理。
//
// 配置加载优先级 (从低到高)：
//  1. 默认值 - DefaultConfig() 函数中定义
//  2. 配置文件 - 通过 WithAppName / WithConfigPaths 选项设置
//  3. 环境变量 - 通过 WithEnvPrefix 选项启用
//  4. CLI flags - 通过 WithCommand 选项设置
package config

import (
	"time"
)

// Config 应用配置。
type Config struct {
	Server ServerConfig `json:"server" desc:"服务端配置"`
	Client ClientConfig `json:"client" desc:"客户端配置"`
	Redis  RedisConfig  `json:"redis" desc:"Redis 配置"`
}

// ServerConfig 服务端配置。
type ServerConfig struct {
	Addr     string        `json:"addr" desc:"服务器监听地址"`
	Docs     string        `json:"docs" desc:"VitePress 文档目录路径"`
	Timeout  time.Duration `json:"timeout" desc:"HTTP 读写超时"`
	Idletime time.Duration `json:"idletime" desc:"HTTP 空闲超时"`
}

// ClientConfig 客户端配置。
type ClientConfig struct {
	URL     string        `json:"url" desc:"服务器地址"`
	Timeout time.Duration `json:"timeout" desc:"请求超时时间"`
	Retries int           `json:"retries" desc:"重试次数"`
}

// RedisConfig Redis 配置。
//
//nolint:tagliatelle
type RedisConfig struct {
	URL          string        `json:"url" desc:"Redis URL"`
	Password     string        `json:"password" desc:"Redis 密码 (REDISCLI_AUTH)"`
	Prefix       string        `json:"prefix" desc:"Redis key 前缀"`
	MaxLen       int64         `json:"max-len" desc:"日志最大长度"`
	DialTimeout  time.Duration `json:"dial-timeout" desc:"连接超时"`
	ReadTimeout  time.Duration `json:"read-timeout" desc:"读超时"`
	WriteTimeout time.Duration `json:"write-timeout" desc:"写超时"`
	Disabled     bool          `json:"disabled" desc:"禁用 Redis"`
}

// DefaultConfig 返回默认配置。
// 注意：internal/command/command.go 中的 Defaults 变量引用此函数以实现单一配置来源。
func DefaultConfig() Config {
	return Config{
		Server: ServerConfig{
			Addr:     ":40117",
			Docs:     "docs/.vitepress/dist",
			Timeout:  15 * time.Second,
			Idletime: 60 * time.Second,
		},
		Client: ClientConfig{
			URL:     `${API_BASE_URL:-:40117}`,
			Timeout: 30 * time.Second,
			Retries: 3,
		},
		Redis: RedisConfig{
			URL:      `${REDIS_URL:-:redis://localhost:6379/0}`,
			Password: `${REDISCLI_AUTH:-}`,
		},
	}
}
