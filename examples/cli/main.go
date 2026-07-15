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
	definition := cfgm.New(defaultConfig(), cfgm.AppName("cfgm-example"))
	binding := definition.Bind(
		cfgm.Command("server"),
		cfgm.Alias("addr", "a"),
		cfgm.NoCLI("redis.password"),
	)

	app := &cli.Command{
		Name:  "cfgm-example",
		Usage: "cfgm CLI flag generation example",
		Flags: cfgm.RootFlags(),
		Commands: []*cli.Command{{
			Name:  "server",
			Usage: "load and print server configuration",
			Flags: binding.Flags(),
			Action: func(ctx context.Context, cmd *cli.Command) error {
				loaded, err := binding.Load(ctx, cmd)
				if err != nil {
					return err
				}
				_, _ = fmt.Fprintf(os.Stdout, "%s", cfgm.MarshalYAML(loaded))
				return nil
			},
		}},
	}

	if err := app.Run(context.Background(), os.Args); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
