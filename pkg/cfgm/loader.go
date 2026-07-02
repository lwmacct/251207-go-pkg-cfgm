package cfgm

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"

	"github.com/lwmacct/251207-go-pkg-cfgm/pkg/templexp"
)

// Source loads one layer of configuration data.
//
// Sources are applied in the order they are added to a Loader. Later sources
// override earlier sources.
type Source interface {
	Name() string
	Load(ctx context.Context, schema ConfigSchema) (map[string]any, error)
}

// SourceReport records the data contributed by one source.
type SourceReport struct {
	Name string
	Keys []string
}

// Report describes how a Loader built the final configuration.
type Report struct {
	Sources []SourceReport
}

// Loader loads a config value from explicit sources.
type Loader[T any] struct {
	defaults          T
	schema            ConfigSchema
	sources           []Source
	logger            *slog.Logger
	expandDefaults    bool
	strictUnknownKeys bool
}

// New creates a Loader using defaultConfig as the lowest-priority layer.
func New[T any](defaultConfig T) *Loader[T] {
	return &Loader[T]{
		defaults:          defaultConfig,
		schema:            Schema(defaultConfig),
		logger:            slog.Default(),
		strictUnknownKeys: true,
	}
}

// Add appends sources to the loader. Later sources have higher priority.
func (l *Loader[T]) Add(sources ...Source) *Loader[T] {
	l.sources = append(l.sources, sources...)

	return l
}

// WithLogger sets the logger used for debug diagnostics.
func (l *Loader[T]) WithLogger(logger *slog.Logger) *Loader[T] {
	if logger == nil {
		logger = slog.Default()
	}
	l.logger = logger

	return l
}

// ExpandDefaults enables template expansion for string values in defaultConfig.
func (l *Loader[T]) ExpandDefaults() *Loader[T] {
	l.expandDefaults = true

	return l
}

// AllowUnknownKeys disables source key validation against the config schema.
func (l *Loader[T]) AllowUnknownKeys() *Loader[T] {
	l.strictUnknownKeys = false

	return l
}

// Load applies the configured sources and decodes the final config value.
func (l *Loader[T]) Load(ctx context.Context) (*T, *Report, error) {
	if ctx == nil {
		return nil, nil, errors.New("cfgm: nil context")
	}
	if l.logger == nil {
		l.logger = slog.Default()
	}

	configMap := structToMap(l.defaults)
	if l.expandDefaults {
		if _, err := expandTemplateValues(configMap); err != nil {
			return nil, nil, fmt.Errorf("expand template in defaults: %w", err)
		}
	}

	report := &Report{}
	for _, source := range l.sources {
		if err := ctx.Err(); err != nil {
			return nil, report, err
		}
		if source == nil {
			continue
		}

		sourceMap, err := source.Load(ctx, l.schema)
		if err != nil {
			return nil, report, fmt.Errorf("%s: %w", source.Name(), err)
		}
		keys := flattenMapKeys(sourceMap)
		slices.Sort(keys)
		if l.strictUnknownKeys {
			if err := l.schema.validateKeys(keys); err != nil {
				return nil, report, fmt.Errorf("%s: %w", source.Name(), err)
			}
		}
		mergeMaps(configMap, sourceMap)
		report.Sources = append(report.Sources, SourceReport{
			Name: source.Name(),
			Keys: keys,
		})
		l.logger.Debug("Loaded config source", "source", source.Name(), "keys", len(keys))
	}

	var cfg T
	if err := decodeConfigMap(configMap, &cfg); err != nil {
		return nil, report, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return &cfg, report, nil
}

func expandTemplateValues(value any) (any, error) {
	switch typed := value.(type) {
	case map[string]any:
		for key, item := range typed {
			expanded, err := expandTemplateValues(item)
			if err != nil {
				return nil, err
			}
			typed[key] = expanded
		}
		return typed, nil
	case []any:
		for i, item := range typed {
			expanded, err := expandTemplateValues(item)
			if err != nil {
				return nil, err
			}
			typed[i] = expanded
		}
		return typed, nil
	case string:
		if !containsTemplateMarker(typed) {
			return typed, nil
		}
		expanded, err := templexp.ExpandTemplate(typed)
		if err != nil {
			return nil, err
		}
		return expanded, nil
	default:
		return value, nil
	}
}

func containsTemplateMarker(value string) bool {
	for idx := range len(value) - 1 {
		if value[idx] == '$' && value[idx+1] == '{' {
			return true
		}
	}

	return false
}
