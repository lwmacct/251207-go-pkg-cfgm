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

// SourceOption is a built-in source that can be passed directly to Load.
type SourceOption interface {
	Source
	Option
}

// Option configures a Load operation.
type Option interface {
	applyLoadOption(options *loadOptions)
}

type optionFunc func(*loadOptions)

func (f optionFunc) applyLoadOption(options *loadOptions) {
	f(options)
}

type loadOptions struct {
	sources           []Source
	logger            *slog.Logger
	noTemplates       bool
	strictUnknownKeys bool
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
		logger:            nil,
		expandDefaults:    true,
		strictUnknownKeys: true,
	}
}

// Load applies options and decodes the final config value.
func Load[T any](ctx context.Context, defaultConfig T, opts ...Option) (*T, error) {
	cfg, _, err := LoadReport(ctx, defaultConfig, opts...)

	return cfg, err
}

// MustLoad is like Load but panics on error.
func MustLoad[T any](ctx context.Context, defaultConfig T, opts ...Option) *T {
	cfg, err := Load(ctx, defaultConfig, opts...)
	if err != nil {
		panic(fmt.Sprintf("cfgm: failed to load config: %v", err))
	}

	return cfg
}

// LoadReport is like Load and also returns source contribution metadata.
func LoadReport[T any](ctx context.Context, defaultConfig T, opts ...Option) (*T, *Report, error) {
	options := loadOptions{strictUnknownKeys: true}
	for _, opt := range opts {
		if opt != nil {
			opt.applyLoadOption(&options)
		}
	}

	loader := New(defaultConfig)
	loader.logger = options.logger
	loader.expandDefaults = !options.noTemplates
	loader.strictUnknownKeys = options.strictUnknownKeys
	loader.Add(options.sources...)
	if options.noTemplates {
		disableTemplateExpansion(loader.sources)
	}

	return loader.Load(ctx)
}

// Logger sets the logger used for debug diagnostics.
func Logger(logger *slog.Logger) Option {
	return optionFunc(func(options *loadOptions) {
		options.logger = logger
	})
}

// Use adds a custom source to Load.
func Use(source Source) Option {
	return optionFunc(func(options *loadOptions) {
		if source != nil {
			options.sources = append(options.sources, source)
		}
	})
}

// ExpandDefaultTemplates enables template expansion for strings in defaultConfig.
//
// Template expansion is enabled by default; this option is mainly useful after
// NoTemplateExpansion when composing options.
func ExpandDefaultTemplates() Option {
	return optionFunc(func(options *loadOptions) {
		options.noTemplates = false
	})
}

// NoTemplateExpansion disables ${...} expansion in defaults and built-in file sources.
func NoTemplateExpansion() Option {
	return optionFunc(func(options *loadOptions) {
		options.noTemplates = true
	})
}

// AllowUnknownKeys disables source key validation against the config schema.
func AllowUnknownKeys() Option {
	return optionFunc(func(options *loadOptions) {
		options.strictUnknownKeys = false
	})
}

// Add appends sources to the loader. Later sources have higher priority.
func (l *Loader[T]) Add(sources ...Source) *Loader[T] {
	l.sources = append(l.sources, sources...)

	return l
}

// ExpandDefaults enables template expansion for string values in defaultConfig.
func (l *Loader[T]) ExpandDefaults() *Loader[T] {
	l.expandDefaults = true

	return l
}

// DisableTemplateExpansion disables template expansion in defaults and built-in file sources.
func (l *Loader[T]) DisableTemplateExpansion() *Loader[T] {
	l.expandDefaults = false
	disableTemplateExpansion(l.sources)

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

type templateExpansionSource interface {
	setTemplateExpansion(enabled bool)
}

func disableTemplateExpansion(sources []Source) {
	for _, source := range sources {
		if configurable, ok := source.(templateExpansionSource); ok {
			configurable.setTemplateExpansion(false)
		}
	}
}
