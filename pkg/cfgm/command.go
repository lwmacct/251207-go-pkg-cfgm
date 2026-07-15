package cfgm

import (
	"strings"

	"github.com/urfave/cli/v3"
)

const configFlagName = "config"
const envPrefixFlagName = "env-prefix"

// RootFlags returns cfgm's conventional root command flags.
func RootFlags() []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{Name: configFlagName, Aliases: []string{"c"}, Usage: "配置文件路径"},
		&cli.StringFlag{Name: envPrefixFlagName, Aliases: []string{"e"}, Usage: "环境变量前缀"},
	}
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
