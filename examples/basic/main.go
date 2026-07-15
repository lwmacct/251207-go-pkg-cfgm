package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/lwmacct/251207-go-pkg-cfgm/pkg/cfgm"
)

type config struct {
	Endpoint string        `json:"endpoint" desc:"服务端地址"`
	Timeout  time.Duration `json:"timeout"  desc:"请求超时"`
}

func main() {
	var defaults config
	defaults.Endpoint = "https://api.example.com"
	defaults.Timeout = 30 * time.Second
	definition := cfgm.New(defaults, cfgm.WithoutDefaultPaths())

	loaded, err := definition.Load(
		context.Background(),
		cfgm.File("examples/basic/config.yaml"),
		cfgm.Env("APP_"),
	)
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	_, _ = fmt.Fprintf(os.Stdout, "endpoint=%s timeout=%s\n", loaded.Endpoint, loaded.Timeout)
}
