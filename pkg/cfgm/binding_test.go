package cfgm

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v3"
)

type bindingTLSCertificate struct {
	ID          string        `json:"id"          desc:"证书标识"`
	Certificate string        `json:"certificate" desc:"证书 URI"`
	PrivateKey  string        `json:"private-key" desc:"私钥 URI"`
	Refresh     time.Duration `json:"refresh"     desc:"刷新间隔"`
}

type bindingRoute struct {
	Path     string `json:"path"`
	Backends []struct {
		URL string `json:"url"`
	} `json:"backends"`
}

type bindingTestConfig struct {
	Server struct {
		Addr         string                  `json:"addr"         desc:"监听地址"`
		Debug        bool                    `json:"debug"        desc:"调试模式"`
		Workers      int                     `json:"workers"      desc:"工作线程"`
		Tags         []string                `json:"tags"         desc:"标签"`
		Certificates []bindingTLSCertificate `json:"certificates" desc:"证书列表"`
		Routes       []bindingRoute          `json:"routes"       desc:"路由列表"`
	} `json:"server" desc:"服务端配置"`
	Redis struct {
		URL      string `json:"url"      desc:"Redis URL"`
		Password string `json:"password" desc:"Redis 密码"`
	} `json:"redis" desc:"Redis 配置"`
}

func bindingDefaults() bindingTestConfig {
	var cfg bindingTestConfig
	cfg.Server.Addr = ":8080"
	cfg.Server.Workers = 2
	cfg.Server.Tags = []string{"default"}
	cfg.Server.Certificates = []bindingTLSCertificate{{
		ID:          "default",
		Certificate: "file:///default.crt",
		PrivateKey:  "file:///default.key",
		Refresh:     time.Minute,
	}}
	cfg.Redis.URL = "redis://localhost:6379"
	return cfg
}

func TestBindingGeneratesTypedFlags(t *testing.T) {
	definition := New(bindingDefaults(), AppName("testapp"))
	binding := definition.Bind(
		Scope("server"),
		Include("redis"),
		Alias("server.addr", "a"),
		NoCLI("redis.password"),
	)

	rootFlags := definition.Flags()
	requireFlagType[*cli.StringFlag](t, rootFlags, "config")
	requireFlagType[*cli.StringFlag](t, rootFlags, "env-prefix")

	flags := binding.Flags()
	addr := requireFlagType[*cli.StringFlag](t, flags, "addr")
	assert.Contains(t, addr.Aliases, "a")
	requireFlagType[*cli.BoolFlag](t, flags, "debug")
	requireFlagType[*cli.IntFlag](t, flags, "workers")
	requireFlagType[*cli.StringSliceFlag](t, flags, "tags")
	requireFlagType[*cli.GenericFlag](t, flags, "certificates")
	requireFlagType[*cli.StringFlag](t, flags, "redis.url")
	assert.Nil(t, findFlag(flags, "redis.password"))
}

func TestBindingLoadsSourcesInPriorityOrder(t *testing.T) {
	definition := New(bindingDefaults(), AppName("testapp"))
	binding := definition.Bind(Scope("server"), Include("redis"), NoCLI("redis.password"))
	configPath := writeTempConfig(t, `
server:
  addr: ":8000"
  workers: 3
redis:
  url: "redis://file:6379"
`)
	t.Setenv("TESTAPP_SERVER_ADDR", ":8100")
	t.Setenv("TESTAPP_REDIS_URL", "redis://env:6379")

	var loaded *bindingTestConfig
	root := &cli.Command{
		Name:  "testapp",
		Flags: definition.Flags(),
		Commands: []*cli.Command{{
			Name:  "server",
			Flags: binding.Flags(),
			Action: func(ctx context.Context, cmd *cli.Command) error {
				var err error
				loaded, err = binding.Load(ctx, cmd)
				return err
			},
		}},
	}

	err := root.Run(t.Context(), []string{
		"testapp", "--config", configPath, "server",
		"--addr=:8200",
		"--workers=4",
	})
	require.NoError(t, err)
	require.NotNil(t, loaded)
	assert.Equal(t, ":8200", loaded.Server.Addr)
	assert.Equal(t, 4, loaded.Server.Workers)
	assert.Equal(t, "redis://env:6379", loaded.Redis.URL)
}

