package main

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"

	"github.com/urfave/cli/v3"

	"github.com/lwmacct/251207-go-pkg-cfgm/pkg/cfgm"
)

type endpoint struct {
	URL *url.URL
}

func parseEndpoint(value string) (endpoint, error) {
	parsed, err := url.Parse(value)
	if err != nil {
		return endpoint{}, fmt.Errorf("parse endpoint: %w", err)
	}
	if parsed.Scheme != "svc" || parsed.Host == "" {
		return endpoint{}, errors.New("endpoint must use svc:// with a host")
	}
	return endpoint{URL: parsed}, nil
}

func (e endpoint) String() string {
	if e.URL == nil {
		return ""
	}
	return e.URL.String()
}

type config struct {
	Endpoint endpoint `json:"endpoint" desc:"服务端点"`
}

func main() {
	defaultEndpoint, err := parseEndpoint("svc://default")
	if err != nil {
		panic(err)
	}
	definition := cfgm.New(
		config{Endpoint: defaultEndpoint},
		cfgm.AppName("codec"),
		cfgm.WithoutDefaultPaths(),
		cfgm.WithCodec(cfgm.Codec[endpoint]{Parse: parseEndpoint, Format: endpoint.String}),
	)
	binding := definition.Bind()
	app := &cli.Command{
		Name:  "codec",
		Flags: append(cfgm.RootFlags(), binding.Flags()...),
		Action: func(ctx context.Context, cmd *cli.Command) error {
			loaded, err := binding.Load(ctx, cmd)
			if err != nil {
				return err
			}
			_, _ = fmt.Fprintln(os.Stdout, loaded.Endpoint.String())
			return nil
		},
	}

	if err := app.Run(context.Background(), os.Args); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
