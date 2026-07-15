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
	Redis    struct {
		URL      string `json:"url"      desc:"Redis URL"`
		Password string `json:"password" desc:"Redis 密码"`
	} `json:"redis" desc:"Redis 配置"`
}

func main() {
	var defaults config
	defaults.Endpoint = "https://api.example.com"
	defaults.Timeout = 30 * time.Second
	defaults.Redis.URL = "${REDIS_URL:-redis://localhost:6379/0}"
	defaults.Redis.Password = "${REDISCLI_AUTH}"
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

	_, _ = fmt.Fprintf(
		os.Stdout,
		"endpoint=%s timeout=%s redis_url=%s redis_password_set=%t\n",
		loaded.Endpoint,
		loaded.Timeout,
		loaded.Redis.URL,
		loaded.Redis.Password != "",
	)
}
