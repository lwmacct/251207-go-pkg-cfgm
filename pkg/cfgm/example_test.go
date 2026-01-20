// Author: lwmacct (https://github.com/lwmacct)
package cfgm_test

import (
	"fmt"
	"os"
	"time"

	"github.com/lwmacct/251207-go-pkg-cfgm/pkg/cfgm"
)

// Example_defaultPaths 演示 DefaultPaths 的搜索顺序。
func Example_defaultPaths() {
	// 不指定应用名称时，返回基础路径
	paths := cfgm.DefaultPaths()
	fmt.Println("基础路径数量:", len(paths))

	// 指定应用名称时，会包含应用专属配置路径
	paths = cfgm.DefaultPaths("myapp")
	fmt.Println("带应用名路径数量:", len(paths))

	// Output:
	// 基础路径数量: 2
	// 带应用名路径数量: 5
}

// Example_exampleYAML 演示根据配置结构体生成 YAML 示例。
func Example_exampleYAML() {
	// 定义配置结构体，使用 json 和 desc 标签
	type ServerConfig struct {
		Host string `json:"host" desc:"服务器主机地址"`
		Port int    `json:"port" desc:"服务器端口"`
	}
	type AppConfig struct {
		Name    string        `json:"name"    desc:"应用名称"`
		Debug   bool          `json:"debug"   desc:"是否启用调试模式"`
		Timeout time.Duration `json:"timeout" desc:"超时时间"`
		Server  ServerConfig  `json:"server"  desc:"服务器配置"`
	}

	// 创建默认配置
	defaultCfg := AppConfig{
		Name:    "example-app",
		Debug:   false,
		Timeout: 30 * time.Second,
		Server: ServerConfig{
			Host: "localhost",
			Port: 8080,
		},
	}

	// 生成 YAML 示例
	yaml := cfgm.ExampleYAML(defaultCfg)
	fmt.Println(string(yaml))

	// Output:
	// # 配置示例文件, 复制此文件为 config.yaml 并根据需要修改
	// name: 'example-app' # 应用名称
	// debug: false # 是否启用调试模式
	// timeout: 30s # 超时时间
	//
	// # 服务器配置
	// server:
	//   host: 'localhost' # 服务器主机地址
	//   port: 8080 # 服务器端口
}

// Example_load 演示如何加载配置。
//
// Load 函数按以下优先级合并配置:
//  1. 默认值 (最低优先级)
//  2. 配置文件
//  3. 环境变量
//  4. CLI flags (最高优先级)
func Example_load() {
	type Config struct {
		Name  string `json:"name"`
		Debug bool   `json:"debug"`
	}

	defaultCfg := Config{
		Name:  "default-app",
		Debug: false,
	}

	// 使用函数选项模式加载配置
	// 配置文件不存在时，使用默认值
	cfg, err := cfgm.Load(defaultCfg,
		cfgm.WithConfigPaths("nonexistent.yaml"),
	)
	if err != nil {
		fmt.Println("加载失败:", err)

		return
	}

	fmt.Println("Name:", cfg.Name)
	fmt.Println("Debug:", cfg.Debug)

	// Output:
	// Name: default-app
	// Debug: false
}

// Example_load_withEnvPrefix 演示如何通过环境变量加载配置。
//
// 环境变量命名规则：
//   - 前缀 + 大写的配置 key
//   - 点号 (.) 和连字符 (-) 转为下划线 (_)
func Example_load_withEnvPrefix() {
	type Config struct {
		Name  string `json:"name"`
		Debug bool   `json:"debug"`
	}

	defaultCfg := Config{
		Name:  "default-app",
		Debug: false,
	}

	// 使用环境变量前缀 "MYAPP_"
	// 支持的环境变量：MYAPP_NAME, MYAPP_DEBUG
	cfg, err := cfgm.Load(defaultCfg,
		cfgm.WithEnvPrefix("MYAPP_"),
	)
	if err != nil {
		fmt.Println("加载失败:", err)

		return
	}

	// 如果设置了 MYAPP_NAME=prod-app，则 cfg.Name 为 "prod-app"
	// 如果没有设置环境变量，则使用默认值
	fmt.Println("Name:", cfg.Name)
	fmt.Println("Debug:", cfg.Debug)

	// Output:
	// Name: default-app
	// Debug: false
}

// Example_marshalJSON 演示如何根据配置结构体生成 JSON。
func Example_marshalJSON() {
	type ServerConfig struct {
		Host string `json:"host"`
		Port int    `json:"port"`
	}
	type AppConfig struct {
		Name   string       `json:"name"`
		Debug  bool         `json:"debug"`
		Server ServerConfig `json:"server"`
	}

	defaultCfg := AppConfig{
		Name:  "example-app",
		Debug: false,
		Server: ServerConfig{
			Host: "localhost",
			Port: 8080,
		},
	}

	// 生成 JSON
	jsonBytes := cfgm.MarshalJSON(defaultCfg)
	fmt.Println(string(jsonBytes))

	// Output:
	// {
	//   "name": "example-app",
	//   "debug": false,
	//   "server": {
	//     "host": "localhost",
	//     "port": 8080
	//   }
	// }
}

// Example_load_withJSONConfig 演示如何加载 JSON 格式的配置文件。
//
// Load 函数会根据文件扩展名自动选择解析器：
//   - .yaml, .yml → YAML 解析器
//   - .json → JSON 解析器
func Example_load_withJSONConfig() {
	type Config struct {
		Name  string `json:"name"`
		Debug bool   `json:"debug"`
	}

	// 创建临时 JSON 配置文件
	configContent := `{
  "name": "json-app",
  "debug": true
}`
	tmpFile := "/tmp/example_json_test.json"
	if err := os.WriteFile(tmpFile, []byte(configContent), 0600); err != nil {
		fmt.Println("创建临时文件失败:", err)

		return
	}
	defer func() { _ = os.Remove(tmpFile) }()

	defaultCfg := Config{
		Name:  "default-app",
		Debug: false,
	}

	// 根据 .json 扩展名自动使用 JSON 解析器
	cfg, err := cfgm.Load(defaultCfg,
		cfgm.WithConfigPaths(tmpFile),
	)
	if err != nil {
		fmt.Println("加载失败:", err)

		return
	}

	fmt.Println("Name:", cfg.Name)
	fmt.Println("Debug:", cfg.Debug)

	// Output:
	// Name: json-app
	// Debug: true
}

// Example_withAppName 演示如何使用 WithAppName 设置应用名称。
//
// WithAppName 会自动配置默认的配置文件搜索路径（如果未通过 WithConfigPaths 显式设置）。
func Example_withAppName() {
	type Config struct {
		Name  string `json:"name"`
		Debug bool   `json:"debug"`
	}

	defaultCfg := Config{
		Name:  "default-app",
		Debug: false,
	}

	// 使用 WithAppName 设置应用名称
	// 会自动搜索 .myapp.yaml, ~/.myapp.yaml, /etc/myapp/config.yaml 等路径
	cfg, err := cfgm.Load(defaultCfg,
		cfgm.WithAppName("myapp"),
	)
	if err != nil {
		fmt.Println("加载失败:", err)

		return
	}

	// 配置文件不存在时，使用默认值
	fmt.Println("Name:", cfg.Name)
	fmt.Println("Debug:", cfg.Debug)

	// Output:
	// Name: default-app
	// Debug: false
}
