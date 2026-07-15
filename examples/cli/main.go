package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/urfave/cli/v3"

	"github.com/lwmacct/251207-go-pkg-cfgm/pkg/cfgm"
)

type certificate struct {
	ID          string `json:"id"          desc:"证书标识"`
	Certificate string `json:"certificate" desc:"证书 URI"`
	PrivateKey  string `json:"private-key" desc:"私钥 URI"`
}

type config struct {
	Server struct {
		Addr         string        `json:"addr"         desc:"监听地址"`
		Timeout      time.Duration `json:"timeout"      desc:"请求超时"`
		Tags         []string      `json:"tags"         desc:"服务标签"`
		Certificates []certificate `json:"certificates" desc:"TLS 证书"`
	} `json:"server" desc:"服务端配置"`
	Logging struct {
		Level string `json:"level" desc:"日志级别"`
		Token string `json:"token" desc:"日志服务令牌"`
	} `json:"logging" desc:"日志配置"`
}

func defaultConfig() config {
	var defaults config
	defaults.Server.Addr = ":8080"
	defaults.Server.Timeout = 30 * time.Second
	defaults.Server.Tags = []string{"default"}
	defaults.Logging.Level = "info"
	return defaults
}

func main() {
	definition := cfgm.New(defaultConfig(), cfgm.AppName("cfgm-example"))
	binding := definition.Bind(
		cfgm.Scope("server"),
		cfgm.Include("logging"),
		cfgm.Alias("server.addr", "a"),
		cfgm.NoCLI("logging.token"),
	)

	app := &cli.Command{
		Name:  "cfgm-example",
		Usage: "cfgm CLI flag generation example",
		Flags: definition.Flags(),
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
