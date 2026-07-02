// Package config 提供应用配置管理。
//
// 配置加载由 cfgm.Load 的显式 options 决定；DefaultConfig 是最低优先级默认值。
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
	Addr        string        `json:"addr" desc:"服务器监听地址"`
	FrontendDir string        `json:"frontend-dir" desc:"前端静态文件目录"`
	Timeout     time.Duration `json:"timeout" desc:"HTTP 读写超时"`
	Idletime    time.Duration `json:"idletime" desc:"HTTP 空闲超时"`
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
// 注意：internal/app 下的命令定义引用此函数以实现单一配置来源。
func DefaultConfig() Config {
	return Config{
		Server: ServerConfig{
			Addr:        ":40117",
			FrontendDir: "${FRONTEND_DIR:-dist}",
			Timeout:     15 * time.Second,
			Idletime:    60 * time.Second,
		},
		Client: ClientConfig{
			URL:     "http://127.0.0.1:40117",
			Timeout: 30 * time.Second,
			Retries: 3,
		},
		Redis: RedisConfig{
			URL: `${REDIS_URL:-:redis://localhost:6379/0}`,
			// #nosec G101 -- shell-style template placeholder references an env var, not a hardcoded secret.
			Password: `${REDISCLI_AUTH}`,
		},
	}
}
