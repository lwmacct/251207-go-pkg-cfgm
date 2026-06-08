// Package server 提供 HTTP 服务器命令。
package server

import (
	"github.com/urfave/cli/v3"

	appflags "github.com/lwmacct/251207-go-pkg-cfgm/internal/app/flags"
	"github.com/lwmacct/251207-go-pkg-cfgm/internal/config"
)

var defaults = config.DefaultConfig()

// Command 服务器命令
var Command = &cli.Command{
	Name:     "server",
	Usage:    "启动 HTTP 服务器",
	Action:   action,
	Commands: []*cli.Command{},
	Flags: append([]cli.Flag{
		&cli.StringFlag{
			Name:    "addr",
			Aliases: []string{"a"},
			Value:   defaults.Server.Addr,
			Usage:   "服务器监听地址",
		},
		&cli.StringFlag{
			Name:  "frontend-dir",
			Value: defaults.Server.FrontendDir,
			Usage: "前端静态文件目录",
		},
		&cli.DurationFlag{
			Name:  "timeout",
			Value: defaults.Server.Timeout,
			Usage: "HTTP 读写超时",
		},
		&cli.DurationFlag{
			Name:  "idletime",
			Value: defaults.Server.Idletime,
			Usage: "HTTP 空闲超时",
		},
	}, appflags.Redis(defaults.Redis)...),
}
