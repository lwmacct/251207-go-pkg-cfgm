// Package client 提供 HTTP 客户端命令。
package client

import (
	"github.com/lwmacct/251207-go-pkg-version/pkg/version"
	"github.com/urfave/cli/v3"

	"github.com/lwmacct/251207-go-pkg-cfgm/internal/command"
)

// Command 客户端命令
var Command = &cli.Command{
	Name:  "client",
	Usage: "HTTP 客户端工具",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:    "client-url",
			Aliases: []string{"s"},
			Value:   command.Defaults.Client.URL,
			Usage:   "服务器地址",
		},
		&cli.DurationFlag{
			Name:  "client-timeout",
			Value: command.Defaults.Client.Timeout,
			Usage: "请求超时时间",
		},
		&cli.IntFlag{
			Name:  "client-retries",
			Value: command.Defaults.Client.Retries,
			Usage: "重试次数",
		},
	},
	Action: action,
	Commands: []*cli.Command{
		version.Command,
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
