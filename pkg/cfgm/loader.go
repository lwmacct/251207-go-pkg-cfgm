package cfgm

import (
	"context"
	"fmt"
	"os"
	"sort"

	"github.com/lwmacct/251207-go-pkg-cfgm/pkg/templexp"
)

type Source interface {
	Name() string
	Load(ctx context.Context, schema Schema) (map[string]any, error)
}

type SourceReport struct {
	Name string
	Keys []string
}

type Report struct {
	Sources []SourceReport
}

func expandTemplateValues(value any, path string) (any, error) {
	switch typed := value.(type) {
	case map[string]any:
		keys := make([]string, 0, len(typed))
		for key := range typed {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			expanded, err := expandTemplateValues(typed[key], templateMapPath(path, key))
			if err != nil {
				return nil, err
			}
			typed[key] = expanded
		}
		return typed, nil
	case []any:
		for index, item := range typed {
			expanded, err := expandTemplateValues(item, fmt.Sprintf("%s[%d]", path, index))
			if err != nil {
				return nil, err
			}
			typed[index] = expanded
		}
		return typed, nil
	case string:
		if !containsTemplateMarker(typed) {
			return typed, nil
		}
		expanded, err := templexp.Expand(typed, os.LookupEnv)
		if err != nil {
			return nil, fmt.Errorf("expand template at %s: %w", path, err)
		}
		return expanded, nil
	default:
		return value, nil
	}
}

func templateMapPath(parent, key string) string {
	if parent == "" {
		return key
	}

	return parent + "." + key
}

func containsTemplateMarker(value string) bool {
	for index := range len(value) - 1 {
		if value[index] == '$' && (value[index+1] == '{' || value[index+1] == '$') {
			return true
		}
	}
	return false
}
