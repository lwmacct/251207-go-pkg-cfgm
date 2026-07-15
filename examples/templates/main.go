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
	definition := cfgm.New(config{}, cfgm.WithoutDefaultPaths())
	expanded, err := definition.Load(ctx, cfgm.File("examples/templates/config.yaml"))
	if err != nil {
		exit(err)
	}
	raw, err := definition.Load(ctx, cfgm.File("examples/templates/config.yaml", cfgm.Raw()))
	if err != nil {
		exit(err)
	}
	globallyRaw := cfgm.New(config{}, cfgm.WithoutDefaultPaths(), cfgm.WithoutTemplateExpansion())
	forced, err := globallyRaw.Load(ctx, cfgm.File("examples/templates/config.yaml", cfgm.ExpandTemplates()))
	if err != nil {
		exit(err)
	}

	_, _ = fmt.Fprintf(
		os.Stdout,
		"expanded: url=%s password_set=%t\nraw: url=%s password=%s\nforced: url=%s password_set=%t\n",
		expanded.Redis.URL,
		expanded.Redis.Password != "",
		raw.Redis.URL,
		raw.Redis.Password,
		forced.Redis.URL,
		forced.Redis.Password != "",
	)
}

func exit(err error) {
	_, _ = fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
