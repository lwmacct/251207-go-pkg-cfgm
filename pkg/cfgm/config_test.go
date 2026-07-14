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
	require.Len(t, report.Sources, 2)
	assert.Equal(t, "files", report.Sources[0].Name)
	assert.Empty(t, report.Sources[0].Keys)
	assert.Equal(t, "file:"+configPath, report.Sources[1].Name)
	assert.Equal(t, []string{"name"}, report.Sources[1].Keys)
	assert.Contains(t, logs.String(), "msg=\"Loaded config source\"")
}

func TestLoadSearchesDefaultPaths(t *testing.T) {
	type Config struct {
		Name  string `json:"name"`
		Debug bool   `json:"debug"`
	}

	tmpDir := t.TempDir()
	t.Chdir(tmpDir)
	require.NoError(t, os.WriteFile("config.yaml", []byte(`
name: "from-default-path"
debug: true
`), 0600))

	cfg, err := Load(context.Background(), Config{Name: "default"})
	require.NoError(t, err)

	assert.Equal(t, "from-default-path", cfg.Name)
	assert.True(t, cfg.Debug)
}

func TestLoadSearchesDefaultJSONPaths(t *testing.T) {
	type Config struct {
		Name  string `json:"name"`
		Debug bool   `json:"debug"`
	}

	tmpDir := t.TempDir()
	t.Chdir(tmpDir)
	require.NoError(t, os.WriteFile("config.json", []byte(`{
  "name": "from-default-json",
  "debug": true
}`), 0600))

	cfg, err := Load(context.Background(), Config{Name: "default"})
	require.NoError(t, err)

	assert.Equal(t, "from-default-json", cfg.Name)
	assert.True(t, cfg.Debug)
}

func TestLoadSearchesDefaultYMLPaths(t *testing.T) {
	type Config struct {
		Name  string `json:"name"`
		Debug bool   `json:"debug"`
	}

	tmpDir := t.TempDir()
	t.Chdir(tmpDir)
	require.NoError(t, os.WriteFile("config.yml", []byte(`
name: "from-default-yml"
debug: true
`), 0600))

	cfg, err := Load(context.Background(), Config{Name: "default"})
	require.NoError(t, err)

	assert.Equal(t, "from-default-yml", cfg.Name)
	assert.True(t, cfg.Debug)
}

func TestDefaultPathYAMLPrecedesYMLAndJSONAtSameLocation(t *testing.T) {
	type Config struct {
		Name string `json:"name"`
	}

	tmpDir := t.TempDir()
	t.Chdir(tmpDir)
	require.NoError(t, os.WriteFile("config.yaml", []byte(`name: "from-yaml"`), 0600))
	require.NoError(t, os.WriteFile("config.yml", []byte(`name: "from-yml"`), 0600))
	require.NoError(t, os.WriteFile("config.json", []byte(`{"name": "from-json"}`), 0600))

	cfg, err := Load(context.Background(), Config{Name: "default"})
	require.NoError(t, err)

	assert.Equal(t, "from-yaml", cfg.Name)
}

func TestLoadDefaultPathsAreOptional(t *testing.T) {
	type Config struct {
		Name string `json:"name"`
	}

	t.Chdir(t.TempDir())

	cfg, err := Load(context.Background(), Config{Name: "default"})
	require.NoError(t, err)

	assert.Equal(t, "default", cfg.Name)
}

func TestNoDefaultPaths(t *testing.T) {
	type Config struct {
		Name string `json:"name"`
	}

	tmpDir := t.TempDir()
	t.Chdir(tmpDir)
	require.NoError(t, os.WriteFile("config.yaml", []byte(`name: "from-default-path"`), 0600))

	cfg, err := Load(context.Background(), Config{Name: "default"}, NoDefaultPaths())
	require.NoError(t, err)

	assert.Equal(t, "default", cfg.Name)
}

