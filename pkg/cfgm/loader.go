package cfgm

import (
	"context"

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

func expandTemplateValues(value any) (any, error) {
	switch typed := value.(type) {
	case map[string]any:
		for key, item := range typed {
			expanded, err := expandTemplateValues(item)
			if err != nil {
				return nil, err
			}
			typed[key] = expanded
		}
		return typed, nil
	case []any:
		for index, item := range typed {
			expanded, err := expandTemplateValues(item)
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
		expanded, err := templexp.ExpandTemplate(typed)
		if err != nil {
			return nil, err
		}
		return expanded, nil
	default:
		return value, nil
	}
}

func containsTemplateMarker(value string) bool {
	for index := range len(value) - 1 {
		if value[index] == '$' && value[index+1] == '{' {
			return true
		}
	}
	return false
}
