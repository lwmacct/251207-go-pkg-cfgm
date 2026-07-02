package cfgm

import (
	"bytes"
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v3"
)

func writeTempConfig(t *testing.T, content string) string {
	t.Helper()

	tmpFile, err := os.CreateTemp(t.TempDir(), "config_test_*.yaml")
	require.NoError(t, err)
	_, err = tmpFile.WriteString(content)
	require.NoError(t, err)
	require.NoError(t, tmpFile.Close())

	return tmpFile.Name()
}

func TestLoadUsesExplicitSourcesInOrder(t *testing.T) {
	type Config struct {
		Name  string `json:"name"`
		Debug bool   `json:"debug"`
	}

	configPath := writeTempConfig(t, `
name: "from-file"
debug: true
`)
	t.Setenv("APP_NAME", "from-env")

	cfg, err := Load(
		context.Background(),
		Config{Name: "default"},
		File(configPath),
		Env("APP_"),
	)
	require.NoError(t, err)

	assert.Equal(t, "from-env", cfg.Name)
	assert.True(t, cfg.Debug)
}

func TestMustLoadPanicsOnError(t *testing.T) {
	type Config struct {
		Name string `json:"name"`
	}

	assert.PanicsWithValue(t,
		"cfgm: failed to load config: file:/missing/config.yaml: none of the config files exist: /missing/config.yaml",
		func() {
			MustLoad(context.Background(), Config{}, File("/missing/config.yaml"))
		},
	)
}

func TestLoadReportWithLogger(t *testing.T) {
	type Config struct {
		Name string `json:"name"`
	}

	configPath := writeTempConfig(t, `name: "from-file"`)
	var logs bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug}))

	cfg, report, err := LoadReport(context.Background(), Config{}, Logger(logger), File(configPath))
	require.NoError(t, err)

	assert.Equal(t, "from-file", cfg.Name)
	require.Len(t, report.Sources, 1)
	assert.Equal(t, "file:"+configPath, report.Sources[0].Name)
	assert.Equal(t, []string{"name"}, report.Sources[0].Keys)
	assert.Contains(t, logs.String(), "msg=\"Loaded config source\"")
}

func TestLoadRejectsUnknownKeysByDefault(t *testing.T) {
	type Config struct {
		Name string `json:"name"`
	}

	configPath := writeTempConfig(t, `
name: "app"
typo: true
`)

	_, err := Load(context.Background(), Config{}, File(configPath))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown config keys")
	assert.Contains(t, err.Error(), "typo")
}

func TestLoadAllowUnknownKeys(t *testing.T) {
	type Config struct {
		Name string `json:"name"`
	}

	configPath := writeTempConfig(t, `
name: "app"
typo: true
`)

	cfg, err := Load(context.Background(), Config{}, AllowUnknownKeys(), File(configPath))
	require.NoError(t, err)
	assert.Equal(t, "app", cfg.Name)
}

