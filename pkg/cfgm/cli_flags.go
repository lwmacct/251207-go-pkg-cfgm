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
	fieldType  reflect.Type
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
	for idx := len(lineage) - 1; idx >= 0; idx-- {
		name := lineage[idx].Name
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

	for _, field := range i.fields {
		flagName, ok := cliFlagName(field.configPath, scopes)
		if !ok {
			continue
		}

		if existing, exists := fields[flagName]; exists {
			return nil, nil, fmt.Errorf(
				"cfgm: CLI flag --%s is ambiguous: matches %s, %s",
				flagName,
				existing.configPath,
				field.configPath,
			)
		}
		fields[flagName] = field
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

func isFrameworkFlag(flag cli.Flag) bool {
	for _, name := range flag.Names() {
		if name == "help" || name == "h" || name == "version" || name == "v" {
			return true
		}
	}
	return false
}

func cliFlagName(configPath string, scopes []string) (string, bool) {
	if !strings.Contains(configPath, ".") {
		return configPath, true
	}

	for _, v := range slices.Backward(scopes) {
		scopePrefix := v + "."
		if flagName, ok := strings.CutPrefix(configPath, scopePrefix); ok {
			return flagName, true
		}
	}

	if len(scopes) == 0 {
		return configPath, true
	}

	return "", false
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
