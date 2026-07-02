package flags

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v3"

	"github.com/lwmacct/251207-go-pkg-cfgm/internal/config"
	"github.com/lwmacct/251207-go-pkg-cfgm/pkg/cfgm"
)

func TestRedisFlagsMapToConfigOnClientAndServerCommands(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{name: "client", args: []string{"client", "--redis.url", "redis://client:6379/0"}},
		{name: "server", args: []string{"server", "--redis.url", "redis://server:6379/0"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defaults := config.DefaultConfig()
			usage := cfgm.Schema(defaults).Command(tt.name)
			var loaded *config.Config
			cmd := &cli.Command{
				Name:  tt.name,
				Flags: Redis(defaults.Redis, usage),
				Action: func(ctx context.Context, cmd *cli.Command) error {
					cfg, err := cfgm.Load(ctx, defaults, cfgm.Command(cmd))
					if err != nil {
						return err
					}
					loaded = cfg

					return nil
				},
			}

			err := cmd.Run(context.Background(), tt.args)
			require.NoError(t, err)
			require.NotNil(t, loaded)
			assert.Equal(t, tt.args[2], loaded.Redis.URL)
		})
	}
}
