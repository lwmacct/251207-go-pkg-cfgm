package cfgm

import (
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"github.com/go-viper/mapstructure/v2"
	yamlv3 "go.yaml.in/yaml/v3"
)

var (
	durationType = reflect.TypeFor[time.Duration]()
	timeType     = reflect.TypeFor[time.Time]()
)

func configTagName(field reflect.StructField) string {
	return parseTagName(field.Tag.Get("json"))
}

func parseTagName(tag string) string {
	if tag == "" {
		return ""
	}
	parts := strings.Split(tag, ",")
	if len(parts) == 0 || parts[0] == "" || parts[0] == "-" {
		return ""
	}

	return parts[0]
}

func isStructType(typ reflect.Type) bool {
	if typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}

	return typ.Kind() == reflect.Struct && typ != durationType && typ != timeType
}

func structToMap(cfg any) map[string]any {
	val := reflect.ValueOf(cfg)
	return structValueToMap(val, val.Type())
}

func structValueToMap(val reflect.Value, typ reflect.Type) map[string]any {
	if val.Kind() == reflect.Pointer {
		if val.IsNil() {
			return map[string]any{}
		}
		val = val.Elem()
		typ = typ.Elem()
	}
	if typ.Kind() != reflect.Struct {
		return map[string]any{}
	}

	out := make(map[string]any)
	for i := range typ.NumField() {
		field := typ.Field(i)
		if field.PkgPath != "" {
			continue
		}

		key := configTagName(field)
		if key == "" {
			continue
		}

		fieldVal := val.Field(i)
		out[key] = valueToAny(fieldVal, field.Type)
	}

	return out
}

func valueToAny(val reflect.Value, typ reflect.Type) any {
	if val.Kind() == reflect.Pointer {
		if val.IsNil() {
			return nil
		}
		val = val.Elem()
		typ = typ.Elem()
	}

	if isStructType(typ) {
		return structValueToMap(val, typ)
	}

	switch val.Kind() {
	case reflect.Slice:
		if val.IsNil() {
			return nil
		}
		out := make([]any, val.Len())
		for i := range val.Len() {
			elem := val.Index(i)
			out[i] = valueToAny(elem, elem.Type())
		}

		return out
	case reflect.Map:
		if val.IsNil() {
			return nil
		}
		out := make(map[string]any, val.Len())
		iter := val.MapRange()
		for iter.Next() {
			key := fmt.Sprintf("%v", iter.Key().Interface())
			out[key] = valueToAny(iter.Value(), iter.Value().Type())
		}

		return out
	default:
		return val.Interface()
	}
}

func parseConfigBytes(path string, content []byte) (map[string]any, error) {
	var raw any
	var err error
	if isJSONPath(path) {
		err = json.Unmarshal(content, &raw)
	} else {
		err = yamlv3.Unmarshal(content, &raw)
	}
	if err != nil {
		return nil, err
	}

	normalized := normalizeMapKeys(raw)
	if normalized == nil {
		return map[string]any{}, nil
	}
	configMap, ok := normalized.(map[string]any)
	if !ok {
		return nil, errors.New("config root must be object")
	}

	return configMap, nil
}

func isJSONPath(path string) bool {
	return strings.EqualFold(filepath.Ext(path), ".json")
}

func normalizeMapKeys(val any) any {
	switch typed := val.(type) {
	case map[string]any:
		out := make(map[string]any, len(typed))
		for key, value := range typed {
			out[key] = normalizeMapKeys(value)
		}

		return out
	case map[any]any:
		out := make(map[string]any, len(typed))
		for key, value := range typed {
			out[fmt.Sprintf("%v", key)] = normalizeMapKeys(value)
		}

		return out
	case []any:
		for i := range typed {
			typed[i] = normalizeMapKeys(typed[i])
		}
		return typed
	default:
		return val
	}
}

func mergeMaps(dst, src map[string]any) {
	for key, value := range src {
		if valueMap, ok := value.(map[string]any); ok {
			if dstMap, ok := dst[key].(map[string]any); ok {
				mergeMaps(dstMap, valueMap)
				continue
			}
		}

		dst[key] = value
	}
}

func setByPath(dst map[string]any, path string, value any) {
	parts := strings.Split(path, ".")
	current := dst
	for i, part := range parts {
		if i == len(parts)-1 {
			current[part] = value

			return
		}

		next, ok := current[part].(map[string]any)
		if !ok {
			next = make(map[string]any)
			current[part] = next
		}
		current = next
	}
}

func decodeConfigMap(data map[string]any, out any) error {
	conf := &mapstructure.DecoderConfig{
		DecodeHook: mapstructure.ComposeDecodeHookFunc(
			mapstructure.StringToTimeDurationHookFunc(),
			mapstructure.TextUnmarshallerHookFunc(),
		),
		Metadata:         nil,
		Result:           out,
		WeaklyTypedInput: true,
		TagName:          "json",
	}
	decoder, err := mapstructure.NewDecoder(conf)
	if err != nil {
		return err
	}

	return decoder.Decode(data)
}

func flattenMapKeys(data map[string]any) []string {
	var keys []string
	flattenMapKeysRecursive(data, "", &keys)

	return keys
}

func flattenMapKeysRecursive(data map[string]any, prefix string, keys *[]string) {
	for key, value := range data {
		fullKey := key
		if prefix != "" {
			fullKey = prefix + "." + key
		}
		if child, ok := value.(map[string]any); ok {
			if len(child) == 0 {
				*keys = append(*keys, fullKey)

				continue
			}
			flattenMapKeysRecursive(child, fullKey, keys)

			continue
		}

		*keys = append(*keys, fullKey)
	}
}
