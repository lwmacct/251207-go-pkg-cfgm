package cfgm

import (
	"strings"

	"github.com/urfave/cli/v3"
)

const configFlagName = "config"
const envPrefixFlagName = "env-prefix"

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
