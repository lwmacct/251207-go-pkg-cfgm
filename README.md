# go-pkg-config

[![License](https://img.shields.io/github/license/lwmacct/251207-go-pkg-cfgm)](LICENSE)
[![Go Reference](https://pkg.go.dev/badge/github.com/lwmacct/251207-go-pkg-cfgm.svg)](https://pkg.go.dev/github.com/lwmacct/251207-go-pkg-cfgm)
[![Go CI](https://github.com/lwmacct/251207-go-pkg-cfgm/actions/workflows/go-ci.yml/badge.svg)](https://github.com/lwmacct/251207-go-pkg-cfgm/actions/workflows/go-ci.yml)
[![codecov](https://codecov.io/gh/lwmacct/251207-go-pkg-cfgm/branch/main/graph/badge.svg)](https://codecov.io/gh/lwmacct/251207-go-pkg-cfgm)
[![Go Report Card](https://goreportcard.com/badge/github.com/lwmacct/251207-go-pkg-cfgm)](https://goreportcard.com/report/github.com/lwmacct/251207-go-pkg-cfgm)
[![GitHub Tag](https://img.shields.io/github/v/tag/lwmacct/251207-go-pkg-cfgm?sort=semver)](https://github.com/lwmacct/251207-go-pkg-cfgm/tags)

通用的 Go 配置加载库，支持泛型，可被外部项目复用。

<!--TOC-->

## Table of Contents

- [特性](#特性) `:31+10`
- [安装](#安装) `:41+6`
- [快速开始](#快速开始) `:47+252`
  - [1. 定义配置结构体](#1-定义配置结构体) `:49+36`
  - [2. 加载配置](#2-加载配置) `:85+112`
  - [3. 环境变量](#3-环境变量) `:197+24`
  - [4. 测试驱动的配置管理](#4-测试驱动的配置管理) `:221+78`
- [模板语法](#模板语法) `:299+44`
  - [基本语法](#基本语法) `:307+16`
  - [语义说明](#语义说明) `:323+7`
  - [使用示例](#使用示例) `:330+13`
- [License](#license) `:343+3`

<!--TOC-->

## 特性

- **泛型支持**：适用于任意配置结构体
- **多源合并**：默认值 → 配置文件 → 环境变量 → CLI flags（优先级递增）
- **函数选项模式**：灵活配置，向后兼容
- **环境变量支持**：前缀匹配，适合 Docker/K8s 容器化部署
- **自动映射**：CLI flag 名称自动从配置路径推导，递归移除命令链前缀并保留 `.` 层级
- **示例生成**：自动根据结构体生成带注释的 YAML 配置示例
- **模板展开**：支持环境变量引用、默认值与多级 fallback（Shell 参数展开）

## 安装

```bash
go get github.com/lwmacct/251207-go-pkg-cfgm/pkg/cfgm
```

## 快速开始

### 1. 定义配置结构体

```go
// internal/config/config.go
package config

import (
    "time"
    "github.com/lwmacct/251207-go-pkg-cfgm/pkg/cfgm"
)

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

func Load(opts ...cfgm.Option) (*Config, error) {
    return cfgm.Load(DefaultConfig(), opts...)
}
```

说明：YAML/JSON 都以 `json` tag 作为配置 key。

### 2. 加载配置

```go
// 使用默认值 + 默认配置文件路径 (config.yaml, config/config.yaml)
cfg, err := cfgm.Load(DefaultConfig())

// 使用应用专属配置文件路径 (.app.yaml, ~/.app.yaml, /etc/app/config.yaml 等)
cfg, err := cfgm.Load(DefaultConfig(),
    cfgm.WithAppName("app"),
)

// 默认启用环境变量（前缀 APP_）
cfg, err := cfgm.Load(DefaultConfig())

// 自定义环境变量前缀
cfg, err := cfgm.Load(DefaultConfig(),
    cfgm.WithEnvPrefix("APP_"),
)

// 完整示例：配置文件 + 环境变量 + CLI flags
cfg, err := cfgm.Load(DefaultConfig(),
    cfgm.WithConfigPaths("config.yaml", "/etc/app/config.yaml"),
    cfgm.WithEnvPrefix("APP_"),
    cfgm.WithCommand(cmd),
)

// CLI 场景推荐使用 LoadCmd，并在根命令挂载 cfgm.ConfigFlag()
app := &cli.Command{
    Name:  "app",
    Flags: []cli.Flag{cfgm.ConfigFlag()},
    Commands: []*cli.Command{
        {
            Name: "server",
            Action: func(ctx context.Context, cmd *cli.Command) error {
                cfg, err := cfgm.LoadCmd(cmd, DefaultConfig(), "app")
                if err != nil {
                    return err
                }
                _ = cfg
                return nil
            },
        },
    },
}
```

指定配置文件路径：

```bash
app --config ./config.yaml server
app -c ./config.yaml server
```

`--config` 指定的路径会作为唯一配置文件搜索路径；未指定时使用 `WithAppName` / 默认路径规则。

CLI flag 名称会从配置路径自动推导。cfgm 会根据当前 `cli.Command` 的命令链递归剥离匹配的配置前缀：

| 命令链           | 配置 key                | 可声明的 flag      |
| ---------------- | ----------------------- | ------------------ |
| 无               | `server.addr`           | `--server.addr`    |
| `server`         | `server.addr`           | `--addr`           |
| `client`         | `client.server.hostkey` | `--server.hostkey` |
| `server service` | `server.service.port`   | `--port`           |

完整配置路径仍是 fallback 候选，例如 `server` 命令下也可以声明 `--redis.url`。当 fallback 候选与命令链剥离后的候选重名时，剥离更深的候选优先；同一优先级仍然重复时会返回错误，避免静默写入错误字段。

CLI help 文案可以从配置字段的 `desc` tag 获取，与配置示例文件保持一致：

```go
defaults := DefaultConfig()
usage := cfgm.Schema(defaults).Command("client")

flag := &cli.StringFlag{
    Name:  "url",
    Value: defaults.Client.URL,
    Usage: usage.MustUsage("url"),
}
```

`Schema(...).Command(...)` 使用同一套 flag 名称映射规则，所以 `client` 命令下的 `url` 会解析到 `client.url`，共享配置仍可使用完整路径，例如 `redis.url`。

命令也可以包含不属于配置的业务 flags。默认情况下，cfgm 会严格校验每个非框架 flag 都能映射到配置字段，以便发现拼错的配置 flag；对于只控制本次操作的 flags，可显式忽略：

```go
cfg, err := cfgm.LoadCmd(
    cmd,
    DefaultConfig(),
    "app",
    cfgm.WithIgnoredCLIFlags("host", "dry-run", "format"),
)
```

这类 flag 适合表达一次性操作参数，例如 `--host`、`--dry-run`、`--force`，不需要污染配置结构体。

如果希望在测试中确保配置字段都有对应 CLI flag，可使用覆盖率校验：

```go
func TestClientCommandCoversConfigFlags(t *testing.T) {
    cfgm.AssertCommandFlagCoverage(
        t,
        clientCommand,
        DefaultConfig(),
        []string{"client", "redis"},
        cfgm.IgnoreConfigKeys("redis.password"),
    )
}
```

这适合防止新增配置字段后忘记补 CLI flag；敏感字段可通过 `IgnoreConfigKeys` 排除。

注意：`IgnoreConfigKeys` 只用于覆盖率校验；运行时忽略非配置 CLI flag 请使用 `WithIgnoredCLIFlags`。

### 3. 环境变量

#### 前缀匹配（默认 `APP_`，可用 WithEnvPrefix 覆盖）

| 环境变量                   | 配置 key               |
| -------------------------- | ---------------------- |
| `APP_SERVER_ADDR`          | `server.addr`          |
| `APP_SERVER_TIMEOUT`       | `server.timeout`       |
| `APP_DEBUG`                | `debug`                |
| `APP_CLIENT_REV_AUTH_USER` | `client.rev-auth-user` |

转换规则：

1. 移除前缀（如 `APP_`）
2. 点号 `.` 与连字符 `-` 转为下划线 `_`
3. 转为大写

**注意**：

- 默认前缀是 `APP_`
- 可通过 `cfgm.WithEnvPrefix("APP_")` 覆盖默认前缀
- 可通过 `cfgm.WithEnvPrefix("")` 显式禁用环境变量前缀绑定
- 通过反射自动生成配置 key 绑定，只匹配结构体中声明的 key（包括包含连字符的 key）

### 4. 测试驱动的配置管理

本库提供 `ConfigFiles` 测试辅助工具，通过单元测试维护两类文件：

- `config/config.example.yaml`：示例配置文件，由测试生成，适合提交到仓库
- `config/config.yaml`：本地运行配置文件，由用户显式初始化或复制示例文件得到，通常不提交

创建测试文件 `internal/config/config_test.go`：

```go
package config

import (
    "testing"
    "github.com/lwmacct/251207-go-pkg-cfgm/pkg/cfgm"
)

// 定义一次，复用多处
var files = cfgm.ConfigFiles[Config]{
    Defaults:    DefaultConfig,
    ExampleFile: "config/config.example.yaml",
    RuntimeFile: "config/config.yaml",
}

func TestWriteConfigExample(t *testing.T) { files.WriteExample(t) }
func TestRuntimeConfigKeysValid(t *testing.T) { files.ValidateRuntimeConfig(t) }
```

路径为相对路径，相对于 `go.mod` 所在目录。

#### 生成配置示例（TestWriteConfigExample）

根据 `DefaultConfig()` 结构体自动生成带注释的示例文件，输出到 `ExampleFile`：

```bash
go test -v -run TestWriteConfigExample ./internal/config/...
```

生成的示例文件：

```yaml
# 默认配置示例文件, 此文件由单元测试生成, 请勿直接修改
# 复制此文件为 config.yaml 并根据需要修改

# 服务端配置
server:
  addr: ":8080" # 监听地址
  timeout: 30s # 超时时间
```

**工作原理**：通过反射读取结构体的 `json` 和 `desc` tag，自动生成完整的 YAML 示例。

#### 校验运行配置（TestRuntimeConfigKeysValid）

验证 `RuntimeFile` 中的所有配置项都在 `ExampleFile` 中定义：

```bash
go test -v -run TestRuntimeConfigKeysValid ./internal/config/...
```

**用途**：

- 防止配置项拼写错误（如 `servr.addr` 写成 `server.addr`）
- 检测已废弃的配置项
- CI 集成，确保配置文件与代码同步

如果存在无效配置项，测试将失败并列出所有问题项。如果运行配置文件不存在，测试会自动跳过。

#### 初始化本地运行配置

`ConfigFiles.WriteExample` 只生成示例文件，不会写入或覆盖 `config/config.yaml`。如需生成本地运行配置，可在应用初始化命令中显式调用：

```go
err := cfgm.InitConfigFile(DefaultConfig(), "config/config.yaml")
```

如果目标文件已存在，`InitConfigFile` 会返回错误并拒绝覆盖。

## 模板语法

本库提供 `templexp` 包用于模板展开，采用 Shell 参数展开语法。

```bash
go get github.com/lwmacct/251207-go-pkg-cfgm/pkg/templexp
```

### 基本语法

| 语法              | 说明                     | 示例                 |
| ----------------- | ------------------------ | -------------------- |
| `${VAR}`          | 参数展开                 | `${HOME}`            |
| `${VAR:-default}` | fallback（未设置或空）   | `${PORT:-8080}`      |
| `${VAR-default}`  | fallback（仅未设置）     | `${PORT-8080}`       |
| `${VAR:+alt}`     | 替代值（已设置且非空）   | `${DEBUG:+1}`        |
| `${VAR+alt}`      | 替代值（已设置即可）     | `${FLAG+1}`          |
| `${VAR:?msg}`     | 必填（未设置或空时报错） | `${TOKEN:?required}` |
| `${VAR?msg}`      | 必填（未设置时报错）     | `${TOKEN?required}`  |
| `${VAR:=default}` | 赋值（未设置或空时赋值） | `${REGION:=cn}`      |
| `${VAR=default}`  | 赋值（未设置时赋值）     | `${REGION=cn}`       |

支持使用 `$$` 输出字面 `$`。

### 语义说明

- 仅识别 `${...}`，不解析 `$VAR` 形式
- 支持嵌套展开：`${A:-${B:-default}}`
- `:=` / `=` 赋值仅作用于当前展开过程，不会写回进程环境
- 无法识别的 `${...}` 会原样保留

### 使用示例

```go
import "github.com/lwmacct/251207-go-pkg-cfgm/pkg/templexp"

// 展开模板
result, err := templexp.ExpandTemplate(`{
  "host": "${DB_HOST:-localhost}",
  "port": "${DB_PORT:-5432}",
  "key": "${PRIMARY_KEY:-${BACKUP_KEY:-sk-default}}"
}`)
```

## License

MIT
