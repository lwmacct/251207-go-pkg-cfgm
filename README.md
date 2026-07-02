# go-pkg-config

[![License](https://img.shields.io/github/license/lwmacct/251207-go-pkg-cfgm)](LICENSE)
[![Go Reference](https://pkg.go.dev/badge/github.com/lwmacct/251207-go-pkg-cfgm.svg)](https://pkg.go.dev/github.com/lwmacct/251207-go-pkg-cfgm)
[![Go CI](https://github.com/lwmacct/251207-go-pkg-cfgm/actions/workflows/go-ci.yml/badge.svg)](https://github.com/lwmacct/251207-go-pkg-cfgm/actions/workflows/go-ci.yml)
[![codecov](https://codecov.io/gh/lwmacct/251207-go-pkg-cfgm/branch/main/graph/badge.svg)](https://codecov.io/gh/lwmacct/251207-go-pkg-cfgm)
[![Go Report Card](https://goreportcard.com/badge/github.com/lwmacct/251207-go-pkg-cfgm)](https://goreportcard.com/report/github.com/lwmacct/251207-go-pkg-cfgm)

通用 Go 配置加载库。配置结构体用 `json` tag 描述 key，加载来源由调用方显式声明。

<!--TOC-->

## Table of Contents

- [特性](#特性) `:26+9`
- [安装](#安装) `:35+6`
- [快速开始](#快速开始) `:41+121`
  - [定义配置](#定义配置) `:43+24`
  - [加载配置](#加载配置) `:67+37`
  - [CLI 集成](#cli-集成) `:104+58`
- [辅助能力](#辅助能力) `:162+44`
- [License](#license) `:206+3`

<!--TOC-->

## 特性

- **显式来源**：默认值、文件、环境变量、CLI flags 都通过 options 声明。
- **泛型加载**：`Load[T]` / `MustLoad[T]` 直接返回强类型配置。
- **CLI profile**：`Command(cmd)` 封装 urfave/cli 常见约定。
- **严格校验**：默认拒绝配置文件中的未知 key。
- **加载报告**：`LoadReport` 返回每个来源贡献的 key。
- **模板展开**：文件和默认值默认启用 `${...}` 展开，可显式关闭。

## 安装

```bash
go get github.com/lwmacct/251207-go-pkg-cfgm/pkg/cfgm
```

## 快速开始

### 定义配置

```go
type Config struct {
    Server ServerConfig `json:"server" desc:"服务端配置"`
}

type ServerConfig struct {
    Addr    string        `json:"addr" desc:"监听地址"`
    Timeout time.Duration `json:"timeout" desc:"超时时间"`
}

func DefaultConfig() Config {
    return Config{
        Server: ServerConfig{
            Addr:    ":8080",
            Timeout: 30 * time.Second,
        },
    }
}
```

YAML 和 JSON 都使用 `json` tag 作为配置 key。

### 加载配置

```go
cfg, err := cfgm.Load(ctx,
    DefaultConfig(),
    cfgm.File("config/config.yaml", cfgm.Optional()),
    cfgm.Env("APP_"),
)
```

source 按声明顺序合并，后面的 source 覆盖前面的 source。只使用默认值时不传任何来源：

```go
cfg, err := cfgm.Load(ctx, DefaultConfig())
```

启动阶段可用 panic 版本：

```go
cfg := cfgm.MustLoad(ctx, DefaultConfig(),
    cfgm.File("config/config.yaml", cfgm.Optional()),
    cfgm.Env("APP_"),
)
```

需要排查配置来源时使用 `LoadReport`：

```go
cfg, report, err := cfgm.LoadReport(ctx,
    DefaultConfig(),
    cfgm.Logger(logger),
    cfgm.File("config/config.yaml", cfgm.Optional()),
    cfgm.Env("APP_"),
)
_ = report.Sources
```

### CLI 集成

高频 urfave/cli 调用：

```go
func action(ctx context.Context, cmd *cli.Command) error {
    cfg := cfgm.MustLoad(ctx,
        DefaultConfig(),
        cfgm.Command(cmd),
    )
    return run(ctx, cfg)
}
```

`Command(cmd)` 会按顺序加载：

1. 显式设置的 `--config / -c` 配置文件
2. `--env-prefix / -e` 指定的环境变量前缀，或根命令名转换出的前缀
3. 当前命令上显式设置的配置 flags

模板展开默认开启。需要保留原始 `${...}` 字符串时：

```go
cfg, err := cfgm.Load(ctx,
    DefaultConfig(),
    cfgm.NoTemplateExpansion(),
    cfgm.Command(cmd),
)
```

根命令可挂载约定 flags：

```go
app := &cli.Command{
    Name:  "app",
    Flags: []cli.Flag{cfgm.ConfigFlag(), cfgm.EnvPrefixFlag()},
}
```

命令可以包含不属于配置的业务 flags，用 `IgnoreFlags` 显式声明：

```go
cfg, err := cfgm.Load(ctx,
    DefaultConfig(),
    cfgm.Command(cmd, cfgm.IgnoreFlags("dry-run", "format")),
)
```

CLI flag 名称从配置路径推导，并按命令链递归剥离作用域：

| 命令链           | 配置 key              | 可声明的 flag   |
| ---------------- | --------------------- | --------------- |
| 无               | `server.addr`         | `--server.addr` |
| `server`         | `server.addr`         | `--addr`        |
| `server service` | `server.service.port` | `--port`        |

完整路径仍是 fallback 候选，例如 `server` 命令下仍可声明 `--redis.url`。

## 辅助能力

使用 `Schema` 从配置字段的 `desc` tag 生成 CLI help 文案：

```go
defaults := DefaultConfig()
usage := cfgm.Schema(defaults).Command("client")

flag := &cli.StringFlag{
    Name:  "url",
    Value: defaults.Client.URL,
    Usage: usage.MustUsage("url"),
}
```

测试中可校验命令是否覆盖指定配置前缀：

```go
cfgm.AssertCommandFlagCoverage(t, clientCommand, DefaultConfig(),
    []string{"client", "redis"},
    cfgm.IgnoreConfigKeys("redis.password"),
)
```

生成配置示例：

```go
yaml := cfgm.ExampleYAML(DefaultConfig())
jsonBytes := cfgm.MarshalJSON(DefaultConfig())
```

维护示例配置文件：

```go
var files = cfgm.ConfigFiles[Config]{
    Defaults:    DefaultConfig,
    ExampleFile: "config/config.example.yaml",
    RuntimeFile: "config/config.yaml",
}

func TestWriteConfigExample(t *testing.T) { files.WriteExample(t) }
func TestRuntimeConfigKeysValid(t *testing.T) { files.ValidateRuntimeConfig(t) }
```

## License

MIT