func TestBindingLoadsRepeatedStructValues(t *testing.T) {
	definition := New(bindingDefaults())
	binding := definition.Bind(Scope("server"))
	loaded, err := runBinding(t, definition, binding,
		`--certificates={"id":"main","certificate":"op://cert/main","private-key":"op://key/main","refresh":"30s"}`,
		`--certificates={"id":"api","certificate":"op://cert/api","private-key":"op://key/api","refresh":"1m"}`,
	)
	require.NoError(t, err)
	require.Len(t, loaded.Server.Certificates, 2)
	assert.Equal(t, "main", loaded.Server.Certificates[0].ID)
	assert.Equal(t, 30*time.Second, loaded.Server.Certificates[0].Refresh)
	assert.Equal(t, "api", loaded.Server.Certificates[1].ID)
	assert.Equal(t, time.Minute, loaded.Server.Certificates[1].Refresh)
}

func TestBindingStructValuesReplaceLowerPrioritySources(t *testing.T) {
	definition := New(bindingDefaults())
	binding := definition.Bind(Scope("server"))
	loaded, err := runBinding(t, definition, binding,
		`--certificates={"id":"only","certificate":"file:///only.crt","private-key":"file:///only.key"}`,
	)
	require.NoError(t, err)
	require.Len(t, loaded.Server.Certificates, 1)
	assert.Equal(t, "only", loaded.Server.Certificates[0].ID)
}

func TestBindingStructValuesCanBeCleared(t *testing.T) {
	definition := New(bindingDefaults())
	binding := definition.Bind(Scope("server"))
	loaded, err := runBinding(t, definition, binding, `--certificates=[]`)
	require.NoError(t, err)
	assert.Empty(t, loaded.Server.Certificates)
}

func TestBindingScalarCollectionsReplaceDefaults(t *testing.T) {
	definition := New(bindingDefaults())
	binding := definition.Bind(Scope("server"))
	loaded, err := runBinding(t, definition, binding, `--tags=cli`)
	require.NoError(t, err)
	assert.Equal(t, []string{"cli"}, loaded.Server.Tags)
}

func TestBindingRejectsInvalidStructValues(t *testing.T) {
	definition := New(bindingDefaults())
	binding := definition.Bind(Scope("server"))
	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "invalid json", args: []string{`--certificates={`}, want: "certificates"},
		{name: "unknown field", args: []string{`--certificates={"id":"main","unknown":true}`}, want: "unknown"},
		{name: "non object", args: []string{`--certificates="main"`}, want: "JSON object"},
		{name: "clear mixed with value", args: []string{`--certificates=[]`, `--certificates={"id":"main"}`}, want: "cannot be combined"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := runBinding(t, definition, binding, test.args...)
			require.ErrorContains(t, err, test.want)
		})
	}
}

type bindingEndpoint string

func TestBindingUsesRegisteredCodec(t *testing.T) {
	type codecConfig struct {
		Endpoint bindingEndpoint `json:"endpoint" desc:"服务端点"`
	}
	definition := New(codecConfig{}, WithCodec(Codec[bindingEndpoint]{
		Parse: func(value string) (bindingEndpoint, error) {
			if !strings.HasPrefix(value, "svc://") {
				return "", errors.New("scheme must be svc")
			}
			return bindingEndpoint(value), nil
		},
		Format: func(value bindingEndpoint) string { return string(value) },
	}))
	binding := definition.Bind()

	var loaded *codecConfig
	cmd := &cli.Command{
		Name:  "app",
		Flags: binding.Flags(),
		Action: func(ctx context.Context, cmd *cli.Command) error {
			var err error
			loaded, err = binding.Load(ctx, cmd)
			return err
		},
	}
	require.NoError(t, cmd.Run(t.Context(), []string{"app", "--endpoint=svc://main"}))
	assert.Equal(t, bindingEndpoint("svc://main"), loaded.Endpoint)
	require.ErrorContains(t, cmd.Run(t.Context(), []string{"app", "--endpoint=http://main"}), "scheme must be svc")
}

func TestBindingRejectsUnknownFileFieldsInsideStructSlices(t *testing.T) {
	definition := New(bindingDefaults())
	binding := definition.Bind(Scope("server"))
	configPath := writeTempConfig(t, `
server:
  certificates:
    - id: main
      certificate: file:///main.crt
      private-key: file:///main.key
      unknown: true
`)

	_, err := runBindingWithRootArgs(t, definition, binding, []string{"--config", configPath})
	require.ErrorContains(t, err, "unknown")
}

