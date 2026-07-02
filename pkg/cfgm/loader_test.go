package cfgm

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoaderExplicitSourcesAndReport(t *testing.T) {
	type Config struct {
		Name  string `json:"name"`
		Debug bool   `json:"debug"`
	}

	configPath := writeTempConfig(t, `
name: "from-file"
debug: false
`)
	t.Setenv("APP_NAME", "from-env")

	cfg, report, err := New(Config{Name: "default", Debug: true}).
		Add(File(configPath), Env("APP_")).
		Load(context.Background())
	require.NoError(t, err)

	assert.Equal(t, "from-env", cfg.Name)
	assert.False(t, cfg.Debug)
	require.Len(t, report.Sources, 2)
	assert.Equal(t, "file:"+configPath, report.Sources[0].Name)
	assert.Equal(t, []string{"debug", "name"}, report.Sources[0].Keys)
	assert.Equal(t, "env:APP_", report.Sources[1].Name)
	assert.Equal(t, []string{"name"}, report.Sources[1].Keys)
}

func TestLoaderRejectsUnknownFileKeys(t *testing.T) {
	type Config struct {
		Name string `json:"name"`
	}

	configPath := writeTempConfig(t, `
name: "app"
typo: true
`)

	_, _, err := New(Config{}).
		Add(File(configPath)).
		Load(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown config keys")
	assert.Contains(t, err.Error(), "typo")
}

func TestLoaderAllowsUnknownFileKeys(t *testing.T) {
	type Config struct {
		Name string `json:"name"`
	}

	configPath := writeTempConfig(t, `
name: "app"
typo: true
`)

	cfg, _, err := New(Config{}).
		AllowUnknownKeys().
		Add(File(configPath)).
		Load(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "app", cfg.Name)
}

func TestFileSourceOptionalAndRequired(t *testing.T) {
	type Config struct {
		Name string `json:"name"`
	}

	cfg, _, err := New(Config{Name: "default"}).
		Add(File("/path/does/not/exist.yaml", Optional())).
		Load(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "default", cfg.Name)

	_, _, err = New(Config{}).
		Add(File("/path/does/not/exist.yaml")).
		Load(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "none of the config files exist")
}
