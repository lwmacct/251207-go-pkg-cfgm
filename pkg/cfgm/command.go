package cfgm

import (
	"fmt"
	"strings"

	"github.com/urfave/cli/v3"
)

const configFlagName = "config"
const envPrefixFlagName = "env-prefix"

func rootFlags() []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{Name: configFlagName, Aliases: []string{"c"}, Usage: "配置文件路径"},
		&cli.StringFlag{Name: envPrefixFlagName, Aliases: []string{"e"}, Usage: "环境变量前缀"},
	}
}

func mergeCLIFlags(existing, additions []cli.Flag) ([]cli.Flag, error) {
	seen := make(map[string]string)
	for _, flag := range existing {
		if flag == nil {
			continue
		}
		primary := ""
		if names := flag.Names(); len(names) > 0 {
			primary = names[0]
		}
		for _, name := range flag.Names() {
			if previous, exists := seen[name]; exists {
				return nil, fmt.Errorf("CLI flag --%s is ambiguous: matches --%s and --%s", name, previous, primary)
			}
			seen[name] = primary
		}
	}
	for _, flag := range additions {
		if flag == nil {
			continue
		}
		primary := ""
		if names := flag.Names(); len(names) > 0 {
			primary = names[0]
		}
		for _, name := range flag.Names() {
			if previous, exists := seen[name]; exists {
				return nil, fmt.Errorf("CLI flag --%s is ambiguous: matches --%s and --%s", name, previous, primary)
			}
			seen[name] = primary
		}
	}
	return append(append([]cli.Flag(nil), existing...), additions...), nil
}

func commandConfigPath(cmd *cli.Command) string {
	if cmd == nil {
		return ""
	}
	for _, command := range cmd.Lineage() {
		if command.IsSet(configFlagName) {
			return command.String(configFlagName)
		}
	}
	return ""
}

func commandEnvPrefix(cmd *cli.Command) (string, bool) {
	if cmd == nil {
		return "", false
	}
	for _, command := range cmd.Lineage() {
		if command.IsSet(envPrefixFlagName) {
			return command.String(envPrefixFlagName), true
		}
	}
	return "", false
}

func commandRootName(cmd *cli.Command) string {
	if cmd == nil {
		return ""
	}
	lineage := cmd.Lineage()
	if len(lineage) == 0 {
		return ""
	}
	return strings.TrimSpace(lineage[len(lineage)-1].Name)
}

func commandLineagePath(cmd *cli.Command) string {
	if cmd == nil {
		return ""
	}
	lineage := cmd.Lineage()
	if len(lineage) <= 1 {
		return ""
	}
	names := make([]string, 0, len(lineage)-1)
	for index := len(lineage) - 2; index >= 0; index-- {
		name := strings.TrimSpace(lineage[index].Name)
		if name != "" {
			names = append(names, name)
		}
	}
	return strings.Join(names, ".")
}
