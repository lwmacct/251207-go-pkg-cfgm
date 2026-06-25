// Package cfgm 提供通用的配置加载功能。
//
// 支持 YAML/JSON，按默认值、配置文件、环境变量与 CLI flags 逐层覆盖。
// 配置 key 使用 json tag 统一描述，YAML 与 JSON 共享同一套 key。
//
// # 加载优先级 (从低到高)
//
//  1. 默认值 - 通过 defaultConfig 参数传入
//  2. 配置文件 - 通过 [WithConfigPaths] 或 [WithAppName] 设置
//  3. 环境变量(前缀) - 默认使用 APP_，可通过 [WithEnvPrefix] 覆盖或禁用
//  4. CLI flags - 通过 [WithCommand] 选项设置，最高优先级
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
// 推荐使用 LoadCmd：
//
//	app := &cli.Command{
//	    Name:  "myapp",
//	    Flags: []cli.Flag{
//	        cfgm.ConfigFlag(),  // 支持 --config / -c
//	        cfgm.EnvPrefixFlag(), // 支持 --env-prefix / -e
//	    },
//	}
//
//	cfg, err := cfgm.LoadCmd(cmd, DefaultConfig(), "myapp",
//	    cfgm.WithEnvPrefix("MYAPP_"), // 可选；会被 CLI flag 覆盖
//	)
//
// 或使用 Load 组合选项：
//
//	cfg, err := cfgm.Load(Config{
//	    Name:    "default",
//	    Debug:   false,
//	    Timeout: 30 * time.Second,
//	},
//	    cfgm.WithAppName("myapp"),
//	    cfgm.WithEnvPrefix("MYAPP_"),
//	    cfgm.WithCommand(cmd),
//	)
//
// # 配置文件路径
//
// [WithAppName] 会生成默认搜索路径（见 [DefaultPaths]）：
//   - .myapp.yaml (当前目录)
//   - ~/.myapp.yaml (用户主目录)
//   - /etc/myapp/config.yaml (系统配置)
//   - config.yaml, config/config.yaml (通用路径)
//
// 如需自定义路径，使用 [WithConfigPaths]：
//
//	cfgm.Load(config,
//	    cfgm.WithAppName("myapp"),          // 仍可用于其他用途
//	    cfgm.WithConfigPaths("custom.yaml"), // 覆盖默认路径
//	)
//
// CLI 场景可将 [ConfigFlag] 挂到根命令或子命令：
//
//	cmd := &cli.Command{
//	    Flags: []cli.Flag{cfgm.ConfigFlag()},
//	}
//
// [LoadCmd] / [MustLoadCmd] 会自动读取命令链上的 --config / -c：
//
//	myapp --config ./config.yaml server
//
// 指定后，该路径会作为唯一配置文件搜索路径；未指定时使用 [WithAppName] / 默认路径规则。
//
// # 环境变量(前缀)
//
// 默认会使用 "APP_" 前缀生成环境变量绑定。
// 调用 [WithEnvPrefix] 可覆盖默认前缀；传入空字符串可禁用该行为。
//
// 环境变量命名规则：
//   - 前缀 + 大写的配置 key
//   - 点号 (.) 和连字符 (-) 转为下划线 (_)
//
// 示例 (前缀为 "MYAPP_")：
//   - MYAPP_DEBUG → debug
//   - MYAPP_SERVER_URL → server.url
//   - MYAPP_CLIENT_REV_AUTH_USER → client.rev-auth-user
//
// # 环境变量前缀 CLI flag
//
// [EnvPrefixFlag] 提供全局 CLI flag --env-prefix / -e，用于在运行时覆盖环境变量前缀。
// 优先级：CLI flag > [WithEnvPrefix] 选项 > 默认值 "APP_"。
//
// 示例：
//
//	myapp --env-prefix CUSTOM_        // 使用 CUSTOM_ 前缀
//	myapp --env-prefix ""             // 禁用环境变量绑定
//	myapp -e PROD_                    // 使用短别名
//
// CLI 场景可将 [EnvPrefixFlag] 挂到根命令或子命令：
//
//	cmd := &cli.Command{
//	    Flags: []cli.Flag{cfgm.EnvPrefixFlag()},
//	}
//
// [LoadCmd] / [MustLoadCmd] 会自动读取命令链上的 --env-prefix / -e。
//
// # 模板展开
//
// 默认值与配置文件都会进行字符串展开（YAML/JSON 均支持）。
// 使用 [WithoutTemplateExpansion] 可禁用该行为。
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
