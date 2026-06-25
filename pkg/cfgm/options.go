package cfgm

import "github.com/urfave/cli/v3"

const configFlagName = "config"
const envPrefixFlagName = "env-prefix"

// options 配置加载选项。
type options struct {
	appName             string // 应用名称，用于生成默认配置路径
	cmd                 *cli.Command
	configPaths         []string
	baseDir             string // 路径基准目录，用于将相对路径转换为绝对路径
	baseDirSet          bool   // 是否显式设置了 baseDir（区分空字符串和未设置）
	envPrefix           string
	envPrefixSet        bool
	ignoredCLIFlags     map[string]bool
	noTemplateExpansion bool // 是否禁用配置模板展开（默认启用）
	callerSkip          int  // FindProjectRoot 的调用栈跳过层数（0 表示使用默认值）
}

// Option 配置加载选项函数。
type Option func(*options)

// ConfigFlag 返回 cfgm 识别的配置文件路径 CLI flag。
//
// 将该 flag 挂到根命令或子命令后，[LoadCmd] / [MustLoadCmd] 会自动读取
// --config 指定的路径，并使用它作为唯一配置文件搜索路径。
// 该 flag 不会映射到配置结构体字段。
func ConfigFlag() cli.Flag {
	return &cli.StringFlag{
		Name:    configFlagName,
		Aliases: []string{"c"},
		Usage:   "配置文件路径",
	}
}

// EnvPrefixFlag 返回 cfgm 识别的环境变量前缀 CLI flag。
//
// 将该 flag 挂到根命令或子命令后，[LoadCmd] / [MustLoadCmd] 会自动读取
// --env-prefix / -e 指定的前缀，并使用它覆盖 [WithEnvPrefix] 设置的值。
// 该 flag 不会映射到配置结构体字段。
//
// 默认值为 "APP_"；显式设置空字符串可禁用环境变量绑定。
func EnvPrefixFlag() cli.Flag {
	return &cli.StringFlag{
		Name:    envPrefixFlagName,
		Aliases: []string{"e"},
		Value:   "APP_",
		Usage:   "环境变量前缀（默认：APP_；空字符串禁用环境变量绑定）",
	}
}

// WithCommand 绑定 CLI 命令，读取显式设置的 flags 以覆盖配置（最高优先级）。
func WithCommand(cmd *cli.Command) Option {
	return func(o *options) {
		o.cmd = cmd
	}
}

// WithIgnoredCLIFlags 声明不映射到配置结构体的 CLI flags。
//
// Load / LoadCmd 默认会校验命令上的每个非框架 flag 都能映射到配置字段，
// 以便尽早发现拼错的配置 flag。对于只控制本次操作、不属于持久配置的
// 业务 flag（例如 --dry-run、--force、--format），可通过该选项显式跳过。
func WithIgnoredCLIFlags(names ...string) Option {
	return func(o *options) {
		if o.ignoredCLIFlags == nil {
			o.ignoredCLIFlags = make(map[string]bool, len(names))
		}
		for _, name := range names {
			if name != "" {
				o.ignoredCLIFlags[name] = true
			}
		}
	}
}

// WithAppName 设置应用名称，用于生成默认搜索路径（见 [DefaultPaths]）。
//
// 示例：
//
//	cfgm.Load(defaultConfig,
//	    cfgm.WithAppName("myapp"),  // 自动搜索 .myapp.yaml 等
//	    cfgm.WithCommand(cmd),
//	)
func WithAppName(name string) Option {
	return func(o *options) {
		o.appName = name
	}
}

// WithConfigPaths 设置配置文件搜索路径。
//
// 按顺序查找，命中首个文件即停止；相对路径会基于 [WithBaseDir] 解析。
func WithConfigPaths(paths ...string) Option {
	return func(o *options) {
		o.configPaths = paths
	}
}

// WithBaseDir 设置配置路径的解析基准。
//
// 默认基准为项目根目录（go.mod 所在目录）；空字符串表示当前工作目录。
// 注意：绝对路径不受影响。
func WithBaseDir(path string) Option {
	return func(o *options) {
		o.baseDir = path
		o.baseDirSet = true
	}
}

// WithCallerSkip 设置 [FindProjectRoot] 的调用栈跳过层数。
//
// 当 [Load] 被多层封装时，用于修正项目根目录定位。
// 若已通过 [WithBaseDir] 指定基准目录，则该选项不会生效。
//
// 示例：
//
//	// 在封装函数中使用
//	func LoadMyConfig() (*Config, error) {
//	    return cfgm.Load(DefaultConfig(),
//	        cfgm.WithCallerSkip(2),  // 跳过: load → Load → LoadMyConfig
//	    )
//	}
//
// 注意：
//   - 默认值根据调用函数自动确定（Load/LoadCmd: 1, MustLoad/MustLoadCmd: 2）
//   - 每增加一层封装，skip 值需要相应增大
//   - 如果使用 [WithBaseDir] 显式设置了基准目录，此选项无效
func WithCallerSkip(skip int) Option {
	return func(o *options) {
		o.callerSkip = skip
	}
}

// WithEnvPrefix 设置环境变量前缀。
//
// 环境变量命名规则：
//   - 前缀 + 大写的配置 key
//   - 点号 (.) 和连字符 (-) 转为下划线 (_)
//
// 默认前缀为 "APP_"；显式调用后以传入值为准。
// 传入空字符串可禁用默认的前缀绑定。
//
// 示例 (前缀为 "MYAPP_")：
//   - MYAPP_DEBUG → debug
//   - MYAPP_SERVER_URL → server.url
//   - MYAPP_CLIENT_REV_AUTH_USER → client.rev-auth-user
//
// 注意：通过反射自动生成配置 key 的绑定，只匹配结构体中定义的 key。
func WithEnvPrefix(prefix string) Option {
	return func(o *options) {
		o.envPrefix = prefix
		o.envPrefixSet = true
	}
}

// WithoutTemplateExpansion 禁用配置模板展开。
//
// 默认会对默认值与配置文件执行 Shell 参数展开（如 ${VAR:-default}）。
// 该选项会保留原始 ${...} 字符串。
func WithoutTemplateExpansion() Option {
	return func(o *options) {
		o.noTemplateExpansion = true
	}
}
