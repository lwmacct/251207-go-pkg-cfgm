// Package server 提供 HTTP 服务器命令。
package server

import (
	"github.com/urfave/cli/v3"

	appflags "github.com/lwmacct/251207-go-pkg-cfgm/internal/app/flags"
	"github.com/lwmacct/251207-go-pkg-cfgm/internal/config"
	"github.com/lwmacct/251207-go-pkg-cfgm/pkg/cfgm"
)

var (
	defaults = config.DefaultConfig()
	usage    = cfgm.Schema(defaults).Command("server")
)

// Command 服务器命令
var Command = &cli.Command{
	Name:     "server",
	Usage:    "启动 HTTP 服务器",
	Action:   action,
	Commands: []*cli.Command{},
	Flags: append([]cli.Flag{
		&cli.StringFlag{
			Name:    "addr",
			Usage:   usage.MustUsage("addr"),
			Aliases: []string{"a"},
			Value:   defaults.Server.Addr,
		},
		&cli.StringFlag{
			Name:  "frontend-dir",
			Usage: usage.MustUsage("frontend-dir"),
			Value: defaults.Server.FrontendDir,
		},
		&cli.DurationFlag{
			Name:  "timeout",
			Usage: usage.MustUsage("timeout"),
			Value: defaults.Server.Timeout,
		},
		&cli.DurationFlag{
			Name:  "idletime",
			Usage: usage.MustUsage("idletime"),
			Value: defaults.Server.Idletime,
		},
	}, appflags.Redis(defaults.Redis, usage)...),
}
