package main

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/lwmacct/251207-go-pkg-cfgm/pkg/cfgm"
)

type config struct {
	Name string `json:"name" desc:"应用名称"`
}

func main() {
	if err := run(context.Background()); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(ctx context.Context) error {
	strict := cfgm.New(config{}, cfgm.WithoutDefaultPaths())
	if _, err := strict.Load(ctx, cfgm.File("examples/validation/unknown.yaml")); err != nil {
		_, _ = fmt.Fprintf(os.Stdout, "strict unknown-key error: %v\n", err)
	} else {
		return errors.New("strict loading unexpectedly accepted an unknown key")
	}

	permissive := cfgm.New(config{}, cfgm.WithoutDefaultPaths(), cfgm.AllowUnknownKeys())
	loaded, err := permissive.Load(ctx, cfgm.File("examples/validation/unknown.yaml"))
	if err != nil {
		return err
	}
	_, _ = fmt.Fprintf(os.Stdout, "permissive name: %s\n", loaded.Name)

	if _, err := permissive.Load(ctx, cfgm.File("examples/validation/wrong-type.yaml")); err != nil {
		_, _ = fmt.Fprintf(os.Stdout, "known-field type error: %v\n", err)
		return nil
	}
	return errors.New("permissive loading unexpectedly accepted a wrong known-field type")
}