func TestFileOptionalAndRequired(t *testing.T) {
	type Config struct {
		Name string `json:"name"`
	}

	cfg, err := Load(context.Background(), Config{Name: "default"}, File("/path/does/not/exist.yaml", Optional()))
	require.NoError(t, err)
	assert.Equal(t, "default", cfg.Name)

	_, err = Load(context.Background(), Config{}, File("/path/does/not/exist.yaml"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "none of the config files exist")
}

func TestTemplateExpansionIsEnabledByDefault(t *testing.T) {
	type Config struct {
		Name     string `json:"name"`
		Fallback string `json:"fallback"`
	}

	t.Setenv("CFG_NAME", "from-template")
	t.Setenv("CFG_DEFAULT", "from-default-template")
	configPath := writeTempConfig(t, `name: "${CFG_NAME}"`)

	cfg, err := Load(
		context.Background(),
		Config{Fallback: "${CFG_DEFAULT}"},
		File(configPath),
	)
	require.NoError(t, err)

	assert.Equal(t, "from-template", cfg.Name)
	assert.Equal(t, "from-default-template", cfg.Fallback)
}

func TestNoTemplateExpansion(t *testing.T) {
	type Config struct {
		Name     string `json:"name"`
		Fallback string `json:"fallback"`
	}

	t.Setenv("CFG_NAME", "from-template")
	t.Setenv("CFG_DEFAULT", "from-default-template")
	configPath := writeTempConfig(t, `name: "${CFG_NAME}"`)

	cfg, err := Load(
		context.Background(),
		Config{Fallback: "${CFG_DEFAULT}"},
		NoTemplateExpansion(),
		File(configPath),
	)
	require.NoError(t, err)

	assert.Equal(t, "${CFG_NAME}", cfg.Name)
	assert.Equal(t, "${CFG_DEFAULT}", cfg.Fallback)
}

func TestCommandProfileLoadsConfigEnvAndFlags(t *testing.T) {
	type ServerConfig struct {
		Addr    string        `json:"addr"`
		Timeout time.Duration `json:"timeout"`
	}
	type Config struct {
		Name   string       `json:"name"`
		Server ServerConfig `json:"server"`
	}

	configPath := writeTempConfig(t, `
name: "from-file"
server:
  addr: ":8080"
`)
	t.Setenv("APP_NAME", "from-env")

	var loaded *Config
	cmd := &cli.Command{
		Name: "server",
		Flags: []cli.Flag{
			ConfigFlag(),
			EnvPrefixFlag(),
			&cli.StringFlag{Name: "addr"},
			&cli.DurationFlag{Name: "timeout"},
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			cfg, err := Load(ctx, Config{}, Command(cmd))
			loaded = cfg

			return err
		},
	}

	err := cmd.Run(context.Background(), []string{
		"server",
		"--config", configPath,
		"--env-prefix", "APP_",
		"--addr", ":9090",
		"--timeout", "5s",
	})
	require.NoError(t, err)
	require.NotNil(t, loaded)

	assert.Equal(t, "from-env", loaded.Name)
	assert.Equal(t, ":9090", loaded.Server.Addr)
	assert.Equal(t, 5*time.Second, loaded.Server.Timeout)
}

func TestCommandProfileExpandsDefaultsByDefault(t *testing.T) {
	type Config struct {
		Name string `json:"name"`
	}

	t.Setenv("APP_NAME_DEFAULT", "from-default-template")

	var loaded *Config
	cmd := &cli.Command{
		Name: "app",
		Action: func(ctx context.Context, cmd *cli.Command) error {
			cfg, err := Load(ctx, Config{Name: "${APP_NAME_DEFAULT}"}, Command(cmd))
			loaded = cfg

			return err
		},
	}

	err := cmd.Run(context.Background(), []string{"app"})
	require.NoError(t, err)
	require.NotNil(t, loaded)
	assert.Equal(t, "from-default-template", loaded.Name)
}

func TestCommandProfileCanIgnoreNonConfigFlags(t *testing.T) {
	type Config struct {
		Name string `json:"name"`
	}

	cmd := &cli.Command{
		Name: "app",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "name"},
			&cli.BoolFlag{Name: "dry-run"},
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			_, err := Load(ctx, Config{}, Command(cmd, IgnoreFlags("dry-run")))

			return err
		},
	}

	err := cmd.Run(context.Background(), []string{"app", "--name", "from-cli", "--dry-run"})
	require.NoError(t, err)
}

func TestCommandProfileRejectsUnmappedFlags(t *testing.T) {
	type Config struct {
		Name string `json:"name"`
	}

	cmd := &cli.Command{
		Name:  "app",
		Flags: []cli.Flag{&cli.BoolFlag{Name: "dry-run"}},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			_, err := Load(ctx, Config{}, Command(cmd))

			return err
		},
	}

	err := cmd.Run(context.Background(), []string{"app", "--dry-run"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "has no matching config field")
}

func TestDefaultPaths(t *testing.T) {
	assert.Equal(t, []string{"config.yaml", "config/config.yaml"}, DefaultPaths())
	assert.Len(t, DefaultPaths("app"), 5)
}
