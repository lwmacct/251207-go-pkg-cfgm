package templexp_test

import (
	"fmt"
	"os"

	"github.com/lwmacct/251207-go-pkg-cfgm/pkg/templexp"
)

// Example_shellExpansion 演示 Shell 参数展开。
func Example_shellExpansion() {
	_ = os.Setenv("API_KEY", "sk-12345")
	defer func() { _ = os.Unsetenv("API_KEY") }()

	result, _ := templexp.ExpandTemplate(`key=${API_KEY}`)
	fmt.Println(result)

	// Output:
	// key=sk-12345
}

// Example_shellFallback 演示默认值回退语义。
func Example_shellFallback() {
	result, _ := templexp.ExpandTemplate(`host=${HOST:-localhost}`)
	fmt.Println(result)

	// Output:
	// host=localhost
}

// Example_shellAssign 演示 := 赋值仅在当前展开内生效。
func Example_shellAssign() {
	result, _ := templexp.ExpandTemplate(`${MODEL:=gpt-4}-${MODEL}`)
	fmt.Println(result)

	// Output:
	// gpt-4-gpt-4
}
