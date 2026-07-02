// Package cfgm 提供通用的配置加载功能。
//
// 支持 YAML/JSON，配置 key 使用 json tag 统一描述，YAML 与 JSON 共享同一套 key。
// 推荐使用显式 source pipeline：默认值作为最低优先级，随后按 Add 的顺序合并
// File、Env、CLI 等来源，后添加的 source 覆盖先添加的 source。
//
// # 加载模型
//
//  1. 默认值 - 通过 defaultConfig 参数传入
//  2. 显式 sources - 通过 [Loader.Add] 声明，声明顺序就是优先级
//  3. 解码 - 将合并后的 map 解码回配置结构体
//
// 新 Loader 默认会校验 source 中的未知 key，避免配置文件字段拼写错误被静默忽略。
// 如需保留宽松行为，可使用 [Loader.AllowUnknownKeys]。
//
// # 快速开始
//
// 定义配置结构体（json + desc 标签）：
//
//	type Config struct {
//	    Name    string        `json:"name"    desc:"应用名称"`
//	    Debug   bool          `json:"debug"   desc:"调试模式"`
//	    Timeout time.Duration `json:"timeout" desc:"超时时间"`
//	}
//
// 使用显式 source pipeline：
//
//	cfg, report, err := cfgm.New(Config{
//	    Name:    "default",
//	    Debug:   false,
//	    Timeout: 30 * time.Second,
//	}).
//	    Add(
//	        cfgm.File("config/config.yaml", cfgm.Optional(), cfgm.ExpandTemplates()),
//	        cfgm.Env("APP_"),
//	        cfgm.CLI(cmd),
//	    ).
//	    Load(ctx)
//
// report 会记录每个 source 贡献的配置 key，便于排查最终值来源。
//
// # 配置文件路径
//
// [File] 加载单个配置文件，默认 required；使用 [Optional] 可允许文件不存在。
// [Files] 会按顺序读取首个存在的文件。
//
// CLI 场景可将 [ConfigFlag] 挂到根命令或子命令：
//
//	cmd := &cli.Command{
//	    Flags: []cli.Flag{cfgm.ConfigFlag()},
//	}
//
// 应用可自行读取命令链上的 --config / -c，并传给 [File]：
//
//	app --config ./config.yaml server
//
// # 环境变量(前缀)
//
// [Env] 根据配置 schema 自动生成环境变量绑定。
//
// 环境变量命名规则：
//   - 前缀 + 大写的配置 key
//   - 点号 (.) 和连字符 (-) 转为下划线 (_)
//
// 示例：
//   - APP_DEBUG → debug
//   - APP_SERVER_URL → server.url
//   - APP_CLIENT_REV_AUTH_USER → client.rev-auth-user
//
// # 模板展开
//
// [ExpandTemplates] 可对配置文件执行 Shell 参数展开。
// [Loader.ExpandDefaults] 可对 defaultConfig 中的字符串执行相同展开。
//
// 支持 Shell 参数展开：
//   - 仅识别 ${...}（不解析 $VAR）
//   - ${VAR} / ${VAR:-default} / ${VAR?msg} / ${VAR:=default}
//   - 支持嵌套与 "$$" 字面量
//
// 示例：
//
//	# config.yaml
//	api_key: "${OPENAI_API_KEY}"
//	model: "${LLM_MODEL:-gpt-4}"
//	base_url: "${PROD_URL:-${DEV_URL:-http://localhost:8080}}"
//
// # CLI Flag 映射
//
// CLI flag 默认使用配置路径，并递归移除与命令链匹配的作用域前缀：
//   - `server` 命令下，`server.addr` → `--addr`
//   - `client` 命令下，`client.server.addr` → `--server.addr`
//   - `server service` 命令链下，`server.service.port` → `--port`
//   - 无命令作用域时，`server.addr` → `--server.addr`
//   - 完整路径作为 fallback 候选，如 `server` 命令下仍可声明 `--redis.url`
//
// 当完整路径候选与命令链剥离后的候选重名时，剥离更深的候选优先。
// 若同一优先级下生成的 flag 名重复，Load / LoadCmd 会返回错误，而不是静默忽略。
//
// 使用 [Schema] 可在命令定义阶段复用同一套映射规则，从配置字段的 desc tag
// 获取 CLI flag Usage：
//
//	defaults := DefaultConfig()
//	usage := cfgm.Schema(defaults).Command("client")
//	flag := &cli.StringFlag{
//	    Name:  "url",
//	    Value: defaults.Client.URL,
//	    Usage: usage.MustUsage("url"),
//	}
//
// 使用 [AssertCommandFlagCoverage] 可在测试中确保命令覆盖指定配置前缀：
//
//	cfgm.AssertCommandFlagCoverage(t, clientCommand, DefaultConfig(),
//	    []string{"client", "redis"},
//	    cfgm.IgnoreConfigKeys("redis.password"),
//	)
//
// 这适合在配置结构新增字段时防止遗漏对应 CLI flag。
//
// # 生成配置示例
//
// 使用 [ExampleYAML] 生成带注释的 YAML：
//
//	yaml := cfgm.ExampleYAML(defaultConfig)
//	os.WriteFile("config.example.yaml", yaml, 0644)
//
// 使用 [MarshalJSON] 输出 JSON：
//
//	jsonBytes := cfgm.MarshalJSON(defaultConfig)
//	os.WriteFile("config.json", jsonBytes, 0644)
//
// # 测试辅助
//
// [ConfigFiles] 可用于在测试中生成示例文件并校验运行配置：
//
//	var files = cfgm.ConfigFiles[Config]{
//	    Defaults:    DefaultConfig,
//	    ExampleFile: "config/config.example.yaml",
//	    RuntimeFile: "config/config.yaml",
//	}
//
//	func TestWriteConfigExample(t *testing.T) { files.WriteExample(t) }
//	func TestRuntimeConfigKeysValid(t *testing.T) { files.ValidateRuntimeConfig(t) }
//
// 使用 [InitConfigFile] 可显式初始化本地运行配置文件：
//
//	err := cfgm.InitConfigFile(DefaultConfig(), "config/config.yaml")
package cfgm
