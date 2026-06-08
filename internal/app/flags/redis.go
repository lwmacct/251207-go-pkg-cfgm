// Package flags 提供应用命令可复用的 CLI flags。
package flags

import (
	"github.com/urfave/cli/v3"

	"github.com/lwmacct/251207-go-pkg-cfgm/internal/config"
	"github.com/lwmacct/251207-go-pkg-cfgm/pkg/cfgm"
)

// Redis 返回 Redis 配置对应的 CLI flags。
//
// redis.password 不提供 CLI flag，避免敏感值进入 shell history 或进程参数。
func Redis(cfg config.RedisConfig, usage cfgm.CommandSchema) []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{
			Name:  "redis.url",
			Usage: usage.MustUsage("redis.url"),
			Value: cfg.URL,
		},
		&cli.StringFlag{
			Name:  "redis.prefix",
			Usage: usage.MustUsage("redis.prefix"),
			Value: cfg.Prefix,
		},
		&cli.Int64Flag{
			Name:  "redis.max-len",
			Usage: usage.MustUsage("redis.max-len"),
			Value: cfg.MaxLen,
		},
		&cli.DurationFlag{
			Name:  "redis.dial-timeout",
			Usage: usage.MustUsage("redis.dial-timeout"),
			Value: cfg.DialTimeout,
		},
		&cli.DurationFlag{
			Name:  "redis.read-timeout",
			Usage: usage.MustUsage("redis.read-timeout"),
			Value: cfg.ReadTimeout,
		},
		&cli.DurationFlag{
			Name:  "redis.write-timeout",
			Usage: usage.MustUsage("redis.write-timeout"),
			Value: cfg.WriteTimeout,
		},
		&cli.BoolFlag{
			Name:  "redis.disabled",
			Usage: usage.MustUsage("redis.disabled"),
			Value: cfg.Disabled,
		},
	}
}
