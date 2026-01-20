// Package templexp 提供配置字符串的 Shell 参数展开。
//
// 该包仅处理 ${...} 语法，适合在 YAML/JSON 等配置文件中做轻量替换。
// 不执行命令、不引入模板引擎，强调可读性与可预测性。
//
// # 设计参考
//
//   - Bash 参数展开: https://www.gnu.org/software/bash/manual/bash.html#Shell-Parameter-Expansion
//
// # 语义说明
//
//  1. 仅做字符串层面的替换（不解析 $VAR）
//  2. 支持嵌套展开与 "$$" 字面量
//  3. ":=" 赋值仅作用于当前展开过程
//  4. 无法识别的表达式保持原样
//
// # 快速开始
//
// 展开配置文件中的环境变量引用：
//
//	content := `api_key: "${OPENAI_API_KEY}"`
//	expanded, err := templexp.ExpandTemplate(content)
//
// 使用默认值处理缺失的环境变量：
//
//	content := `model: "${LLM_MODEL:-gpt-4}"`
//	expanded, err := templexp.ExpandTemplate(content)
//
// 详见 [ExpandTemplate] 文档。
package templexp
