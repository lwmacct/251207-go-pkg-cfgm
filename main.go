package main

import (
	"context"
	"fmt"
	"os"

	"github.com/urfave/cli/v3"

	"github.com/lwmacct/251207-go-pkg-cfgm/internal/app/client"
	"github.com/lwmacct/251207-go-pkg-cfgm/internal/app/server"
	"github.com/lwmacct/251207-go-pkg-cfgm/pkg/cfgm"
)

func main() {
	app := &cli.Command{
		Name:  "cfgm",
		Usage: "配置管理工具",
		Flags: []cli.Flag{
			cfgm.ConfigFlag(),
		},
		Commands: []*cli.Command{
			client.Command,
			server.Command,
		},
	}

	if err := app.Run(context.Background(), os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
