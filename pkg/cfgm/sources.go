package cfgm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"reflect"
	"strings"
)

type fileSource struct {
	paths     []string
	optional  bool
	templates *bool
}

func File(path string, opts ...FileOption) Source {
	return Files([]string{path}, opts...)
}

// Files loads the first existing file from paths.
func Files(paths []string, opts ...FileOption) Source {
	source := &fileSource{
		paths: append([]string{}, paths...),
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

// ExpandTemplates expands ${...} in string values after parsing the file.
//
// File template expansion is enabled by default; this option is mainly useful
// after Raw when composing file options.
func ExpandTemplates() FileOption {
	return func(s *fileSource) {
		enabled := true
		s.templates = &enabled
	}
}

// Raw disables ${...} template expansion for this file source.
func Raw() FileOption {
	return func(s *fileSource) {
		enabled := false
		s.templates = &enabled
	}
}

func (s *fileSource) Name() string {
	if len(s.paths) == 1 {
		return "file:" + s.paths[0]
	}

	return "files"
}

func (s *fileSource) Load(ctx context.Context, schema Schema) (map[string]any, error) {
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
			return nil, fmt.Errorf("read %s: %w", path, err)
		}

		expandTemplates := schema.expandTemplates
		if s.templates != nil {
			expandTemplates = *s.templates
		}
		configMap, err := parseConfigBytes(path, content)
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", path, err)
		}
		if expandTemplates {
			if _, expandErr := expandTemplateValues(configMap, "root"); expandErr != nil {
				return nil, fmt.Errorf("expand template in %s: %w", path, expandErr)
			}
		}

		return configMap, nil
	}

	if s.optional {
		return map[string]any{}, nil
	}

	return nil, fmt.Errorf("none of the config files exist: %s", strings.Join(s.paths, ", "))
}

type envSource struct {
	prefix string
}

// Env loads environment variables for schema fields using the given prefix.
func Env(prefix string) Source {
	return &envSource{prefix: prefix}
}

func (s *envSource) Name() string {
	return "env:" + s.prefix
}

func (s *envSource) Load(ctx context.Context, schema Schema) (map[string]any, error) {
	if s.prefix == "" {
		return map[string]any{}, nil
	}

	out := map[string]any{}
	for _, field := range schema.Fields() {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		envKey := s.prefix + envName(field.Path)
		value, exists := os.LookupEnv(envKey)
		if !exists {
			continue
		}
		parsed, err := schema.parseEnvValue(field, value)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", envKey, err)
		}
		setByPath(out, field.Path, parsed)
	}

	return out, nil
}

func (s Schema) parseEnvValue(field Field, raw string) (any, error) {
	if _, ok := s.codecs[field.Type]; ok {
		return raw, nil
	}
	typ := field.Type
	if typ.Kind() != reflect.Slice && typ.Kind() != reflect.Map {
		return raw, nil
	}
	var value any
	decoder := json.NewDecoder(strings.NewReader(raw))
	if err := decoder.Decode(&value); err != nil {
		return nil, fmt.Errorf("parse %s as JSON %s: %w", field.Path, typ, err)
	}
	if err := ensureEnvJSONEOF(decoder); err != nil {
		return nil, fmt.Errorf("parse %s as JSON %s: %w", field.Path, typ, err)
	}
	if typ.Kind() == reflect.Slice {
		if _, ok := value.([]any); !ok {
			return nil, fmt.Errorf("%s must be a JSON array", field.Path)
		}
	} else if _, ok := value.(map[string]any); !ok {
		return nil, fmt.Errorf("%s must be a JSON object", field.Path)
	}
	return value, nil
}

func ensureEnvJSONEOF(decoder *json.Decoder) error {
	var trailing any
	err := decoder.Decode(&trailing)
	if errors.Is(err, io.EOF) {
		return nil
	}
	if err != nil {
		return err
	}
	return errors.New("must contain exactly one JSON value")
}

func envName(path string) string {
	return strings.ToUpper(strings.NewReplacer(".", "_", "-", "_").Replace(path))
}
