package main

import (
	"context"
	"fmt"
	"os"

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
		Tags         []string      `json:"tags"         desc:"服务标签"`
		Certificates []certificate `json:"certificates" desc:"TLS 证书"`
	} `json:"server" desc:"服务端配置"`
}

func main() {
	manager := cfgm.New(config{}, cfgm.AppName("composite"), cfgm.WithoutDefaultPaths())
	app := &cli.Command{
		Name: "composite",
		Commands: []*cli.Command{{
			Name: "server",
			Action: manager.Action(func(_ context.Context, _ *cli.Command, loaded *config) error {
				_, _ = fmt.Fprintf(os.Stdout, "%s", cfgm.MarshalYAML(loaded))
				return nil
			}),
		}},
	}
	manager.MustConfigure(app)

	if err := app.Run(context.Background(), os.Args); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
