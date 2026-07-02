package cfgm

import (
	"context"
	"errors"
	"fmt"
	"os"
	"reflect"
	"strings"

	"github.com/urfave/cli/v3"

	"github.com/lwmacct/251207-go-pkg-cfgm/pkg/templexp"
)

type fileSource struct {
	paths     []string
	optional  bool
	templates bool
}

// File loads one required config file.
func File(path string, opts ...FileOption) SourceOption {
	return Files([]string{path}, opts...)
}

// Files loads the first existing file from paths.
func Files(paths []string, opts ...FileOption) SourceOption {
	source := &fileSource{
		paths:     append([]string{}, paths...),
		templates: true,
	}
	for _, opt := range opts {
		opt(source)
	}

	return source
}

// FileOption configures file sources.
type FileOption func(*fileSource)

// Optional allows a file source to be absent.
func Optional() FileOption {
	return func(s *fileSource) {
		s.optional = true
	}
}

// Required requires a file source to exist. This is the default.
func Required() FileOption {
	return func(s *fileSource) {
		s.optional = false
	}
}

// ExpandTemplates expands ${...} templates before parsing the file.
//
// File template expansion is enabled by default; this option is mainly useful
// after Raw when composing file options.
func ExpandTemplates() FileOption {
	return func(s *fileSource) {
		s.templates = true
	}
}

// Raw disables ${...} template expansion for this file source.
func Raw() FileOption {
	return func(s *fileSource) {
		s.templates = false
	}
}

func (s *fileSource) Name() string {
	if len(s.paths) == 1 {
		return "file:" + s.paths[0]
	}

	return "files"
}

func (s *fileSource) Load(ctx context.Context, _ ConfigSchema) (map[string]any, error) {
	if len(s.paths) == 0 {
		if s.optional {
			return map[string]any{}, nil
		}

		return nil, errors.New("no config paths configured")
	}

	for _, path := range s.paths {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		content, err := os.ReadFile(path) //nolint:gosec // path is provided by the caller
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			if s.optional {
				continue
			}

			return nil, fmt.Errorf("read %s: %w", path, err)
		}

		if s.templates {
			expanded, expandErr := templexp.ExpandTemplate(string(content))
			if expandErr != nil {
				return nil, fmt.Errorf("expand template in %s: %w", path, expandErr)
			}
			content = []byte(expanded)
		}

		configMap, err := parseConfigBytes(path, content)
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", path, err)
		}

		return configMap, nil
	}

	if s.optional {
		return map[string]any{}, nil
	}

	return nil, fmt.Errorf("none of the config files exist: %s", strings.Join(s.paths, ", "))
}

func (s *fileSource) applyLoadOption(options *loadOptions) {
	options.sources = append(options.sources, s)
}

func (s *fileSource) setTemplateExpansion(enabled bool) {
	s.templates = enabled
}

type envSource struct {
	prefix string
}

// Env loads environment variables for schema fields using the given prefix.
func Env(prefix string) SourceOption {
	return &envSource{prefix: prefix}
}

func (s *envSource) Name() string {
	return "env:" + s.prefix
}

func (s *envSource) Load(ctx context.Context, schema ConfigSchema) (map[string]any, error) {
	if s.prefix == "" {
		return map[string]any{}, nil
	}

	out := map[string]any{}
	for _, field := range schema.Fields() {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		envKey := s.prefix + envName(field.Path)
		if val := os.Getenv(envKey); val != "" {
			setByPath(out, field.Path, val)
		}
	}

	return out, nil
}

func (s *envSource) applyLoadOption(options *loadOptions) {
	options.sources = append(options.sources, s)
}

func envName(path string) string {
	return strings.ToUpper(strings.NewReplacer(".", "_", "-", "_").Replace(path))
}

type cliSource struct {
	cmd             *cli.Command
	ignoredCLIFlags map[string]bool
}

// CLI loads explicitly set urfave/cli flags.
func CLI(cmd *cli.Command, opts ...CLISourceOption) SourceOption {
	source := &cliSource{cmd: cmd}
	for _, opt := range opts {
		opt(source)
	}

	return source
}

// CLISourceOption configures CLI sources.
type CLISourceOption func(*cliSource)

// IgnoreCLIFlags marks flags that do not map to config fields.
func IgnoreCLIFlags(names ...string) CLISourceOption {
	return func(s *cliSource) {
		if s.ignoredCLIFlags == nil {
			s.ignoredCLIFlags = make(map[string]bool, len(names))
		}
		for _, name := range names {
			if name != "" {
				s.ignoredCLIFlags[name] = true
			}
		}
	}
}

func (s *cliSource) Name() string {
	return "cli"
}

func (s *cliSource) Load(ctx context.Context, schema ConfigSchema) (map[string]any, error) {
	if s.cmd == nil {
		return map[string]any{}, nil
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if schema.index == nil {
		return map[string]any{}, nil
	}

	fields, flagNames, err := schema.index.commandFields(s.cmd)
	if err != nil {
		return nil, err
	}
	if err := validateCommandFlags(s.cmd, fields, s.ignoredCLIFlags); err != nil {
		return nil, err
	}

	out := map[string]any{}
	for _, flagName := range flagNames {
		if !s.cmd.IsSet(flagName) {
			continue
		}

		field := fields[flagName]
		if !setCLIFlagValue(s.cmd, out, field.configPath, flagName, field.fieldType) {
			return nil, fmt.Errorf("unsupported CLI flag type for --%s: %s", flagName, field.fieldType)
		}
	}

	return out, nil
}

func (s *cliSource) applyLoadOption(options *loadOptions) {
	options.sources = append(options.sources, s)
}

func isStringMapType(fieldType reflect.Type) bool {
	return fieldType.Kind() == reflect.Map &&
		fieldType.Key().Kind() == reflect.String &&
		fieldType.Elem().Kind() == reflect.String
}
