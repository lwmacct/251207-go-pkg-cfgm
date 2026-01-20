package cfgm

import "github.com/urfave/cli/v3"

// options 配置加载选项。
type options struct {
	appName             string // 应用名称，用于生成默认配置路径
	cmd                 *cli.Command
	configPaths         []string
	baseDir             string // 路径基准目录，用于将相对路径转换为绝对路径
	baseDirSet          bool   // 是否显式设置了 baseDir（区分空字符串和未设置）
	envPrefix           string
	noTemplateExpansion bool // 是否禁用配置文件模板展开（默认启用）
	callerSkip          int  // FindProjectRoot 的调用栈跳过层数（0 表示使用默认值）
}

// Option 配置加载选项函数。
type Option func(*options)

// WithCommand 绑定 CLI 命令，读取显式设置的 flags 以覆盖配置（最高优先级）。
func WithCommand(cmd *cli.Command) Option {
	return func(o *options) {
		o.cmd = cmd
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

// WithEnvPrefix 启用环境变量前缀解析。
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
// 注意：通过反射自动生成配置 key 的绑定，只匹配结构体中定义的 key。
func WithEnvPrefix(prefix string) Option {
	return func(o *options) {
		o.envPrefix = prefix
	}
}

// WithoutTemplateExpansion 禁用配置文件的模板展开。
//
// 默认会执行 Shell 参数展开（如 ${VAR:-default}）。
// 该选项会保留原始 ${...} 字符串。
func WithoutTemplateExpansion() Option {
	return func(o *options) {
		o.noTemplateExpansion = true
	}
}
