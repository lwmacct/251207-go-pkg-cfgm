package cfgm

import (
	"fmt"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/urfave/cli/v3"
)

type cliFieldMeta struct {
	configPath string
	fieldType  reflect.Type
}

type cliFlagCandidate struct {
	name     string
	scoped   bool
	field    cliFieldMeta
	priority int
}

type cliConfigIndex struct {
	rootType reflect.Type
	fields   []cliFieldMeta
}

func newCLIConfigIndex(typ reflect.Type) *cliConfigIndex {
	rootType := normalizeStructType(typ)
	index := &cliConfigIndex{
		rootType: rootType,
		fields:   make([]cliFieldMeta, 0),
	}
	index.collect(rootType, "")

	return index
}

func (i *cliConfigIndex) collect(typ reflect.Type, prefix string) {
	typ = normalizeStructType(typ)
	if typ.Kind() != reflect.Struct {
		return
	}

	for field := range typ.Fields() {
		key := configTagName(field)
		if key == "" {
			continue
		}

		fullPath := joinConfigPath(prefix, key)
		if isStructType(field.Type) {
			i.collect(field.Type, fullPath)

			continue
		}

		i.fields = append(i.fields, cliFieldMeta{
			configPath: fullPath,
			fieldType:  field.Type,
		})
	}
}

func (i *cliConfigIndex) commandScopes(cmd *cli.Command) []string {
	currentType := i.rootType
	if currentType.Kind() != reflect.Struct {
		return nil
	}

	var scopes []string
	scope := ""
	lineage := cmd.Lineage()
	for _, cmd := range slices.Backward(lineage) {
		name := cmd.Name
		nextType, ok := findNestedStructType(currentType, name)
		if !ok {
			continue
		}

		scope = joinConfigPath(scope, name)
		scopes = append(scopes, scope)
		currentType = nextType
	}

	return scopes
}

func (i *cliConfigIndex) commandFields(cmd *cli.Command) (map[string]cliFieldMeta, []string, error) {
	scopes := i.commandScopes(cmd)
	fields := make(map[string]cliFieldMeta)
	priorities := make(map[string]int)

	for _, field := range i.fields {
		for _, candidate := range cliFlagCandidates(field, scopes) {
			flagName := candidate.name
			if existingPriority, exists := priorities[flagName]; exists && existingPriority > candidate.priority {
				continue
			}
			if existing, exists := fields[flagName]; exists {
				if priorities[flagName] < candidate.priority {
					fields[flagName] = field
					priorities[flagName] = candidate.priority

					continue
				}
				return nil, nil, fmt.Errorf(
					"cfgm: CLI flag --%s is ambiguous: matches %s, %s",
					flagName,
					existing.configPath,
					field.configPath,
				)
			}
			fields[flagName] = field
			priorities[flagName] = candidate.priority
		}
	}

	flagNames := make([]string, 0, len(fields))
	for flagName := range fields {
		flagNames = append(flagNames, flagName)
	}
	slices.Sort(flagNames)

	return fields, flagNames, nil
}

func validateCommandFlags(cmd *cli.Command, fields map[string]cliFieldMeta) error {
	for _, command := range cmd.Lineage() {
		for _, flag := range command.VisibleFlags() {
			names := flag.Names()
			if len(names) == 0 || isFrameworkFlag(flag) {
				continue
			}
			if _, ok := fields[names[0]]; !ok {
				return fmt.Errorf("cfgm: CLI flag --%s has no matching config field", names[0])
			}
		}
	}

	return nil
}

type flagCoverageOptions struct {
	ignoredKeys map[string]bool
}

// FlagCoverageOption 配置 CLI flag 覆盖率校验。
type FlagCoverageOption func(*flagCoverageOptions)

// IgnoreConfigKeys 在 CLI flag 覆盖率校验中忽略指定配置 key。
//
// 适合排除不应通过 CLI 传入的敏感字段，例如 redis.password。
func IgnoreConfigKeys(keys ...string) FlagCoverageOption {
	return func(o *flagCoverageOptions) {
		if o.ignoredKeys == nil {
			o.ignoredKeys = make(map[string]bool, len(keys))
		}
		for _, key := range keys {
			o.ignoredKeys[key] = true
		}
	}
}

