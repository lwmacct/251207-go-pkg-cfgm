package cfgm

import (
	"strings"

	"github.com/urfave/cli/v3"
)

const configFlagName = "config"
const envPrefixFlagName = "env-prefix"

type commandProfile struct {
	cmd          *cli.Command
	ignoredFlags []string
}

// Command applies cfgm's urfave/cli integration profile.
//
// It loads, in order:
//  1. the file pointed to by --config / -c when explicitly set
//  2. environment variables using --env-prefix / -e or the root command name
//  3. explicitly set CLI flags
func Command(cmd *cli.Command, opts ...CommandOption) Option {
	profile := &commandProfile{cmd: cmd}
	for _, opt := range opts {
		opt(profile)
	}

	return profile
}

// CommandOption configures Command.
type CommandOption func(*commandProfile)

// IgnoreFlags marks command flags that do not map to config fields.
func IgnoreFlags(names ...string) CommandOption {
	return func(profile *commandProfile) {
		profile.ignoredFlags = append(profile.ignoredFlags, names...)
	}
}

func (p *commandProfile) applyLoadOption(options *loadOptions) {
	if p == nil || p.cmd == nil {
		return
	}

	if configPath := commandConfigPath(p.cmd); configPath != "" {
		options.sources = append(options.sources, File(configPath))
	}

	if prefix, ok := commandEnvPrefix(p.cmd); ok {
		if prefix != "" {
			options.sources = append(options.sources, Env(prefix))
		}
	} else if prefix := commandNameToEnvPrefix(p.cmd); prefix != "" {
		options.sources = append(options.sources, Env(prefix))
	}

	options.sources = append(options.sources, CLI(p.cmd, IgnoreCLIFlags(p.ignoredFlags...)))
}

// ConfigFlag returns cfgm's conventional config file path flag.
func ConfigFlag() cli.Flag {
	return &cli.StringFlag{
		Name:    configFlagName,
		Aliases: []string{"c"},
		Usage:   "配置文件路径",
	}
}

// EnvPrefixFlag returns cfgm's conventional environment prefix flag.
func EnvPrefixFlag() cli.Flag {
	return &cli.StringFlag{
		Name:    envPrefixFlagName,
		Aliases: []string{"e"},
		Usage:   "环境变量前缀",
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

func commandNameToEnvPrefix(cmd *cli.Command) string {
	if cmd == nil {
		return ""
	}
	lineage := cmd.Lineage()
	if len(lineage) == 0 {
		return ""
	}
	rootCmd := lineage[len(lineage)-1]
	name := rootCmd.Name
	if name == "" {
		return ""
	}

	return strings.ToUpper(strings.ReplaceAll(name, "-", "_")) + "_"
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
