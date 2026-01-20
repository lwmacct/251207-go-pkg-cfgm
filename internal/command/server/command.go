// Package server 提供 HTTP 服务器命令。
package server

import (
	"github.com/lwmacct/251207-go-pkg-version/pkg/version"
	"github.com/urfave/cli/v3"

	"github.com/lwmacct/251207-go-pkg-cfgm/internal/command"
)

// Command 服务器命令
var Command = &cli.Command{
	Name:     "server",
	Usage:    "启动 HTTP 服务器",
	Action:   action,
	Commands: []*cli.Command{version.Command},
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:    "server-addr",
			Aliases: []string{"a"},
			Value:   command.Defaults.Server.Addr,
			Usage:   "服务器监听地址",
		},
		&cli.StringFlag{
			Name:  "server-docs",
			Value: command.Defaults.Server.Docs,
			Usage: "VitePress 文档目录路径",
		},
		&cli.DurationFlag{
			Name:  "server-timeout",
			Value: command.Defaults.Server.Timeout,
			Usage: "HTTP 读写超时",
		},
		&cli.DurationFlag{
			Name:  "server-idletime",
			Value: command.Defaults.Server.Idletime,
			Usage: "HTTP 空闲超时",
		},
	},
}
