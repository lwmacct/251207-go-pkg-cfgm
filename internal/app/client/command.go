// Package client 提供 HTTP 客户端命令。
package client

import (
	"github.com/urfave/cli/v3"

	appflags "github.com/lwmacct/251207-go-pkg-cfgm/internal/app/flags"
	"github.com/lwmacct/251207-go-pkg-cfgm/internal/config"
	"github.com/lwmacct/251207-go-pkg-cfgm/pkg/cfgm"
)

var (
	defaults = config.DefaultConfig()
	usage    = cfgm.Schema(defaults).Command("client")
)

// Command 客户端命令
var Command = &cli.Command{
	Name:  "client",
	Usage: "HTTP 客户端工具",
	Flags: append([]cli.Flag{
		&cli.StringFlag{
			Name:    "url",
			Usage:   usage.MustUsage("url"),
			Aliases: []string{"s"},
			Value:   defaults.Client.URL,
		},
		&cli.DurationFlag{
			Name:  "timeout",
			Usage: usage.MustUsage("timeout"),
			Value: defaults.Client.Timeout,
		},
		&cli.IntFlag{
			Name:  "retries",
			Usage: usage.MustUsage("retries"),
			Value: defaults.Client.Retries,
		},
	}, appflags.Redis(defaults.Redis, usage)...),
	Action: action,
	Commands: []*cli.Command{
		{
			Name:   "health",
			Usage:  "检查服务器健康状态",
			Action: healthAction,
		},
		{
			Name:      "get",
			Usage:     "发送 GET 请求",
			ArgsUsage: "[path]",
			Action:    getAction,
		},
	},
}
