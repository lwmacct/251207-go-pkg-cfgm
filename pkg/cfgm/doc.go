// Package cfgm 提供通用的配置加载功能。
//
// 支持 YAML/JSON，按默认值、配置文件、环境变量与 CLI flags 逐层覆盖。
// 配置 key 使用 json tag 统一描述，YAML 与 JSON 共享同一套 key。
//
// # 加载优先级 (从低到高)
//
//  1. 默认值 - 通过 defaultConfig 参数传入
//  2. 配置文件 - 通过 [WithConfigPaths] 或 [WithAppName] 设置
//  3. 环境变量(前缀) - 通过 [WithEnvPrefix] 自动生成绑定
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
//	cfg, err := cfgm.LoadCmd(cmd, DefaultConfig(), "myapp",
//	    cfgm.WithEnvPrefix("MYAPP_"),
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
// # 环境变量(前缀)
//
// 通过 [WithEnvPrefix] 启用环境变量支持：
//   - 前缀 + 大写的配置 key
//   - 点号 (.) 和连字符 (-) 转为下划线 (_)
//
// 示例 (前缀为 "MYAPP_")：
//   - MYAPP_DEBUG → debug
//   - MYAPP_SERVER_URL → server.url
//   - MYAPP_CLIENT_REV_AUTH_USER → client.rev-auth-user
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
// 仅替换 "." 为 "-"：
//   - server.url → --server-url
//   - tls.skip_verify → --tls-skip_verify
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
// [ConfigTestHelper] 可用于校验配置项与示例文件的一致性：
//
//	var helper = cfgm.ConfigTestHelper[Config]{
//	    ExamplePath: "config/config.example.yaml",
//	    ConfigPath:  "config/config.yaml",
//	}
//
//	func TestWriteExample(t *testing.T) { helper.WriteExampleFile(t, DefaultConfig()) }
//	func TestConfigKeysValid(t *testing.T) { helper.ValidateKeys(t) }
package cfgm
