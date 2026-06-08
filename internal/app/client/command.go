// Package client 提供 HTTP 客户端命令。
package client

import (
	"github.com/urfave/cli/v3"

	appflags "github.com/lwmacct/251207-go-pkg-cfgm/internal/app/flags"
	"github.com/lwmacct/251207-go-pkg-cfgm/internal/config"
)

var defaults = config.DefaultConfig()

// Command 客户端命令
var Command = &cli.Command{
	Name:  "client",
	Usage: "HTTP 客户端工具",
	Flags: append([]cli.Flag{
		&cli.StringFlag{
			Name:    "url",
			Aliases: []string{"s"},
			Value:   defaults.Client.URL,
			Usage:   "服务器地址",
		},
		&cli.DurationFlag{
			Name:  "timeout",
			Value: defaults.Client.Timeout,
			Usage: "请求超时时间",
		},
		&cli.IntFlag{
			Name:  "retries",
			Value: defaults.Client.Retries,
			Usage: "重试次数",
		},
	}, appflags.Redis(defaults.Redis)...),
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
