package app_test

import (
	"testing"

	"github.com/lwmacct/251207-go-pkg-cfgm/internal/app/client"
	"github.com/lwmacct/251207-go-pkg-cfgm/internal/app/server"
	"github.com/lwmacct/251207-go-pkg-cfgm/internal/config"
	"github.com/lwmacct/251207-go-pkg-cfgm/pkg/cfgm"
)

func TestClientCommandCoversConfigFlags(t *testing.T) {
	cfgm.AssertCommandFlagCoverage(
		t,
		client.Command,
		config.DefaultConfig(),
		[]string{"client", "redis"},
		cfgm.IgnoreConfigKeys("redis.password"),
	)
}

func TestServerCommandCoversConfigFlags(t *testing.T) {
	cfgm.AssertCommandFlagCoverage(
		t,
		server.Command,
		config.DefaultConfig(),
		[]string{"server", "redis"},
		cfgm.IgnoreConfigKeys("redis.password"),
	)
}
