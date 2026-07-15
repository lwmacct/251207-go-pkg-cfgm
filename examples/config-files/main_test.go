package main

import (
	"testing"

	"github.com/lwmacct/251207-go-pkg-cfgm/pkg/cfgm"
)

var files = cfgm.ConfigFiles[config]{
	Definition:  definition,
	ExampleFile: "examples/config-files/config.example.yaml",
	RuntimeFile: "examples/config-files/config.yaml",
}

func TestWriteConfigExample(t *testing.T) {
	files.WriteExample(t)
}

func TestRuntimeConfigValid(t *testing.T) {
	files.ValidateRuntimeConfig(t)
}
