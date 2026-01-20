package cfgm

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"github.com/urfave/cli/v3"

	"github.com/lwmacct/251207-go-pkg-cfgm/pkg/templexp"
)

// DefaultPaths 返回默认配置文件的搜索顺序。
//
// appName 可选，提供后会追加应用专属路径。
// 返回顺序即查找顺序，先命中的文件生效。
//
// 优先级 (从高到低)：
//  1. ./.appname.yaml - 当前目录应用配置
//  2. ~/.appname.yaml - 用户主目录配置
//  3. /etc/appname/config.yaml - 系统级配置
//  4. config.yaml - 当前目录通用配置
//  5. config/config.yaml - 子目录通用配置
func DefaultPaths(appName ...string) []string {
	var paths []string

	if len(appName) > 0 && appName[0] != "" {
		name := appName[0]
		// 当前目录应用配置 (最高优先级)
		paths = append(paths, "."+name+".yaml")
		// 用户主目录
		if home, err := os.UserHomeDir(); err == nil {
			paths = append(paths, filepath.Join(home, "."+name+".yaml"))
		}
		// 系统配置目录
		paths = append(paths, "/etc/"+name+"/config.yaml")
	}

	// 当前目录通用配置 (最低优先级)
	paths = append(paths, "config.yaml", "config/config.yaml")

	return paths
}

// Load 读取配置并按优先级合并。
//
// 优先级 (从低到高)：
//  1. 默认值 - defaultConfig
//  2. 配置文件 - [WithConfigPaths] / [WithAppName]
//  3. 环境变量(前缀) - [WithEnvPrefix]
//  4. CLI flags - [WithCommand]
//
// 配置 key 由 json tag 定义，YAML 与 JSON 共享同一套 key。
// 配置文件按顺序查找，命中首个文件即停止。
func Load[T any](defaultConfig T, opts ...Option) (*T, error) {
	return load(defaultConfig, 1, opts...)
}

