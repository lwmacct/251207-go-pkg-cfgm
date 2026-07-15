package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/lwmacct/251207-go-pkg-cfgm/pkg/cfgm"
)

type config struct {
	Name    string        `json:"name"    desc:"应用名称"`
	Timeout time.Duration `json:"timeout" desc:"请求超时"`
}

func defaultConfig() config {
	return config{Name: "example", Timeout: 30 * time.Second}
}

var manager = cfgm.New(defaultConfig(), cfgm.WithoutDefaultPaths())

func main() {
	loaded, err := manager.Load(context.Background(), cfgm.File("examples/config-files/config.yaml"))
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	_, _ = fmt.Fprintf(os.Stdout, "name=%s timeout=%s\n", loaded.Name, loaded.Timeout)
}
