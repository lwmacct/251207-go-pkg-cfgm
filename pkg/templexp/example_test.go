package templexp_test

import (
	"fmt"
	"os"

	"github.com/lwmacct/251207-go-pkg-cfgm/pkg/templexp"
)

func ExampleExpand() {
	_ = os.Setenv("API_KEY", "sk-12345")
	defer func() { _ = os.Unsetenv("API_KEY") }()

	result, _ := templexp.Expand(`key=${API_KEY}`, os.LookupEnv)
	fmt.Println(result)

	// Output:
	// key=sk-12345
}

func ExampleExpand_defaultValue() {
	result, _ := templexp.Expand(`host=${HOST:-localhost}`, os.LookupEnv)
	fmt.Println(result)

	// Output:
	// host=localhost
}

func ExampleExpand_requiredValue() {
	_, err := templexp.Expand(`${API_KEY:?API_KEY is required}`, func(string) (string, bool) {
		return "", false
	})
	fmt.Println(err)

	// Output:
	// templexp: API_KEY: API_KEY is required
}