// load 是内部加载实现，callerSkip 用于控制 FindProjectRoot 的跳过层数。
// 各入口函数会根据自身调用深度传入合适的 skip 值。
func load[T any](defaultConfig T, callerSkip int, opts ...Option) (*T, error) {
	// 解析选项
	options := &options{}
	for _, opt := range opts {
		opt(options)
	}

	// 如果用户显式设置了 callerSkip，则优先使用
	if options.callerSkip > 0 {
		callerSkip = options.callerSkip
	}

	// 默认使用项目根目录作为相对路径基准
	if !options.baseDirSet {
		if root, err := FindProjectRoot(callerSkip); err == nil {
			options.baseDir = root
		}
	}

	// 默认使用 DefaultPaths 作为配置文件搜索路径
	// 如果设置了 appName，使用 DefaultPaths(appName) 生成应用专属路径
	if len(options.configPaths) == 0 {
		if options.appName != "" {
			options.configPaths = DefaultPaths(options.appName)
		} else {
			options.configPaths = DefaultPaths()
		}
	}

	configMap := structToMap(defaultConfig)

	// 2️⃣ 加载配置文件 (按顺序搜索，找到第一个即停止)
	configLoaded := false
	paths := options.configPaths
	if options.baseDir != "" {
		paths = make([]string, len(options.configPaths))
		for i, p := range options.configPaths {
			if !filepath.IsAbs(p) {
				paths[i] = filepath.Join(options.baseDir, p)
			} else {
				paths[i] = p
			}
		}
	}
	for _, path := range paths {
		// 尝试读取配置文件
		content, err := os.ReadFile(path) //nolint:gosec // path is from trusted config
		if err != nil {
			continue // 文件不存在或无法读取，尝试下一个路径
		}

		// 默认启用模板展开，在解析前处理模板
		if !options.noTemplateExpansion {
			expanded, expandErr := templexp.ExpandTemplate(string(content))
			if expandErr != nil {
				return nil, fmt.Errorf("expand template in %s: %w", path, expandErr)
			}
			content = []byte(expanded)
		}

		fileMap, err := parseConfigBytes(path, content)
		if err != nil {
			return nil, fmt.Errorf("parse config file %s: %w", path, err)
		}
		mergeMaps(configMap, fileMap)

		slog.Debug("Loaded config from file", "path", path, "templateExpansion", !options.noTemplateExpansion)
		configLoaded = true

		break
	}

	if len(options.configPaths) > 0 && !configLoaded {
		slog.Debug("No config file found, using defaults")
	}

	// 3️⃣ 自动生成环境变量绑定 (基于配置结构体的 key)
	// 支持包含连字符的 key，例如 rev-auth-user
	if options.envPrefix != "" {
		autoBindings := generateEnvBindings(options.envPrefix, collectConfigKeys(defaultConfig))
		slog.Debug("Generated auto env bindings", "prefix", options.envPrefix, "count", len(autoBindings))
		for envKey, configPath := range autoBindings {
			if val := os.Getenv(envKey); val != "" {
				setByPath(configMap, configPath, val)
				slog.Debug("Loaded env binding", "env", envKey, "path", configPath)
			}
		}
	}

	// 4️⃣ 加载 CLI flags (最高优先级，仅当用户明确指定时)
	if options.cmd != nil {
		applyCLIFlagsGeneric(options.cmd, configMap, defaultConfig)
	}

	// 解析到结构体
	var cfg T
	if err := decodeConfigMap(configMap, &cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return &cfg, nil
}

// LoadCmd 是 [Load] 的便捷版本，适用于 CLI 场景。
//
// 它会注入 [WithCommand]，appName 非空时额外注入 [WithAppName]。
//
// 等价于：
//
//	cfgm.Load(defaultConfig,
//	    cfgm.WithCommand(cmd),
//	    cfgm.WithAppName(appName),  // 如果 appName 非空
//	    opts...,
//	)
//
// 示例：
//
//	// 带应用名（推荐）
//	cfg, err := cfgm.LoadCmd(cmd, DefaultConfig(), "myapp",
//	    cfgm.WithEnvPrefix("MYAPP_"),
//	)
//
//	// 不带应用名
//	cfg, err := cfgm.LoadCmd(cmd, DefaultConfig(), "")
func LoadCmd[T any](cmd *cli.Command, defaultConfig T, appName string, opts ...Option) (*T, error) {
	baseOpts := []Option{WithCommand(cmd)}
	if appName != "" {
		baseOpts = append(baseOpts, WithAppName(appName))
	}
	return load(defaultConfig, 1, append(baseOpts, opts...)...)
}

// MustLoad 调用 [Load] 并在失败时 panic，适合启动阶段。
//
// 示例：
//
//	cfg := cfgm.MustLoad(DefaultConfig(),
//	    cfgm.WithAppName("myapp"),
//	    cfgm.WithEnvPrefix("MYAPP_"),
//	)
func MustLoad[T any](defaultConfig T, opts ...Option) *T {
	cfg, err := load(defaultConfig, 2, opts...)
	if err != nil {
		panic(fmt.Sprintf("cfgm: failed to load config: %v", err))
	}

	return cfg
}

// MustLoadCmd 调用 [LoadCmd] 并在失败时 panic，适合启动阶段。
//
// 示例：
//
//	cfg := cfgm.MustLoadCmd(cmd, DefaultConfig(), "myapp",
//	    cfgm.WithEnvPrefix("MYAPP_"),
//	)
func MustLoadCmd[T any](cmd *cli.Command, defaultConfig T, appName string, opts ...Option) *T {
	baseOpts := []Option{WithCommand(cmd)}
	if appName != "" {
		baseOpts = append(baseOpts, WithAppName(appName))
	}
	cfg, err := load(defaultConfig, 2, append(baseOpts, opts...)...)
	if err != nil {
		panic(fmt.Sprintf("cfgm: failed to load config: %v", err))
	}

	return cfg
}

// collectConfigKeys 递归收集配置结构体的 key 列表。
//
// 以 json tag 为准，返回叶子路径（如 client.rev-auth-user）。
func collectConfigKeys[T any](defaultConfig T) []string {
	var keys []string
	collectConfigKeysRecursive(reflect.TypeOf(defaultConfig), "", &keys)

	return keys
}

// collectConfigKeysRecursive 递归遍历字段并拼接完整 key 路径。
func collectConfigKeysRecursive(typ reflect.Type, prefix string, keys *[]string) {
	// 处理指针类型
	if typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}

	if typ.Kind() != reflect.Struct {
		return
	}

	for i := range typ.NumField() {
		field := typ.Field(i)

		key := configTagName(field)
		if key == "" {
			continue
		}

		fullKey := key
		if prefix != "" {
			fullKey = prefix + "." + key
		}

		// 如果是嵌套结构体（非特殊类型），递归处理
		if isStructType(field.Type) {
			collectConfigKeysRecursive(field.Type, fullKey, keys)

			continue
		}

		*keys = append(*keys, fullKey)
	}
}