func TestExplicitSourcesOverrideDefaultPaths(t *testing.T) {
	type Config struct {
		Name string `json:"name"`
	}

	tmpDir := t.TempDir()
	t.Chdir(tmpDir)
	require.NoError(t, os.WriteFile("config.yaml", []byte(`name: "from-default-path"`), 0600))
	explicitPath := writeTempConfig(t, `name: "from-explicit-file"`)

	cfg, err := Load(context.Background(), Config{Name: "default"}, File(explicitPath))
	require.NoError(t, err)

	assert.Equal(t, "from-explicit-file", cfg.Name)
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

func TestNullableStructCanOverrideDefaultWithNull(t *testing.T) {
	type Provider struct {
		Issuer string `json:"issuer"`
	}
	type Config struct {
		Provider *Provider `json:"provider"`
	}

	configPath := writeTempConfig(t, "provider: null\n")
	cfg, err := Load(context.Background(), Config{Provider: &Provider{Issuer: "https://default.example.com"}}, File(configPath))
	require.NoError(t, err)
	assert.Nil(t, cfg.Provider)
}

func TestNullableStructCanOverrideNilWithObject(t *testing.T) {
	type Provider struct {
		Issuer string `json:"issuer"`
	}
	type Config struct {
		Provider *Provider `json:"provider"`
	}

	configPath := writeTempConfig(t, "provider:\n  issuer: https://auth.example.com\n")
	cfg, err := Load(context.Background(), Config{}, File(configPath))
	require.NoError(t, err)
	require.NotNil(t, cfg.Provider)
	assert.Equal(t, "https://auth.example.com", cfg.Provider.Issuer)
}

func TestNullableStructAcceptsEmptyObject(t *testing.T) {
	type Provider struct {
		Issuer string `json:"issuer"`
	}
	type Config struct {
		Provider *Provider `json:"provider"`
	}

	configPath := writeTempConfig(t, "provider: {}\n")
	cfg, err := Load(context.Background(), Config{}, File(configPath))
	require.NoError(t, err)
	require.NotNil(t, cfg.Provider)
	assert.Empty(t, cfg.Provider.Issuer)
}

func TestNonPointerStructRejectsNull(t *testing.T) {
	type Provider struct {
		Issuer string `json:"issuer"`
	}
	type Config struct {
		Provider Provider `json:"provider"`
	}

	configPath := writeTempConfig(t, "provider: null\n")
	_, err := Load(context.Background(), Config{}, File(configPath))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown config keys")
	assert.Contains(t, err.Error(), "provider")
}

func TestNullableStructStillRejectsUnknownSibling(t *testing.T) {
	type Provider struct {
		Issuer string `json:"issuer"`
	}
	type Config struct {
		Provider *Provider `json:"provider"`
	}

	configPath := writeTempConfig(t, "provider: null\nprovider-typo: null\n")
	_, err := Load(context.Background(), Config{}, File(configPath))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "provider-typo")
}

func TestExampleYAMLKeepsMapCommentOnMap(t *testing.T) {
	type Provider struct {
		Issuer string `json:"issuer"`
	}
	type Config struct {
		Credentials map[string]string `json:"credentials" desc:"Credential map"`
		Provider    *Provider         `json:"provider"    desc:"Optional provider"`
	}

	yaml := string(ExampleYAML(Config{
		Credentials: map[string]string{"admin": "digest"},
	}))
	assert.Contains(t, yaml, "# Credential map\ncredentials:")
	assert.Contains(t, yaml, "# Optional provider\nprovider: null")
	assert.NotContains(t, yaml, "provider: null # Credential map")
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

func TestCommandProfileUsesRootNameForDefaultPaths(t *testing.T) {
	type Config struct {
		Name string `json:"name"`
	}

	tmpDir := t.TempDir()
	t.Chdir(tmpDir)
	require.NoError(t, os.WriteFile(".app.yaml", []byte(`name: "from-app-default-path"`), 0600))
	require.NoError(t, os.WriteFile("config.yaml", []byte(`name: "from-generic-default-path"`), 0600))

	var loaded *Config
	cmd := &cli.Command{
		Name: "app",
		Action: func(ctx context.Context, cmd *cli.Command) error {
			cfg, err := Load(ctx, Config{Name: "default"}, Command(cmd))
			loaded = cfg

			return err
		},
	}

	err := cmd.Run(context.Background(), []string{"app"})
	require.NoError(t, err)
	require.NotNil(t, loaded)
	assert.Equal(t, "from-app-default-path", loaded.Name)
}

func TestCommandExplicitConfigOverridesDefaultPaths(t *testing.T) {
	type Config struct {
		Name string `json:"name"`
	}

	tmpDir := t.TempDir()
	t.Chdir(tmpDir)
	require.NoError(t, os.WriteFile(".app.yaml", []byte(`name: "from-app-default-path"`), 0600))
	configPath := writeTempConfig(t, `name: "from-explicit-config"`)

	var loaded *Config
	cmd := &cli.Command{
		Name:  "app",
		Flags: []cli.Flag{ConfigFlag()},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			cfg, err := Load(ctx, Config{Name: "default"}, Command(cmd))
			loaded = cfg

			return err
		},
	}

	err := cmd.Run(context.Background(), []string{"app", "--config", configPath})
	require.NoError(t, err)
	require.NotNil(t, loaded)
	assert.Equal(t, "from-explicit-config", loaded.Name)
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
	assert.Equal(t, []string{
		"config.yaml",
		"config.yml",
		"config.json",
		"config/config.yaml",
		"config/config.yml",
		"config/config.json",
	}, DefaultPaths())
	assert.Len(t, DefaultPaths("app"), 15)
}
