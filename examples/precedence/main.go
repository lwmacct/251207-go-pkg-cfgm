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
	} `json:"server" desc:"服务端配置"`
}

func main() {
	var defaults config
	defaults.Server.Addr = ":7000"
	defaults.Server.Timeout = 30 * time.Second
	definition := cfgm.New(defaults, cfgm.AppName("precedence"), cfgm.WithoutDefaultPaths())
	binding := definition.Bind(cfgm.Scope("server"))

	app := &cli.Command{
		Name:  "precedence",
		Flags: definition.Flags(),
		Commands: []*cli.Command{{
			Name:  "server",
			Flags: binding.Flags(),
			Action: func(ctx context.Context, cmd *cli.Command) error {
				loaded, report, err := binding.LoadReport(ctx, cmd)
				if err != nil {
					return err
				}
				_, _ = fmt.Fprintf(os.Stdout, "addr=%s timeout=%s\n", loaded.Server.Addr, loaded.Server.Timeout)
				for _, source := range report.Sources {
					_, _ = fmt.Fprintf(os.Stdout, "source=%s keys=%v\n", source.Name, source.Keys)
				}
				return nil
			},
		}},
	}

	if err := app.Run(context.Background(), os.Args); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
