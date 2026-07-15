package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/urfave/cli/v3"

	"github.com/lwmacct/251207-go-pkg-cfgm/pkg/cfgm"
)

type config struct {
	Server struct {
		Addr    string        `json:"addr"    desc:"监听地址"`
		Timeout time.Duration `json:"timeout" desc:"请求超时"`
		Redis   struct {
			URL      string `json:"url"      desc:"Redis URL"`
			Password string `json:"password" desc:"Redis 密码"`
		} `json:"redis" desc:"Redis 配置"`
	} `json:"server" desc:"服务端配置"`
}

func defaultConfig() config {
	var defaults config
	defaults.Server.Addr = ":8080"
	defaults.Server.Timeout = 30 * time.Second
	defaults.Server.Redis.URL = "redis://localhost:6379/0"
	return defaults
}

func main() {
	manager := cfgm.New(
		defaultConfig(),
		cfgm.AppName("cfgm-example"),
		cfgm.CLIAlias("server.addr", "a"),
		cfgm.HideCLI("server.redis.password"),
	)

	server := &cli.Command{
		Name:  "server",
		Usage: "load and print server configuration",
		Action: manager.Action(func(_ context.Context, _ *cli.Command, loaded *config) error {
			_, _ = fmt.Fprintf(os.Stdout, "%s", cfgm.MarshalYAML(loaded))
			return nil
		}),
	}
	app := &cli.Command{
		Name:     "cfgm-example",
		Usage:    "cfgm CLI flag generation example",
		Commands: []*cli.Command{server},
	}
	manager.MustConfigure(app)

	if err := app.Run(context.Background(), os.Args); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
