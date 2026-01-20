package main

import (
	"context"
	"log/slog"
	"os"

	app "github.com/lwmacct/251207-go-pkg-cfgm/internal/command/server"
)

func main() {
	if err := app.Command.Run(context.Background(), os.Args); err != nil {
		slog.Error("应用程序运行失败", "error", err)
		os.Exit(1)
	}
}
