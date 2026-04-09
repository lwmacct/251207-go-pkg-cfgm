package cfgm

import (
	"fmt"
	"reflect"
	"slices"
	"strings"

	"github.com/urfave/cli/v3"
)

type cliFieldMeta struct {
	configPath string
	parentPath string
	flagName   string
	fieldType  reflect.Type
}

type cliConfigIndex struct {
	rootType     reflect.Type
	fields       []cliFieldMeta
	fieldsByFlag map[string][]cliFieldMeta
	flagNames    []string
}

func newCLIConfigIndex(typ reflect.Type) *cliConfigIndex {
	rootType := normalizeStructType(typ)
	index := &cliConfigIndex{
		rootType:     rootType,
		fields:       make([]cliFieldMeta, 0),
		fieldsByFlag: make(map[string][]cliFieldMeta),
	}
	index.collect(rootType, "")

	for flagName := range index.fieldsByFlag {
		index.flagNames = append(index.flagNames, flagName)
	}
	slices.Sort(index.flagNames)

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

		meta := cliFieldMeta{
			configPath: fullPath,
			parentPath: prefix,
			flagName:   key,
			fieldType:  field.Type,
		}
		i.fields = append(i.fields, meta)
		i.fieldsByFlag[key] = append(i.fieldsByFlag[key], meta)
	}
}

func inferCommandScope(cmd *cli.Command, rootType reflect.Type) string {
	currentType := normalizeStructType(rootType)
	if currentType.Kind() != reflect.Struct {
		return ""
	}

	scope := ""
	lineage := cmd.Lineage()
	for idx := len(lineage) - 1; idx >= 0; idx-- {
		nextType, ok := findNestedStructType(currentType, lineage[idx].Name)
		if !ok {
			continue
		}

		scope = joinConfigPath(scope, lineage[idx].Name)
		currentType = nextType
	}

	return scope
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

func isPathWithinScope(path, scope string) bool {
	return path == scope || strings.HasPrefix(path, scope+".")
}

func (i *cliConfigIndex) resolveField(scope, flagName string) (cliFieldMeta, bool, error) {
	fields := i.fieldsByFlag[flagName]
	if len(fields) == 0 {
		return cliFieldMeta{}, false, nil
	}

	if scope != "" {
		return resolveScopedField(scope, flagName, fields)
	}

	topLevel := make([]cliFieldMeta, 0, len(fields))
	for _, field := range fields {
		if field.parentPath == "" {
			topLevel = append(topLevel, field)
		}
	}

	if len(topLevel) == 1 {
		return topLevel[0], true, nil
	}
	if len(topLevel) > 1 {
		return cliFieldMeta{}, false, newCLIFlagAmbiguousError("", flagName, topLevel)
	}
	if len(fields) == 1 {
		return fields[0], true, nil
	}

	return cliFieldMeta{}, false, newCLIFlagAmbiguousError("", flagName, fields)
}

func resolveScopedField(scope, flagName string, fields []cliFieldMeta) (cliFieldMeta, bool, error) {
	direct := make([]cliFieldMeta, 0, len(fields))
	descendants := make([]cliFieldMeta, 0, len(fields))
	for _, field := range fields {
		if !isPathWithinScope(field.configPath, scope) {
			continue
		}

		descendants = append(descendants, field)
		if field.parentPath == scope {
			direct = append(direct, field)
		}
	}

	if len(direct) == 1 {
		return direct[0], true, nil
	}
	if len(direct) > 1 {
		return cliFieldMeta{}, false, newCLIFlagAmbiguousError(scope, flagName, direct)
	}
	if len(descendants) == 1 {
		return descendants[0], true, nil
	}
	if len(descendants) > 1 {
		return cliFieldMeta{}, false, newCLIFlagAmbiguousError(scope, flagName, descendants)
	}

	return cliFieldMeta{}, false, fmt.Errorf("cfgm: CLI flag --%s has no matching config field in scope %q", flagName, scope)
}

func newCLIFlagAmbiguousError(scope, flagName string, fields []cliFieldMeta) error {
	paths := make([]string, 0, len(fields))
	for _, field := range fields {
		paths = append(paths, field.configPath)
	}
	slices.Sort(paths)

	if scope == "" {
		return fmt.Errorf("cfgm: CLI flag --%s is ambiguous: matches %s", flagName, strings.Join(paths, ", "))
	}

	return fmt.Errorf("cfgm: CLI flag --%s is ambiguous in scope %q: matches %s", flagName, scope, strings.Join(paths, ", "))
}