// ValidateCommandFlagCoverage 校验命令是否声明了指定配置前缀下的所有 CLI flags。
//
// prefixes 为配置前缀，如 "client"、"server"、"redis"。只校验这些前缀下的叶子配置项。
// flag 名称使用与 [LoadCmd] 相同的映射规则：递归剥离命令链前缀，剥离越深优先级越高。
func ValidateCommandFlagCoverage[T any](
	cmd *cli.Command,
	defaultConfig T,
	prefixes []string,
	opts ...FlagCoverageOption,
) error {
	options := &flagCoverageOptions{}
	for _, opt := range opts {
		opt(options)
	}

	index := newCLIConfigIndex(reflect.TypeOf(defaultConfig))
	scopes := index.commandScopes(cmd)
	visibleFlags := commandVisibleFlagNames(cmd)

	var missing []string
	for _, field := range index.fields {
		if !isCoveredPrefix(field.configPath, prefixes) || options.ignoredKeys[field.configPath] {
			continue
		}

		expectedFlags := cliFlagNames(field.configPath, scopes)
		if hasAnyFlag(visibleFlags, expectedFlags) {
			continue
		}

		missing = append(missing, fmt.Sprintf(
			"%s (expected one of: --%s)",
			field.configPath,
			strings.Join(expectedFlags, ", --"),
		))
	}

	if len(missing) > 0 {
		slices.Sort(missing)

		return fmt.Errorf("cfgm: missing CLI flags for config keys:\n  - %s", strings.Join(missing, "\n  - "))
	}

	return nil
}

// AssertCommandFlagCoverage 是 [ValidateCommandFlagCoverage] 的测试辅助版本。
func AssertCommandFlagCoverage[T any](
	t *testing.T,
	cmd *cli.Command,
	defaultConfig T,
	prefixes []string,
	opts ...FlagCoverageOption,
) {
	t.Helper()

	if err := ValidateCommandFlagCoverage(cmd, defaultConfig, prefixes, opts...); err != nil {
		t.Fatal(err)
	}
}

func commandVisibleFlagNames(cmd *cli.Command) map[string]bool {
	names := map[string]bool{}
	if cmd == nil {
		return names
	}

	for _, flag := range cmd.VisibleFlags() {
		for _, name := range flag.Names() {
			if name != "" {
				names[name] = true
			}
		}
	}

	return names
}

func isCoveredPrefix(configPath string, prefixes []string) bool {
	for _, prefix := range prefixes {
		if configPath == prefix || strings.HasPrefix(configPath, prefix+".") {
			return true
		}
	}

	return false
}

func hasAnyFlag(visibleFlags map[string]bool, names []string) bool {
	for _, name := range names {
		if visibleFlags[name] {
			return true
		}
	}

	return false
}

func isFrameworkFlag(flag cli.Flag) bool {
	for _, name := range flag.Names() {
		if name == "help" || name == "h" || name == "version" || name == "v" ||
			name == configFlagName || name == "c" {
			return true
		}
	}
	return false
}

func cliFlagNames(configPath string, scopes []string) []string {
	field := cliFieldMeta{configPath: configPath}
	candidates := cliFlagCandidates(field, scopes)
	names := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		names = append(names, candidate.name)
	}

	return names
}

func cliFlagCandidates(field cliFieldMeta, scopes []string) []cliFlagCandidate {
	candidates := []cliFlagCandidate{
		{
			name:     field.configPath,
			field:    field,
			priority: 0,
		},
	}

	for idx, v := range scopes {
		scopePrefix := v + "."
		if flagName, ok := strings.CutPrefix(field.configPath, scopePrefix); ok {
			candidates = append(candidates, cliFlagCandidate{
				name:     flagName,
				scoped:   true,
				field:    field,
				priority: idx + 1,
			})
		}
	}

	return candidates
}

func findNestedStructType(typ reflect.Type, key string) (reflect.Type, bool) {
	typ = normalizeStructType(typ)
	if typ.Kind() != reflect.Struct {
		return nil, false
	}

	for field := range typ.Fields() {
		if configTagName(field) != key || !isStructType(field.Type) {
			continue
		}

		return normalizeStructType(field.Type), true
	}

	return nil, false
}

func normalizeStructType(typ reflect.Type) reflect.Type {
	for typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}

	return typ
}

func joinConfigPath(prefix, key string) string {
	if prefix == "" {
		return key
	}

	return prefix + "." + key
}