// generateEnvBindings 根据配置 key 生成环境变量映射。
//
// 转换规则：
//   - key 中的 "." 和 "-" 转为 "_"
//   - 转为大写
//   - 添加前缀
//
// 示例 (前缀 "APP_")：
//   - client.rev-auth-user → APP_CLIENT_REV_AUTH_USER
//   - server.idle-timeout → APP_SERVER_IDLE_TIMEOUT
func generateEnvBindings(prefix string, keys []string) map[string]string {
	bindings := make(map[string]string, len(keys))
	for _, key := range keys {
		// 将 "." 和 "-" 都转为 "_"，然后大写
		envKey := strings.ToUpper(strings.NewReplacer(".", "_", "-", "_").Replace(key))
		bindings[prefix+envKey] = key
	}

	return bindings
}

// applyCLIFlagsGeneric 将用户显式设置的 CLI flags 写入配置 map。
//
// 根据 json tag 生成 CLI flag 名称，仅替换 "." 为 "-"。
//
// 映射示例 (json tag → CLI flags)：
//   - server.url → --server-url
//   - tls.skip_verify → --tls-skip_verify
//
// 支持的类型：
//   - 基本类型: string, bool
//   - 整数类型: int, int8, int16, int32, int64
//   - 无符号整数: uint, uint8, uint16, uint32, uint64
//   - 浮点数: float32, float64
//   - 时间类型: time.Duration, time.Time
//   - 切片类型: []string, []int, []int64, []float64 等
//   - Map 类型: map[string]string
func applyCLIFlagsGeneric[T any](cmd *cli.Command, config map[string]any, defaultConfig T) {
	applyCLIFlagsRecursive(cmd, config, reflect.TypeOf(defaultConfig), "")
}

// applyCLIFlagsRecursive 递归遍历结构体字段并应用 CLI flags。
func applyCLIFlagsRecursive(cmd *cli.Command, config map[string]any, typ reflect.Type, prefix string) {
	if typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}
	if typ.Kind() != reflect.Struct {
		return
	}

	for i := range typ.NumField() {
		field := typ.Field(i)

		// 获取 json 标签作为配置 key
		key := configTagName(field)
		if key == "" {
			continue
		}

		// 构建完整的配置 key
		fullKey := key
		if prefix != "" {
			fullKey = prefix + "." + key
		}

		// 如果是嵌套结构体，递归处理
		if isStructType(field.Type) {
			applyCLIFlagsRecursive(cmd, config, field.Type, fullKey)

			continue
		}

		cliFlag := strings.ReplaceAll(fullKey, ".", "-")
		if !cmd.IsSet(cliFlag) {
			continue
		}

		// 根据字段类型获取值并设置
		setCLIFlagValue(cmd, config, fullKey, cliFlag, field.Type)
	}
}

