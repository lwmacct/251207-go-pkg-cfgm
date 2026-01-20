package templexp

import (
	"fmt"
	"os"
	"strings"
)

// ═══════════════════════════════════════════════════════════════════════════
// 模板数据对象
// ═══════════════════════════════════════════════════════════════════════════

// newTemplateData 生成当前环境变量快照。
//
// 该快照仅用于本次展开，":=" 的赋值只会写入这份数据。
func newTemplateData() map[string]string {
	vars := make(map[string]string)
	for _, env := range os.Environ() {
		parts := strings.SplitN(env, "=", 2)
		if len(parts) == 2 {
			vars[parts[0]] = parts[1]
		}
	}

	return vars
}

// ═══════════════════════════════════════════════════════════════════════════
// Shell Parameter Expansion
// ═══════════════════════════════════════════════════════════════════════════

func isVarNameStart(ch byte) bool {
	return (ch >= 'A' && ch <= 'Z') || (ch >= 'a' && ch <= 'z') || ch == '_'
}

func isVarNameChar(ch byte) bool {
	return isVarNameStart(ch) || (ch >= '0' && ch <= '9')
}

func parseShellParameter(expr string) (string, string, string, bool) {
	if expr == "" {
		return "", "", "", false
	}
	if !isVarNameStart(expr[0]) {
		return "", "", "", false
	}

	i := 1
	for i < len(expr) && isVarNameChar(expr[i]) {
		i++
	}

	name := expr[:i]
	rest := expr[i:]
	if rest == "" {
		return name, "", "", true
	}

	if len(rest) >= 2 && rest[0] == ':' {
		switch rest[1] {
		case '-', '+', '?', '=':
			return name, rest[:2], rest[2:], true
		}
	}

	switch rest[0] {
	case '-', '+', '?', '=':
		return name, rest[:1], rest[1:], true
	}

	return "", "", "", false
}

func errorMessage(name, word string) error {
	if word == "" {
		return fmt.Errorf("templexp: %s: parameter null or not set", name)
	}

	return fmt.Errorf("templexp: %s: %s", name, word)
}

func expandShellWord(word string, env map[string]string) (string, error) {
	if !strings.Contains(word, "${") {
		return word, nil
	}

	return expandShellParameters(word, env)
}

func expandShellExpression(expr string, env map[string]string) (string, bool, error) {
	name, op, word, ok := parseShellParameter(expr)
	if !ok {
		return "", false, nil
	}

	val, isSet := env[name]
	switch op {
	case "":
		if isSet {
			return val, true, nil
		}
		return "", true, nil
	case ":-":
		if !isSet || val == "" {
			expanded, err := expandShellWord(word, env)
			if err != nil {
				return "", false, err
			}
			return expanded, true, nil
		}
		return val, true, nil
	case "-":
		if !isSet {
			expanded, err := expandShellWord(word, env)
			if err != nil {
				return "", false, err
			}
			return expanded, true, nil
		}
		return val, true, nil
	case ":+": // set and not empty
		if isSet && val != "" {
			expanded, err := expandShellWord(word, env)
			if err != nil {
				return "", false, err
			}
			return expanded, true, nil
		}
		return "", true, nil
	case "+":
		if isSet {
			expanded, err := expandShellWord(word, env)
			if err != nil {
				return "", false, err
			}
			return expanded, true, nil
		}
		return "", true, nil
	case ":?":
		if !isSet || val == "" {
			return "", false, errorMessage(name, word)
		}
		return val, true, nil
	case "?":
		if !isSet {
			return "", false, errorMessage(name, word)
		}
		return val, true, nil
	case ":=":
		if !isSet || val == "" {
			expanded, err := expandShellWord(word, env)
			if err != nil {
				return "", false, err
			}
			env[name] = expanded
			return expanded, true, nil
		}
		return val, true, nil
	case "=":
		if !isSet {
			expanded, err := expandShellWord(word, env)
			if err != nil {
				return "", false, err
			}
			env[name] = expanded
			return expanded, true, nil
		}
		return val, true, nil
	}

	return "", false, nil
}

func expandShellParameters(text string, env map[string]string) (string, error) {
	var buf strings.Builder
	buf.Grow(len(text))

	for i := 0; i < len(text); {
		ch := text[i]
		if ch != '$' {
			buf.WriteByte(ch)
			i++
			continue
		}
		if i+1 >= len(text) {
			buf.WriteByte(ch)
			i++
			continue
		}

		next := text[i+1]
		if next == '$' {
			buf.WriteByte('$')
			i += 2
			continue
		}
		if next != '{' {
			buf.WriteByte(ch)
			i++
			continue
		}

		end := findMatchingBrace(text, i+2)
		if end == -1 {
			buf.WriteByte(ch)
			i++
			continue
		}

		expr := text[i+2 : end]
		expanded, ok, err := expandShellExpression(expr, env)
		if err != nil {
			return "", err
		}
		if ok {
			buf.WriteString(expanded)
		} else {
			buf.WriteString(text[i : end+1])
		}

		i = end + 1
	}

	return buf.String(), nil
}

func findMatchingBrace(text string, start int) int {
	depth := 0
	for i := start; i < len(text); i++ {
		if text[i] == '$' && i+1 < len(text) && text[i+1] == '{' {
			depth++
			i++
			continue
		}
		if text[i] == '}' {
			if depth == 0 {
				return i
			}
			depth--
		}
	}

	return -1
}

// ═══════════════════════════════════════════════════════════════════════════
// 模板渲染
// ═══════════════════════════════════════════════════════════════════════════

// ExpandTemplate 对输入字符串执行 Shell 参数展开。
//
// 支持语法：
//   - ${VAR} - 变量替换
//   - ${VAR:-default} / ${VAR-default} - fallback
//   - ${VAR:+alt} / ${VAR+alt} - 替代值
//   - ${VAR:?msg} / ${VAR?msg} - 必填校验
//   - ${VAR:=default} / ${VAR=default} - 赋值（仅作用于当前展开）
//
// 返回展开后的字符串；仅在必填校验失败时返回 error。
func ExpandTemplate(text string) (string, error) {
	data := newTemplateData()
	return expandShellParameters(text, data)
}