func TestDefinitionLoadsCompositeEnvironmentValuesAsJSON(t *testing.T) {
	type Config struct {
		Tags         []string                `json:"tags"`
		Labels       map[string]string       `json:"labels"`
		Certificates []bindingTLSCertificate `json:"certificates"`
	}
	t.Setenv("APP_TAGS", `["api","edge"]`)
	t.Setenv("APP_LABELS", `{"region":"cn"}`)
	t.Setenv("APP_CERTIFICATES", `[{"id":"main","refresh":"15s"}]`)

	cfg, err := New(Config{}, WithoutDefaultPaths()).Load(t.Context(), Env("APP_"))
	require.NoError(t, err)
	assert.Equal(t, []string{"api", "edge"}, cfg.Tags)
	assert.Equal(t, map[string]string{"region": "cn"}, cfg.Labels)
	require.Len(t, cfg.Certificates, 1)
	assert.Equal(t, "main", cfg.Certificates[0].ID)
	assert.Equal(t, 15*time.Second, cfg.Certificates[0].Refresh)
}

func TestDefinitionEnvironmentCanSetEmptyScalar(t *testing.T) {
	type Config struct {
		Name string `json:"name"`
	}
	t.Setenv("APP_NAME", "")
	cfg, err := New(Config{Name: "default"}, WithoutDefaultPaths()).Load(t.Context(), Env("APP_"))
	require.NoError(t, err)
	assert.Empty(t, cfg.Name)
}

func TestDefinitionRejectsInvalidCompositeEnvironmentValues(t *testing.T) {
	type Config struct {
		Tags []string `json:"tags"`
	}
	t.Setenv("APP_TAGS", "api,edge")
	_, err := New(Config{}, WithoutDefaultPaths()).Load(t.Context(), Env("APP_"))
	require.ErrorContains(t, err, "APP_TAGS")
	require.ErrorContains(t, err, "JSON")
}

func TestDefinitionRejectsDeepUnknownFields(t *testing.T) {
	type Config struct {
		Routes []bindingRoute `json:"routes"`
	}
	path := writeTempConfig(t, `
routes:
  - path: /api
    backends:
      - url: https://api.example.com
        typo: true
`)
	_, err := New(Config{}, WithoutDefaultPaths()).Load(t.Context(), File(path))
	require.ErrorContains(t, err, "routes.backends.typo")
}

func TestBindingRejectsDeepUnknownStructFlagFields(t *testing.T) {
	definition := New(bindingDefaults())
	binding := definition.Bind(Scope("server"))
	_, err := runBinding(t, definition, binding,
		`--routes={"path":"/api","backends":[{"url":"https://api.example.com","typo":true}]}`,
	)
	require.ErrorContains(t, err, "backends.typo")
}

func TestDefinitionCodecAppliesToFileAndEnvironment(t *testing.T) {
	type Config struct {
		Endpoint bindingEndpoint `json:"endpoint"`
	}
	definition := New(Config{}, WithoutDefaultPaths(), WithCodec(Codec[bindingEndpoint]{
		Parse: func(value string) (bindingEndpoint, error) {
			if !strings.HasPrefix(value, "svc://") {
				return "", errors.New("scheme must be svc")
			}
			return bindingEndpoint(value), nil
		},
	}))
	path := writeTempConfig(t, "endpoint: svc://file\n")
	cfg, err := definition.Load(t.Context(), File(path))
	require.NoError(t, err)
	assert.Equal(t, bindingEndpoint("svc://file"), cfg.Endpoint)

	t.Setenv("APP_ENDPOINT", "svc://env")
	cfg, err = definition.Load(t.Context(), Env("APP_"))
	require.NoError(t, err)
	assert.Equal(t, bindingEndpoint("svc://env"), cfg.Endpoint)

	path = writeTempConfig(t, "endpoint: http://invalid\n")
	_, err = definition.Load(t.Context(), File(path))
	require.ErrorContains(t, err, "scheme must be svc")

	path = writeTempConfig(t, "endpoint: 42\n")
	_, err = definition.Load(t.Context(), File(path))
	require.ErrorContains(t, err, "must be a string for codec")
}

func TestDefinitionRejectsInvalidOptionsAndSchema(t *testing.T) {
	type Config struct {
		Name string `json:"name"`
	}
	assert.PanicsWithValue(t, "cfgm: codec for string requires Parse", func() {
		New(Config{}, WithCodec(Codec[string]{}))
	})
	assert.PanicsWithValue(t, "cfgm: logger must not be nil", func() {
		New(Config{}, Logger(nil))
	})
	assert.Panics(t, func() { New("") })
	assert.Panics(t, func() { New((*Config)(nil)) })
}