// setCLIFlagValue 按字段类型读取 CLI 值并写入配置 map。
func setCLIFlagValue(cmd *cli.Command, config map[string]any, configPath, cliFlag string, fieldType reflect.Type) {
	// 先检查特殊类型 (time.Duration, time.Time)
	switch fieldType {
	case reflect.TypeFor[time.Duration]():
		setByPath(config, configPath, cmd.Duration(cliFlag))

		return
	case reflect.TypeFor[time.Time]():
		setByPath(config, configPath, cmd.Timestamp(cliFlag))

		return
	}

	// 处理基本类型和切片
	switch fieldType.Kind() {
	// 字符串
	case reflect.String:
		setByPath(config, configPath, cmd.String(cliFlag))

	// 布尔
	case reflect.Bool:
		setByPath(config, configPath, cmd.Bool(cliFlag))

	// 有符号整数
	case reflect.Int:
		setByPath(config, configPath, cmd.Int(cliFlag))
	case reflect.Int8:
		setByPath(config, configPath, cmd.Int8(cliFlag))
	case reflect.Int16:
		setByPath(config, configPath, cmd.Int16(cliFlag))
	case reflect.Int32:
		setByPath(config, configPath, cmd.Int32(cliFlag))
	case reflect.Int64:
		setByPath(config, configPath, cmd.Int64(cliFlag))

	// 无符号整数
	case reflect.Uint:
		setByPath(config, configPath, cmd.Uint(cliFlag))
	case reflect.Uint8:
		setByPath(config, configPath, uint8(cmd.Uint(cliFlag))) //nolint:gosec // CLI value expected to be in uint8 range
	case reflect.Uint16:
		setByPath(config, configPath, cmd.Uint16(cliFlag))
	case reflect.Uint32:
		setByPath(config, configPath, cmd.Uint32(cliFlag))
	case reflect.Uint64:
		setByPath(config, configPath, cmd.Uint64(cliFlag))

	// 浮点数
	case reflect.Float32:
		setByPath(config, configPath, cmd.Float32(cliFlag))
	case reflect.Float64:
		setByPath(config, configPath, cmd.Float64(cliFlag))

	// 切片类型
	case reflect.Slice:
		setSliceFlagValue(cmd, config, configPath, cliFlag, fieldType)

	// Map 类型
	case reflect.Map:
		if fieldType.Key().Kind() == reflect.String && fieldType.Elem().Kind() == reflect.String {
			setByPath(config, configPath, cmd.StringMap(cliFlag))
		}

	default:
		// 不支持的类型，忽略
	}
}

// setSliceFlagValue 处理切片类型的 CLI flag 值。
func setSliceFlagValue(cmd *cli.Command, config map[string]any, configPath, cliFlag string, fieldType reflect.Type) {
	elemType := fieldType.Elem()

	// 先检查特殊元素类型
	if elemType == reflect.TypeFor[time.Time]() {
		setByPath(config, configPath, cmd.TimestampArgs(cliFlag))

		return
	}

	switch elemType.Kind() {
	case reflect.String:
		setByPath(config, configPath, cmd.StringSlice(cliFlag))
	case reflect.Int:
		setByPath(config, configPath, cmd.IntSlice(cliFlag))
	case reflect.Int8:
		setByPath(config, configPath, cmd.Int8Slice(cliFlag))
	case reflect.Int16:
		setByPath(config, configPath, cmd.Int16Slice(cliFlag))
	case reflect.Int32:
		setByPath(config, configPath, cmd.Int32Slice(cliFlag))
	case reflect.Int64:
		setByPath(config, configPath, cmd.Int64Slice(cliFlag))
	case reflect.Uint16:
		setByPath(config, configPath, cmd.Uint16Slice(cliFlag))
	case reflect.Uint32:
		setByPath(config, configPath, cmd.Uint32Slice(cliFlag))
	case reflect.Float32:
		setByPath(config, configPath, cmd.Float32Slice(cliFlag))
	case reflect.Float64:
		setByPath(config, configPath, cmd.Float64Slice(cliFlag))

	default:
		// 不支持的切片元素类型，忽略
	}
}
