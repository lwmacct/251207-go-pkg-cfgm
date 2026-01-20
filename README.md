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
- [快速开始](#快速开始) `:47+144`
  - [1. 定义配置结构体](#1-定义配置结构体) `:49+36`
  - [2. 加载配置](#2-加载配置) `:85+24`
  - [3. 环境变量](#3-环境变量) `:109+19`
  - [4. 测试驱动的配置管理](#4-测试驱动的配置管理) `:128+63`
- [模板语法](#模板语法) `:191+44`
  - [基本语法](#基本语法) `:199+16`
  - [语义说明](#语义说明) `:215+7`
  - [使用示例](#使用示例) `:222+13`
- [License](#license) `:235+3`

<!--TOC-->

## 特性

- **泛型支持**：适用于任意配置结构体
- **多源合并**：默认值 → 配置文件 → 环境变量 → CLI flags（优先级递增）
- **函数选项模式**：灵活配置，向后兼容
- **环境变量支持**：前缀匹配，适合 Docker/K8s 容器化部署
- **自动映射**：CLI flag 名称自动从 `json` tag 推导（仅将 `.` 转为 `-`）
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

// 使用应用专属配置文件路径 (.myapp.yaml, ~/.myapp.yaml, /etc/myapp/config.yaml 等)
cfg, err := cfgm.Load(DefaultConfig(),
    cfgm.WithAppName("myapp"),
)

// 使用环境变量（前缀 MYAPP_）
cfg, err := cfgm.Load(DefaultConfig(),
    cfgm.WithEnvPrefix("MYAPP_"),
)

// 完整示例：配置文件 + 环境变量 + CLI flags
cfg, err := cfgm.Load(DefaultConfig(),
    cfgm.WithConfigPaths("config.yaml", "/etc/myapp/config.yaml"),
    cfgm.WithEnvPrefix("MYAPP_"),
    cfgm.WithCommand(cmd),
)
```

### 3. 环境变量

#### 前缀匹配（WithEnvPrefix）

| 环境变量                     | 配置 key               |
| ---------------------------- | ---------------------- |
| `MYAPP_SERVER_ADDR`          | `server.addr`          |
| `MYAPP_SERVER_TIMEOUT`       | `server.timeout`       |
| `MYAPP_DEBUG`                | `debug`                |
| `MYAPP_CLIENT_REV_AUTH_USER` | `client.rev-auth-user` |

转换规则：

1. 移除前缀（如 `MYAPP_`）
2. 点号 `.` 与连字符 `-` 转为下划线 `_`
3. 转为大写

**注意**：通过反射自动生成配置 key 绑定，只匹配结构体中声明的 key（包括包含连字符的 key）。

### 4. 测试驱动的配置管理

本库提供 `ConfigTestHelper` 测试辅助工具，通过单元测试实现配置示例生成和配置校验。

创建测试文件 `internal/config/config_test.go`：

```go
package cfgm

import (
    "testing"
    "github.com/lwmacct/251207-go-pkg-cfgm/pkg/cfgm"
)

// 定义一次，复用多处
var helper = cfgm.ConfigTestHelper[Config]{
    ExamplePath: "config/config.example.yaml",
    ConfigPath:  "config/config.yaml",
}

func TestWriteExample(t *testing.T) { helper.WriteExampleFile(t, DefaultConfig()) }
func TestConfigKeysValid(t *testing.T) { helper.ValidateKeys(t) }
```

路径为相对路径，相对于 `go.mod` 所在目录。

#### 生成配置示例（TestWriteExample）

根据 `DefaultConfig()` 结构体自动生成带注释的示例文件：

```bash
go test -v -run TestWriteExample ./internal/config/...
```

生成的示例文件：

```yaml
# 配置示例文件, 复制此文件为 config.yaml 并根据需要修改

# 服务端配置
server:
  addr: ":8080" # 监听地址
  timeout: 30s # 超时时间
```

**工作原理**：通过反射读取结构体的 `json` 和 `desc` tag，自动生成完整的 YAML 示例。

#### 校验配置文件（TestConfigKeysValid）

验证配置文件中的所有配置项都在示例文件中定义：

```bash
go test -v -run TestConfigKeysValid ./internal/config/...
```

**用途**：

- 防止配置项拼写错误（如 `servr.addr` 写成 `server.addr`）
- 检测已废弃的配置项
- CI 集成，确保配置文件与代码同步

如果存在无效配置项，测试将失败并列出所有问题项。如果配置文件不存在，测试会自动跳过。

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
