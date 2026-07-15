# cfgm examples

所有命令均从仓库根目录执行。每个目录只展示一个主要能力，便于复制到实际项目。

| 示例 | 主题 |
| --- | --- |
| [`basic`](basic) | 默认值、配置文件和环境变量 |
| [`cli`](cli) | 自动装配 CLI flags、alias、隐藏字段和 typed action |
| [`precedence`](precedence) | defaults → file → env → CLI 优先级和 `LoadReport` |
| [`composite`](composite) | 标量 slice、`[]struct`、JSON object flags 和显式清空 |
| [`codec`](codec) | 文件、环境变量和 CLI 共用自定义 codec |
| [`validation`](validation) | 严格未知字段校验、`AllowUnknownKeys` 和已知字段类型校验 |
| [`templates`](templates) | `${...}`、`Raw`、`ExpandTemplates` 和全局模板策略 |
| [`config-files`](config-files) | 生成 example YAML 并用同一 `Manager` 校验运行配置 |
| [`custom-source`](custom-source) | 实现自定义 `Source` 并读取 `Schema` |

## Basic

配置文件先覆盖默认值，随后 `APP_` 环境变量覆盖配置文件：

```bash
APP_ENDPOINT=https://localhost:8443 APP_TIMEOUT=5s \
  go run ./examples/basic
```

## CLI

`Manager.MustConfigure()` 遍历命令树，生成根 `--config/-c`、`--env-prefix/-e` 和各命令的配置 flags：

```bash
go run ./examples/cli server \
  --addr=:9090 \
  --timeout=10s \
  --redis.url=redis://localhost:6379/1
```

`server.redis.password` 通过 `HideCLI` 排除，不会成为命令行参数。

## Precedence

此命令同时设置 file、env 和 CLI；最终 `addr` 来自 CLI，`timeout` 来自 file：

```bash
PRECEDENCE_SERVER_ADDR=:9000 \
  go run ./examples/precedence \
  --config examples/precedence/config.yaml \
  server --addr=:10000
```

程序会打印最终值，以及 `LoadReport` 记录的每个来源和 key。

## Composite

重复的 JSON object flag 构成一个新的证书列表，并整体替换文件中的列表：

```bash
go run ./examples/composite \
  --config examples/composite/config.yaml \
  server \
  --tags=api --tags=edge \
  --certificates='{"id":"main","certificate":"file:///main.crt","private-key":"file:///main.key"}' \
  --certificates='{"id":"api","certificate":"file:///api.crt","private-key":"file:///api.key"}'
```

使用 `[]` 显式清空证书：

```bash
go run ./examples/composite \
  --config examples/composite/config.yaml \
  server --certificates=[]
```

环境变量中的 slice 必须是完整 JSON：

```bash
COMPOSITE_SERVER_TAGS='["env","json"]' \
  go run ./examples/composite server
```

## Codec

`endpoint` 是自定义 struct，但文件、环境变量和 CLI 都将其视作一个字符串叶子值：

```bash
go run ./examples/codec --config examples/codec/config.yaml
CODEC_ENDPOINT=svc://env go run ./examples/codec
go run ./examples/codec --endpoint=svc://cli
```

`http://invalid` 会被 codec 拒绝。

## Validation

示例依次展示严格未知字段错误、允许额外字段，以及即使允许额外字段仍会执行的已知字段类型校验：

```bash
go run ./examples/validation
```

## Templates

配置文件使用 Redis URL/password 展示 `${VAR}` 和 `${VAR:-fallback}`：

```bash
REDIS_URL=redis://localhost:6379/1 REDISCLI_AUTH=secret \
  go run ./examples/templates
```

程序同时加载三次配置，分别展示默认展开、`Raw()` 保留原文，以及 `WithoutTemplateExpansion()` 配合 `ExpandTemplates()` 的局部强制展开。

## Config Files

运行配置：

```bash
go run ./examples/config-files
```

生成 [`config.example.yaml`](config-files/config.example.yaml) 并校验 [`config.yaml`](config-files/config.yaml)：

```bash
go test -v ./examples/config-files
```

生成与校验使用同一个 `Manager`，不会产生第二套 Schema。

## Custom Source

内存 Source 使用 `Schema.Has` 检查目标配置，并通过 `LoadReport` 暴露来源信息：

```bash
go run ./examples/custom-source
```
