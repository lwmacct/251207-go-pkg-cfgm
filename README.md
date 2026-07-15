# cfgm

[![License](https://img.shields.io/github/license/lwmacct/251207-go-pkg-cfgm)](LICENSE)
[![Go Reference](https://pkg.go.dev/badge/github.com/lwmacct/251207-go-pkg-cfgm.svg)](https://pkg.go.dev/github.com/lwmacct/251207-go-pkg-cfgm)
[![Go CI](https://github.com/lwmacct/251207-go-pkg-cfgm/actions/workflows/go-ci.yml/badge.svg)](https://github.com/lwmacct/251207-go-pkg-cfgm/actions/workflows/go-ci.yml)

Schema 驱动的 Go 配置库。一个 `Definition[T]` 同时负责默认值、文件、环境变量、urfave/cli flags、严格校验和示例配置，避免应用手写 flags 后再靠反射猜测映射关系。

> 当前 API 是破坏式 vNext，不兼容旧的包级 `Load`、`Loader`、`Command`、`CLI` 和手工 flag coverage API。

## 安装

```bash
go get github.com/lwmacct/251207-go-pkg-cfgm/pkg/cfgm
```

## 运行示例

非 CLI 加载：

```bash
REDIS_URL=redis://localhost:6379/1 REDISCLI_AUTH=secret \
  APP_ENDPOINT=https://localhost:8443 APP_TIMEOUT=5s \
  go run ./examples/basic
```

示例会加载 [`examples/basic/config.yaml`](examples/basic/config.yaml)，文件内容如下：

```yaml
endpoint: "${APP_ENDPOINT:-https://api.example.com}"
timeout: "${APP_TIMEOUT:-30s}"

redis:
  url: "${REDIS_URL:-redis://localhost:6379/0}"
  password: "${REDISCLI_AUTH}"
```

加载时 cfgm 会先展开模板，再解析 YAML：

```go
loaded, err := definition.Load(ctx,
    cfgm.File("examples/basic/config.yaml"),
    cfgm.Env("APP_"),
)
```

`${VAR}` 在变量不存在时展开为空字符串；`${VAR:-fallback}` 在变量不存在或为空时使用 fallback。示例只输出 `redis_password_set`，不会打印密码内容。

自动生成 CLI flags：

```bash
go run ./examples/cli server --addr=:9090 --timeout=10s --tags=api
```

复合结构使用重复的 JSON object flags：

```bash
go run ./examples/cli server \
  --certificates='{"id":"main","certificate":"file:///cert.pem","private-key":"file:///key.pem"}'
```

## 定义配置

```go
type Config struct {
    Server ServerConfig `json:"server" desc:"服务端配置"`
    Redis  RedisConfig  `json:"redis"  desc:"Redis 配置"`
}

type ServerConfig struct {
    Addr    string        `json:"addr"    desc:"监听地址"`
    Timeout time.Duration `json:"timeout" desc:"请求超时"`
}

var Definition = cfgm.New(DefaultConfig(), cfgm.AppName("app"))
```

配置根类型必须是非指针 struct。`json` tag 是文件、环境变量和 CLI 的稳定 key，`desc` tag 用作 CLI help 和示例配置注释。

## 非 CLI 加载

```go
config, err := Definition.Load(ctx,
    cfgm.File("/etc/app/config.yaml", cfgm.Optional()),
    cfgm.Env("APP_"),
)
```

来源按声明顺序合并，后面的来源覆盖前面的来源。除非使用 `WithoutDefaultPaths()`，`Definition` 会先搜索 `DefaultPaths(appName)`；启动阶段也可使用 `MustLoad`，诊断时使用 `LoadReport`。

默认严格拒绝未知字段，并递归校验 struct、struct slice 和 map 中的已知结构。`AllowUnknownKeys()` 只允许额外字段，不会关闭已知字段的形状校验。

## CLI 集成

`Definition` 生成根命令的配置文件和环境前缀 flags：

```go
app := &cli.Command{
    Name:  "app",
    Flags: Definition.Flags(), // --config/-c, --env-prefix/-e
}
```

命令通过 binding 选择它拥有的配置字段：

```go
var serverBinding = Definition.Bind(
    cfgm.Scope("server"),
    cfgm.Include("redis"),
    cfgm.Alias("server.addr", "a"),
    cfgm.NoCLI("redis.password"),
)

var serverCommand = &cli.Command{
    Name:   "server",
    Flags:  serverBinding.Flags(),
    Action: func(ctx context.Context, cmd *cli.Command) error {
        config, err := serverBinding.Load(ctx, cmd)
        if err != nil {
            return err
        }
        return runServer(ctx, config)
    },
}
```

`Scope("server")` 将 `server.addr` 暴露为 `--addr`；`Include("redis")` 额外暴露 `--redis.url` 等字段。不存在的路径、冲突 alias 和保留名称会在 `Bind` 时直接 panic，而不是运行到加载阶段才失败。

CLI 加载优先级固定为：

1. 默认值
2. app 对应的默认配置路径
3. 显式 `--config/-c`
4. `--env-prefix/-e`，未设置时使用 app 名推导出的前缀
5. 当前 binding 中显式设置的 CLI flags

未显式设置的 CLI flag 不参与覆盖。

## 集合值

标量 slice 使用 urfave 的重复 flag：

```bash
app server --tags api --tags edge
```

`[]struct` 和 `[]*struct` 使用 cfgm 的严格 JSON object flag。每次出现添加一个元素，整组替换低优先级来源：

```bash
app server \
  --certificates='{"id":"main","certificate":"op://cert/main","private-key":"op://key/main"}' \
  --certificates='{"id":"api","certificate":"op://cert/api","private-key":"op://key/api"}'
```

使用 `--certificates=[]` 清空集合。`[]` 不能和 object 值混用。object 中的未知字段（包括嵌套 struct slice）会被拒绝。

环境变量中的 slice 和 map 必须是完整 JSON：

```bash
export APP_SERVER_TAGS='["api","edge"]'
export APP_SERVER_CERTIFICATES='[{"id":"main","refresh":"30s"}]'
export APP_LABELS='{"region":"cn"}'
```

这样不会受到逗号分隔规则影响，也能明确表达空数组和空对象。

## 自定义类型

无法由内置 flag 表达的叶子类型使用 `WithCodec`：

```go
definition := cfgm.New(defaults, cfgm.WithCodec(cfgm.Codec[Endpoint]{
    Parse:  ParseEndpoint,
    Format: func(value Endpoint) string { return value.String() },
}))
```

同一 codec 适用于文件、环境变量和 CLI。`Parse` 必填，`Format` 只负责生成 CLI 默认值文本。

## 模板与文件

默认值和内置 file source 默认展开 `${...}`。`WithoutTemplateExpansion()` 全局关闭；单个 file source 可用 `Raw()` 强制保留，或用 `ExpandTemplates()` 强制展开。

```go
definition := cfgm.New(defaults, cfgm.WithoutTemplateExpansion())
config, err := definition.Load(ctx,
    cfgm.File("config.yaml", cfgm.ExpandTemplates()),
)
```

## 示例配置

```go
yaml := cfgm.ExampleYAML(DefaultConfig())
jsonBytes := cfgm.MarshalJSON(DefaultConfig())

var files = cfgm.ConfigFiles[Config]{
    Definition:  Definition,
    ExampleFile: "config/config.example.yaml",
    RuntimeFile: "config/config.yaml",
}

func TestWriteConfigExample(t *testing.T)     { files.WriteExample(t) }
func TestRuntimeConfigKeysValid(t *testing.T) { files.ValidateRuntimeConfig(t) }
```

`ValidateRuntimeConfig` 使用 `Definition` 的同一份 Schema 和 codec 规则，不再从 example 文件推导第二套校验语义。

## License

MIT