func TestBindingTreatsCodecStructAsLeaf(t *testing.T) {
	type Endpoint struct {
		Scheme string
		Name   string
	}
	type Config struct {
		Endpoint Endpoint `json:"endpoint"`
	}
	definition := New(Config{}, WithCodec(Codec[Endpoint]{
		Parse: func(value string) (Endpoint, error) {
			parts := strings.SplitN(value, "://", 2)
			if len(parts) != 2 {
				return Endpoint{}, errors.New("invalid endpoint")
			}
			return Endpoint{Scheme: parts[0], Name: parts[1]}, nil
		},
		Format: func(value Endpoint) string { return value.Scheme + "://" + value.Name },
	}))
	binding := definition.Bind()
	requireFlagType[*cli.StringFlag](t, binding.Flags(), "endpoint")
}

func TestBindingUsesCodecInsideStructSlice(t *testing.T) {
	type Endpoint struct {
		Name string
	}
	type Service struct {
		Endpoint Endpoint `json:"endpoint"`
	}
	type Config struct {
		Services []Service `json:"services"`
	}
	definition := New(Config{}, WithCodec(Codec[Endpoint]{
		Parse: func(value string) (Endpoint, error) { return Endpoint{Name: value}, nil },
	}))
	binding := definition.Bind()
	var loaded *Config
	cmd := &cli.Command{
		Name:  "app",
		Flags: binding.Flags(),
		Action: func(ctx context.Context, cmd *cli.Command) error {
			var err error
			loaded, err = binding.Load(ctx, cmd)
			return err
		},
	}
	require.NoError(t, cmd.Run(t.Context(), []string{"app", `--services={"endpoint":"main"}`}))
	require.Len(t, loaded.Services, 1)
	assert.Equal(t, "main", loaded.Services[0].Endpoint.Name)
}

func TestBindingSupportsNamedScalarTypes(t *testing.T) {
	type Name string
	type Count int
	type Config struct {
		Name  Name  `json:"name"`
		Count Count `json:"count"`
	}
	definition := New(Config{Name: "default", Count: 2})
	binding := definition.Bind()
	requireFlagType[*cli.StringFlag](t, binding.Flags(), "name")
	requireFlagType[*cli.IntFlag](t, binding.Flags(), "count")
}

func TestBindingSupportsNamedScalarSliceTypes(t *testing.T) {
	type Name string
	type Names []Name
	type Config struct {
		Names Names `json:"names"`
	}
	definition := New(Config{Names: Names{"default"}})
	binding := definition.Bind()
	requireFlagType[*cli.StringSliceFlag](t, binding.Flags(), "names")
}

func TestBindingSupportsNamedStringMapTypes(t *testing.T) {
	type Labels map[string]string
	type Config struct {
		Labels Labels `json:"labels"`
	}
	definition := New(Config{Labels: Labels{"default": "yes"}})
	binding := definition.Bind()
	requireFlagType[*cli.StringMapFlag](t, binding.Flags(), "labels")
}

func TestDefinitionRejectsAmbiguousSchemaKeys(t *testing.T) {
	assert.PanicsWithError(t, `cfgm: duplicate config path "name"`, func() {
		typ := reflect.StructOf([]reflect.StructField{
			{Name: "First", Type: reflect.TypeFor[string](), Tag: `json:"name"`},
			{Name: "Second", Type: reflect.TypeFor[string](), Tag: `json:"name"`},
		})
		buildSchemaModel(typ, nil)
	})
	assert.PanicsWithError(t, `cfgm: config key "server.addr" must not contain dots`, func() {
		New(struct {
			Addr string `json:"server.addr"`
		}{})
	})
	type Node struct {
		Children []Node `json:"children"`
	}
	assert.Panics(t, func() { New(Node{}) })
}

