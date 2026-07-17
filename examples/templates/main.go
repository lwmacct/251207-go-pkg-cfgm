package main

import (
	"context"
	"fmt"
	"os"

	"github.com/lwmacct/251207-go-pkg-cfgm/pkg/cfgm"
)

type config struct {
	Redis struct {
		URL      string `json:"url"      desc:"Redis URL"`
		Password string `json:"password" desc:"Redis 密码"`
	} `json:"redis" desc:"Redis 配置"`
}

func main() {
	ctx := context.Background()
	manager := cfgm.New(config{}, cfgm.WithoutDefaultPaths())
	loaded, err := manager.Load(ctx, cfgm.File("examples/templates/config.yaml"))
	if err != nil {
		exit(err)
	}

	_, _ = fmt.Fprintf(
		os.Stdout,
		"url=%s password_set=%t\n",
		loaded.Redis.URL,
		loaded.Redis.Password != "",
	)
}

func exit(err error) {
	_, _ = fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
