package main

import (
	"context"
	"fmt"
	"os"

	"github.com/lwmacct/251207-go-pkg-version/pkg/version"
	"github.com/urfave/cli/v3"

	"github.com/lwmacct/251207-go-pkg-cfgm/internal/command/client"
	"github.com/lwmacct/251207-go-pkg-cfgm/internal/command/server"
)

func main() {
	app := &cli.Command{
		Name:    version.AppRawName,
		Usage:   "配置管理工具",
		Version: version.GetVersion(),
		Commands: []*cli.Command{
			version.Command,
			client.Command,
			server.Command,
		},
	}

	if err := app.Run(context.Background(), os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