func TestBindingRejectsInvalidPathsAndAliases(t *testing.T) {
	definition := New(bindingDefaults())
	tests := []struct {
		name string
		bind func()
		want string
	}{
		{name: "scope", bind: func() { definition.Bind(Scope("missing")) }, want: "scope"},
		{name: "include", bind: func() { definition.Bind(Include("missing")) }, want: "include"},
		{name: "no CLI", bind: func() { definition.Bind(NoCLI("missing")) }, want: "no-CLI"},
		{name: "alias path", bind: func() { definition.Bind(Alias("missing", "m")) }, want: "alias path"},
		{name: "excluded alias", bind: func() { definition.Bind(Scope("server"), Alias("redis.url", "r")) }, want: "excluded"},
		{name: "reserved alias", bind: func() { definition.Bind(Alias("server.addr", "c")) }, want: "reserved"},
		{name: "help alias", bind: func() { definition.Bind(Alias("server.addr", "h")) }, want: "reserved"},
		{name: "alias collision", bind: func() {
			definition.Bind(Alias("server.addr", "x"), Alias("server.debug", "x"))
		}, want: "ambiguous"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			defer func() {
				value := recover()
				require.NotNil(t, value)
				assert.Contains(t, fmt.Sprint(value), test.want)
			}()
			test.bind()
		})
	}
}

func TestBindingRejectsGeneratedReservedNames(t *testing.T) {
	type Config struct {
		Config string `json:"config"`
	}
	definition := New(Config{})
	assert.PanicsWithError(t, "cfgm: generated CLI flag --config is reserved by definition flags", func() {
		definition.Bind()
	})
}

func TestDefinitionAllowsNullCollections(t *testing.T) {
	type Config struct {
		Names  []string          `json:"names"`
		Labels map[string]string `json:"labels"`
	}
	path := writeTempConfig(t, "names: null\nlabels: null\n")
	cfg, err := New(Config{}, WithoutDefaultPaths()).Load(t.Context(), File(path))
	require.NoError(t, err)
	assert.Nil(t, cfg.Names)
	assert.Nil(t, cfg.Labels)
}

func TestBindingSupportsNilCompositeDefaultsAndTimestamps(t *testing.T) {
	type Config struct {
		Tags   []string          `json:"tags"`
		Labels map[string]string `json:"labels"`
		At     time.Time         `json:"at"`
	}
	definition := New(Config{})
	binding := definition.Bind()
	requireFlagType[*cli.StringSliceFlag](t, binding.Flags(), "tags")
	requireFlagType[*cli.StringMapFlag](t, binding.Flags(), "labels")
	requireFlagType[*cli.TimestampFlag](t, binding.Flags(), "at")

	var loaded *Config
	cmd := &cli.Command{
		Name:  "app",
		Flags: binding.Flags(),
		Action: func(ctx context.Context, cmd *cli.Command) error {
			var err error
			loaded, err = binding.Load(ctx, cmd)
			return err
		},
	}
	require.NoError(t, cmd.Run(t.Context(), []string{"app", "--at=2026-07-15T10:20:30Z"}))
	assert.Equal(t, time.Date(2026, 7, 15, 10, 20, 30, 0, time.UTC), loaded.At)
}

func runBinding(
	t *testing.T,
	definition *Definition[bindingTestConfig],
	binding *Binding[bindingTestConfig],
	args ...string,
) (*bindingTestConfig, error) {
	t.Helper()
	return runBindingWithRootArgs(t, definition, binding, nil, args...)
}

func runBindingWithRootArgs(
	t *testing.T,
	definition *Definition[bindingTestConfig],
	binding *Binding[bindingTestConfig],
	rootArgs []string,
	args ...string,
) (*bindingTestConfig, error) {
	t.Helper()
	var loaded *bindingTestConfig
	root := &cli.Command{
		Name:  "app",
		Flags: definition.Flags(),
		Commands: []*cli.Command{{
			Name:  "server",
			Flags: binding.Flags(),
			Action: func(ctx context.Context, cmd *cli.Command) error {
				var err error
				loaded, err = binding.Load(ctx, cmd)
				return err
			},
		}},
	}
	commandArgs := append([]string{"app"}, rootArgs...)
	commandArgs = append(commandArgs, "server")
	commandArgs = append(commandArgs, args...)
	err := root.Run(t.Context(), commandArgs)
	return loaded, err
}

func requireFlagType[T cli.Flag](t *testing.T, flags []cli.Flag, name string) T {
	t.Helper()
	flag := findFlag(flags, name)
	require.NotNil(t, flag, "flag %q not found", name)
	typed, ok := flag.(T)
	require.True(t, ok, "flag %q has type %T, want %s", name, flag, reflect.TypeFor[T]())
	return typed
}

func findFlag(flags []cli.Flag, name string) cli.Flag {
	for _, flag := range flags {
		if flag.Names()[0] == name {
			return flag
		}
	}
	return nil
}
