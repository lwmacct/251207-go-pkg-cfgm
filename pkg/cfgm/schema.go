package cfgm

import (
	"fmt"
	"reflect"
	"strings"
)

// ConfigSchema 提供配置结构体的派生元数据。
//
// 它复用 cfgm 的 CLI flag 映射规则，可用于在命令定义阶段从 desc tag
// 获取 flag Usage，避免配置示例注释与 CLI help 文案重复维护。
type ConfigSchema struct {
	index *cliConfigIndex
}

// Field describes one leaf config field derived from a struct.
type Field struct {
	Path string
	Type reflect.Type
	Desc string
}

// CommandSchema 提供指定命令链下的配置元数据投影。
type CommandSchema struct {
	fields map[string]cliFieldMeta
	err    error
}

// Schema 从默认配置结构体构建配置元数据。
func Schema[T any](defaultConfig T) ConfigSchema {
	return ConfigSchema{
		index: newCLIConfigIndex(reflect.TypeOf(defaultConfig)),
	}
}

// Fields returns the leaf fields in the config schema.
func (s ConfigSchema) Fields() []Field {
	if s.index == nil {
		return nil
	}

	fields := make([]Field, 0, len(s.index.fields))
	for _, field := range s.index.fields {
		fields = append(fields, Field{
			Path: field.configPath,
			Type: field.fieldType,
			Desc: field.desc,
		})
	}

	return fields
}

// Command 返回指定命令链下的元数据投影。
//
// names 使用 urfave/cli 的命令名顺序，例如 Command("server", "service")。
// flag 名称映射规则与 Command 一致：递归剥离命令链前缀，完整路径作为 fallback。
func (s ConfigSchema) Command(names ...string) CommandSchema {
	if s.index == nil {
		return CommandSchema{}
	}

	fields, _, err := s.index.fieldsForScopes(s.index.commandScopesFromNames(names))

	return CommandSchema{
		fields: fields,
		err:    err,
	}
}

func (s ConfigSchema) validateKeys(keys []string) error {
	var unknown []string
	for _, key := range keys {
		if key == "" || s.hasPath(key) {
			continue
		}
		unknown = append(unknown, key)
	}
	if len(unknown) == 0 {
		return nil
	}

	return fmt.Errorf("unknown config keys:\n  - %s", strings.Join(unknown, "\n  - "))
}

func (s ConfigSchema) hasPath(path string) bool {
	if s.index == nil {
		return false
	}
	if _, ok := s.index.nullableStructPaths[path]; ok {
		return true
	}
	for _, field := range s.index.fields {
		if path == field.configPath {
			return true
		}
		if isMapType(field.fieldType) && strings.HasPrefix(path, field.configPath+".") {
			return true
		}
	}

	return false
}

// Usage 返回 flagName 对应配置字段的 desc tag。
//
// 找不到字段或存在映射冲突时返回空字符串。需要严格失败时使用 MustUsage。
func (s CommandSchema) Usage(flagName string) string {
	if s.err != nil || s.fields == nil {
		return ""
	}

	field, ok := s.fields[flagName]
	if !ok {
		return ""
	}

	return field.desc
}

// MustUsage 返回 flagName 对应配置字段的 desc tag。
//
// 当命令投影存在冲突或 flagName 找不到配置字段时 panic，适合在静态命令定义中暴露错误。
func (s CommandSchema) MustUsage(flagName string) string {
	if s.err != nil {
		panic(s.err)
	}
	if s.fields == nil {
		panic(fmt.Errorf("cfgm: CLI flag --%s has no matching config field", flagName))
	}

	field, ok := s.fields[flagName]
	if !ok {
		panic(fmt.Errorf("cfgm: CLI flag --%s has no matching config field", flagName))
	}

	return field.desc
}

// UsageMap 返回当前命令投影下 flagName 到 desc 的映射副本。
//
// 若命令投影存在冲突，返回空 map。需要严格失败时使用 MustUsageMap。
func (s CommandSchema) UsageMap() map[string]string {
	if s.err != nil || s.fields == nil {
		return map[string]string{}
	}

	out := make(map[string]string, len(s.fields))
	for name, field := range s.fields {
		out[name] = field.desc
	}

	return out
}

// MustUsageMap 返回当前命令投影下 flagName 到 desc 的映射副本。
func (s CommandSchema) MustUsageMap() map[string]string {
	if s.err != nil {
		panic(s.err)
	}

	return s.UsageMap()
}
