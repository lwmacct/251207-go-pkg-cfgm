// Package flags 提供应用命令可复用的 CLI flags。
package flags

import (
	"github.com/urfave/cli/v3"

	"github.com/lwmacct/251207-go-pkg-cfgm/internal/config"
)

// Redis 返回 Redis 配置对应的 CLI flags。
//
// redis.password 不提供 CLI flag，避免敏感值进入 shell history 或进程参数。
func Redis(cfg config.RedisConfig) []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{
			Name:  "redis.url",
			Value: cfg.URL,
			Usage: "Redis URL",
		},
		&cli.StringFlag{
			Name:  "redis.prefix",
			Value: cfg.Prefix,
			Usage: "Redis key 前缀",
		},
		&cli.Int64Flag{
			Name:  "redis.max-len",
			Value: cfg.MaxLen,
			Usage: "日志最大长度",
		},
		&cli.DurationFlag{
			Name:  "redis.dial-timeout",
			Value: cfg.DialTimeout,
			Usage: "Redis 连接超时",
		},
		&cli.DurationFlag{
			Name:  "redis.read-timeout",
			Value: cfg.ReadTimeout,
			Usage: "Redis 读超时",
		},
		&cli.DurationFlag{
			Name:  "redis.write-timeout",
			Value: cfg.WriteTimeout,
			Usage: "Redis 写超时",
		},
		&cli.BoolFlag{
			Name:  "redis.disabled",
			Value: cfg.Disabled,
			Usage: "禁用 Redis",
		},
	}
}
