package cfgm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"maps"
	"reflect"
	"slices"
	"strings"
	"time"

	"github.com/go-viper/mapstructure/v2"
	"github.com/urfave/cli/v3"
)

type DefinitionOption interface {
	applyDefinition(options *definitionOptions)
}

type definitionOptionFunc func(*definitionOptions)

func (f definitionOptionFunc) applyDefinition(options *definitionOptions) {
	f(options)
}

type definitionOptions struct {
	appName          string
	codecs           map[reflect.Type]valueCodec
	defaultPaths     bool
	expandTemplates  bool
	allowUnknownKeys bool
	logger           *slog.Logger
}

func AppName(name string) DefinitionOption {
	return definitionOptionFunc(func(options *definitionOptions) {
		options.appName = strings.TrimSpace(name)
	})
}

func WithoutDefaultPaths() DefinitionOption {
	return definitionOptionFunc(func(options *definitionOptions) {
		options.defaultPaths = false
	})
}

func WithoutTemplateExpansion() DefinitionOption {
	return definitionOptionFunc(func(options *definitionOptions) {
		options.expandTemplates = false
	})
}

func AllowUnknownKeys() DefinitionOption {
	return definitionOptionFunc(func(options *definitionOptions) {
		options.allowUnknownKeys = true
	})
}

func Logger(logger *slog.Logger) DefinitionOption {
	if logger == nil {
		panic("cfgm: logger must not be nil")
	}
	return definitionOptionFunc(func(options *definitionOptions) {
		options.logger = logger
	})
}

type Codec[T any] struct {
	Parse  func(string) (T, error)
	Format func(T) string
}

func WithCodec[T any](codec Codec[T]) DefinitionOption {
	if codec.Parse == nil {
		panic(fmt.Sprintf("cfgm: codec for %s requires Parse", reflect.TypeFor[T]()))
	}
	return definitionOptionFunc(func(options *definitionOptions) {
		if options.codecs == nil {
			options.codecs = make(map[reflect.Type]valueCodec)
		}
		typ := reflect.TypeFor[T]()
		options.codecs[typ] = valueCodec{
			parse: func(value string) (any, error) { return codec.Parse(value) },
			format: func(value any) string {
				if codec.Format == nil {
					return fmt.Sprint(value)
				}
				typed, ok := value.(T)
				if !ok {
					return fmt.Sprint(value)
				}
				return codec.Format(typed)
			},
		}
	})
}

type valueCodec struct {
	parse  func(string) (any, error)
	format func(any) string
}

type Definition[T any] struct {
	defaults          T
	appName           string
	schema            *schemaModel
	codecs            map[reflect.Type]valueCodec
	defaultPaths      bool
	expandTemplates   bool
	strictUnknownKeys bool
	logger            *slog.Logger
}

func New[T any](defaults T, opts ...DefinitionOption) *Definition[T] {
	options := definitionOptions{defaultPaths: true, expandTemplates: true, logger: slog.Default()}
	for _, opt := range opts {
		if opt != nil {
			opt.applyDefinition(&options)
		}
	}
	return &Definition[T]{
		defaults:          defaults,
		appName:           options.appName,
		schema:            buildSchemaModel(reflect.TypeFor[T](), options.codecs),
		codecs:            mapsClone(options.codecs),
		defaultPaths:      options.defaultPaths,
		expandTemplates:   options.expandTemplates,
		strictUnknownKeys: !options.allowUnknownKeys,
		logger:            options.logger,
	}
}

func (d *Definition[T]) Load(ctx context.Context, sources ...Source) (*T, error) {
	config, _, err := d.LoadReport(ctx, sources...)
	return config, err
}

func (d *Definition[T]) MustLoad(ctx context.Context, sources ...Source) *T {
	config, err := d.Load(ctx, sources...)
	if err != nil {
		panic(fmt.Sprintf("cfgm: failed to load config: %v", err))
	}
	return config
}

func (d *Definition[T]) LoadReport(ctx context.Context, sources ...Source) (*T, *Report, error) {
	loader := d.loader()
	if d.defaultPaths {
		loader.sources = append(loader.sources, Files(DefaultPaths(d.appName), Optional()))
	}
	loader.sources = append(loader.sources, sources...)
	return loader.load(ctx)
}

func (d *Definition[T]) loader() *configLoader[T] {
	return &configLoader[T]{
		defaults:          d.defaults,
		schema:            d.schema,
		logger:            d.logger,
		expandDefaults:    d.expandTemplates,
		strictUnknownKeys: d.strictUnknownKeys,
		codecs:            d.codecs,
	}
}

type BindOption interface {
	applyBinding(options *bindingOptions)
}

type bindingOptionFunc func(*bindingOptions)

func (f bindingOptionFunc) applyBinding(options *bindingOptions) {
	f(options)
}

type bindingOptions struct {
	scope    string
	includes []string
	aliases  map[string][]string
	noCLI    map[string]bool
}

func Scope(path string) BindOption {
	return bindingOptionFunc(func(options *bindingOptions) {
		options.scope = cleanConfigPath(path)
	})
}

func Include(paths ...string) BindOption {
	return bindingOptionFunc(func(options *bindingOptions) {
		for _, path := range paths {
			if path = cleanConfigPath(path); path != "" {
				options.includes = append(options.includes, path)
			}
		}
	})
}

func Alias(path string, aliases ...string) BindOption {
	return bindingOptionFunc(func(options *bindingOptions) {
		if options.aliases == nil {
			options.aliases = make(map[string][]string)
		}
		path = cleanConfigPath(path)
		for _, alias := range aliases {
			if alias = strings.TrimSpace(alias); alias != "" {
				options.aliases[path] = append(options.aliases[path], alias)
			}
		}
	})
}

func NoCLI(paths ...string) BindOption {
	return bindingOptionFunc(func(options *bindingOptions) {
		if options.noCLI == nil {
			options.noCLI = make(map[string]bool)
		}
		for _, path := range paths {
			if path = cleanConfigPath(path); path != "" {
				options.noCLI[path] = true
			}
		}
	})
}

type Binding[T any] struct {
	definition *Definition[T]
	scope      string
	fields     []boundField
}

type boundField struct {
	field schemaField
	name  string
}

func (d *Definition[T]) Bind(opts ...BindOption) *Binding[T] {
	options := bindingOptions{}
	for _, opt := range opts {
		if opt != nil {
			opt.applyBinding(&options)
		}
	}
	d.validateBindingOptions(options)

	fields := make([]boundField, 0, len(d.schema.fields))
	seenNames := make(map[string]string)
	for _, field := range d.schema.fields {
		if bindingExcluded(field.path, options.noCLI) || !bindingIncludes(field.path, options.scope, options.includes) {
			continue
		}
		name := bindingFlagName(field.path, options.scope)
		if isReservedFlagName(name) {
			panic(fmt.Errorf("cfgm: generated CLI flag --%s is reserved by root flags", name))
		}
		field.aliases = append([]string(nil), options.aliases[field.path]...)
		for _, flagName := range append([]string{name}, field.aliases...) {
			if previous, exists := seenNames[flagName]; exists {
				panic(fmt.Errorf("cfgm: CLI flag --%s is ambiguous: matches %s and %s", flagName, previous, field.path))
			}
			seenNames[flagName] = field.path
		}
		fields = append(fields, boundField{field: field, name: name})
	}
	slices.SortFunc(fields, func(a, b boundField) int { return strings.Compare(a.name, b.name) })
	return &Binding[T]{definition: d, scope: options.scope, fields: fields}
}

func (d *Definition[T]) validateBindingOptions(options bindingOptions) {
	if options.scope != "" && !d.schema.isStructPath(options.scope) {
		panic(fmt.Errorf("cfgm: scope %q is not a config struct path", options.scope))
	}
	for _, path := range options.includes {
		if !d.schema.isFieldPath(path) && !d.schema.isStructPath(path) {
			panic(fmt.Errorf("cfgm: include path %q does not select CLI fields", path))
		}
	}
	for path := range options.noCLI {
		if !d.schema.isFieldPath(path) && !d.schema.isStructPath(path) {
			panic(fmt.Errorf("cfgm: no-CLI path %q does not select CLI fields", path))
		}
	}
	for path, aliases := range options.aliases {
		if !d.schema.isFieldPath(path) {
			panic(fmt.Errorf("cfgm: alias path %q is not a CLI config field", path))
		}
		if !bindingIncludes(path, options.scope, options.includes) || bindingExcluded(path, options.noCLI) {
			panic(fmt.Errorf("cfgm: alias path %q is excluded from this binding", path))
		}
		for _, alias := range aliases {
			if isReservedFlagName(alias) {
				panic(fmt.Errorf("cfgm: alias --%s is reserved by root flags", alias))
			}
		}
	}
}

func isReservedFlagName(name string) bool {
	return name == configFlagName || name == envPrefixFlagName ||
		name == "c" || name == "e" || name == "help" || name == "h"
}

func (b *Binding[T]) Flags() []cli.Flag {
	flags := make([]cli.Flag, 0, len(b.fields))
	for _, field := range b.fields {
		flag, err := b.newFlag(field)
		if err != nil {
			panic(err)
		}
		flags = append(flags, flag)
	}
	return flags
}

func (b *Binding[T]) Load(ctx context.Context, cmd *cli.Command) (*T, error) {
	config, _, err := b.LoadReport(ctx, cmd)
	return config, err
}

func (b *Binding[T]) MustLoad(ctx context.Context, cmd *cli.Command) *T {
	config, err := b.Load(ctx, cmd)
	if err != nil {
		panic(fmt.Sprintf("cfgm: failed to load config: %v", err))
	}
	return config
}

func (b *Binding[T]) LoadReport(ctx context.Context, cmd *cli.Command) (*T, *Report, error) {
	if ctx == nil {
		return nil, nil, errors.New("cfgm: nil context")
	}
	loader := b.definition.loader()

	appName := b.definition.appName
	if appName == "" {
		appName = commandRootName(cmd)
	}
	if b.definition.defaultPaths {
		loader.sources = append(loader.sources, Files(DefaultPaths(appName), Optional()))
	}
	if configPath := commandConfigPath(cmd); configPath != "" {
		loader.sources = append(loader.sources, File(configPath))
	}
	if prefix, ok := commandEnvPrefix(cmd); ok {
		if prefix != "" {
			loader.sources = append(loader.sources, Env(prefix))
		}
	} else if appName != "" {
		loader.sources = append(loader.sources, Env(strings.ToUpper(strings.ReplaceAll(appName, "-", "_"))+"_"))
	}
	loader.sources = append(loader.sources, &bindingCLISource[T]{binding: b, cmd: cmd})
	return loader.load(ctx)
}

func (b *Binding[T]) newFlag(bound boundField) (cli.Flag, error) {
	field := bound.field
	aliases := append([]string(nil), field.aliases...)
	usage := field.desc
	defaultValue, ok := valueAtPath(reflect.ValueOf(b.definition.defaults), field.index)
	if !ok {
		defaultValue = reflect.Zero(field.typ)
	}

	if codec, ok := b.definition.codecs[field.typ]; ok {
		value := ""
		if defaultValue.IsValid() {
			value = codec.format(defaultValue.Interface())
		}
		return &cli.StringFlag{Name: bound.name, Aliases: aliases, Usage: usage, Value: value}, nil
	}
	if field.kind == schemaFieldStructSlice {
		return b.newStructSliceFlag(bound, defaultValue), nil
	}

	value := defaultValue.Interface()
	switch field.typ {
	case durationType:
		return &cli.DurationFlag{
			Name: bound.name, Aliases: aliases, Usage: usage, Value: reflectAs[time.Duration](defaultValue),
		}, nil
	case timeType:
		return &cli.TimestampFlag{
			Name: bound.name, Aliases: aliases, Usage: usage, Value: reflectAs[time.Time](defaultValue),
			Config: cli.TimestampConfig{Layouts: []string{time.RFC3339Nano, time.RFC3339}},
		}, nil
	}
	switch field.typ.Kind() { //nolint:exhaustive // unsupported config fields fail binding construction
	case reflect.String:
		return &cli.StringFlag{Name: bound.name, Aliases: aliases, Usage: usage, Value: defaultValue.String()}, nil
	case reflect.Bool:
		return &cli.BoolFlag{Name: bound.name, Aliases: aliases, Usage: usage, Value: defaultValue.Bool()}, nil
	case reflect.Int:
		return &cli.IntFlag{Name: bound.name, Aliases: aliases, Usage: usage, Value: int(defaultValue.Int())}, nil
	case reflect.Int8:
		return &cli.Int8Flag{Name: bound.name, Aliases: aliases, Usage: usage, Value: reflectAs[int8](defaultValue)}, nil
	case reflect.Int16:
		return &cli.Int16Flag{Name: bound.name, Aliases: aliases, Usage: usage, Value: reflectAs[int16](defaultValue)}, nil
	case reflect.Int32:
		return &cli.Int32Flag{Name: bound.name, Aliases: aliases, Usage: usage, Value: reflectAs[int32](defaultValue)}, nil
	case reflect.Int64:
		return &cli.Int64Flag{Name: bound.name, Aliases: aliases, Usage: usage, Value: defaultValue.Int()}, nil
	case reflect.Uint:
		return &cli.UintFlag{Name: bound.name, Aliases: aliases, Usage: usage, Value: uint(defaultValue.Uint())}, nil
	case reflect.Uint8:
		return &cli.Uint8Flag{Name: bound.name, Aliases: aliases, Usage: usage, Value: reflectAs[uint8](defaultValue)}, nil
	case reflect.Uint16:
		return &cli.Uint16Flag{Name: bound.name, Aliases: aliases, Usage: usage, Value: reflectAs[uint16](defaultValue)}, nil
	case reflect.Uint32:
		return &cli.Uint32Flag{Name: bound.name, Aliases: aliases, Usage: usage, Value: reflectAs[uint32](defaultValue)}, nil
	case reflect.Uint64:
		return &cli.Uint64Flag{Name: bound.name, Aliases: aliases, Usage: usage, Value: defaultValue.Uint()}, nil
	case reflect.Float32:
		return &cli.Float32Flag{Name: bound.name, Aliases: aliases, Usage: usage, Value: float32(defaultValue.Float())}, nil
	case reflect.Float64:
		return &cli.Float64Flag{Name: bound.name, Aliases: aliases, Usage: usage, Value: defaultValue.Float()}, nil
	case reflect.Slice:
		return newScalarSliceFlag(bound.name, aliases, usage, field.typ, value)
	case reflect.Map:
		if field.typ.Key().Kind() == reflect.String && field.typ.Elem().Kind() == reflect.String {
			return &cli.StringMapFlag{
				Name: bound.name, Aliases: aliases, Usage: usage, Value: stringMapAs(value),
			}, nil
		}
	}
	return nil, fmt.Errorf("cfgm: config field %s has unsupported CLI type %s", field.path, field.typ)
}

func (b *Binding[T]) newStructSliceFlag(bound boundField, defaultValue reflect.Value) cli.Flag {
	defaultText := "[]"
	if defaultValue.IsValid() {
		encoded, err := json.Marshal(defaultValue.Interface())
		if err == nil {
			defaultText = string(encoded)
		}
	}
	return &cli.GenericFlag{
		Name:        bound.name,
		Aliases:     append([]string(nil), bound.field.aliases...),
		Usage:       bound.field.desc,
		DefaultText: defaultText,
		Value:       newStructSliceValue(bound.field.typ, b.definition.codecs),
	}
}

func reflectAs[T any](value reflect.Value) T {
	converted := value.Convert(reflect.TypeFor[T]()).Interface()
	typed, ok := converted.(T)
	if !ok {
		panic(fmt.Errorf("cfgm: cannot convert %s to %s", value.Type(), reflect.TypeFor[T]()))
	}
	return typed
}

func newScalarSliceFlag(name string, aliases []string, usage string, typ reflect.Type, value any) (cli.Flag, error) {
	switch typ.Elem().Kind() { //nolint:exhaustive // unsupported slice elements return an error
	case reflect.String:
		return &cli.StringSliceFlag{Name: name, Aliases: aliases, Usage: usage, Value: sliceAs[string](value)}, nil
	case reflect.Int:
		return &cli.IntSliceFlag{Name: name, Aliases: aliases, Usage: usage, Value: sliceAs[int](value)}, nil
	case reflect.Int8:
		return &cli.Int8SliceFlag{Name: name, Aliases: aliases, Usage: usage, Value: sliceAs[int8](value)}, nil
	case reflect.Int16:
		return &cli.Int16SliceFlag{Name: name, Aliases: aliases, Usage: usage, Value: sliceAs[int16](value)}, nil
	case reflect.Int32:
		return &cli.Int32SliceFlag{Name: name, Aliases: aliases, Usage: usage, Value: sliceAs[int32](value)}, nil
	case reflect.Int64:
		return &cli.Int64SliceFlag{Name: name, Aliases: aliases, Usage: usage, Value: sliceAs[int64](value)}, nil
	case reflect.Uint16:
		return &cli.Uint16SliceFlag{Name: name, Aliases: aliases, Usage: usage, Value: sliceAs[uint16](value)}, nil
	case reflect.Uint32:
		return &cli.Uint32SliceFlag{Name: name, Aliases: aliases, Usage: usage, Value: sliceAs[uint32](value)}, nil
	case reflect.Uint:
		return &cli.UintSliceFlag{Name: name, Aliases: aliases, Usage: usage, Value: sliceAs[uint](value)}, nil
	case reflect.Uint8:
		return &cli.Uint8SliceFlag{Name: name, Aliases: aliases, Usage: usage, Value: sliceAs[uint8](value)}, nil
	case reflect.Uint64:
		return &cli.Uint64SliceFlag{Name: name, Aliases: aliases, Usage: usage, Value: sliceAs[uint64](value)}, nil
	case reflect.Float32:
		return &cli.Float32SliceFlag{Name: name, Aliases: aliases, Usage: usage, Value: sliceAs[float32](value)}, nil
	case reflect.Float64:
		return &cli.Float64SliceFlag{Name: name, Aliases: aliases, Usage: usage, Value: sliceAs[float64](value)}, nil
	}
	return nil, fmt.Errorf("cfgm: config field --%s has unsupported slice type %s", name, typ)
}

func sliceAs[T any](value any) []T {
	input := reflect.ValueOf(value)
	out := make([]T, input.Len())
	for index := range input.Len() {
		out[index] = reflectAs[T](input.Index(index))
	}
	return out
}

func stringMapAs(value any) map[string]string {
	input := reflect.ValueOf(value)
	out := make(map[string]string, input.Len())
	iterator := input.MapRange()
	for iterator.Next() {
		out[iterator.Key().Convert(reflect.TypeFor[string]()).String()] =
			iterator.Value().Convert(reflect.TypeFor[string]()).String()
	}
	return out
}

type bindingCLISource[T any] struct {
	binding *Binding[T]
	cmd     *cli.Command
}

func (s *bindingCLISource[T]) Name() string { return "cli" }

func (s *bindingCLISource[T]) Load(ctx context.Context, _ Schema) (map[string]any, error) {
	if s.cmd == nil {
		return map[string]any{}, nil
	}
	out := map[string]any{}
	for _, bound := range s.binding.fields {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if !s.cmd.IsSet(bound.name) {
			continue
		}
		value, err := s.binding.flagValue(s.cmd, bound)
		if err != nil {
			return nil, fmt.Errorf("--%s: %w", bound.name, err)
		}
		setByPath(out, bound.field.path, value)
	}
	return out, nil
}

func (b *Binding[T]) flagValue(cmd *cli.Command, bound boundField) (any, error) {
	if bound.field.kind == schemaFieldStructSlice {
		value, ok := cmd.Value(bound.name).(*structSliceValue)
		if !ok || value == nil {
			return nil, errors.New("invalid structured flag value")
		}
		return value.configValue(), nil
	}
	if _, ok := b.definition.codecs[bound.field.typ]; ok {
		return cmd.String(bound.name), nil
	}
	if bound.field.typ.Kind() == reflect.Slice {
		return scalarSliceFlagValue(cmd, bound.name, bound.field.typ)
	}
	if bound.field.typ.Kind() == reflect.Map &&
		bound.field.typ.Key().Kind() == reflect.String && bound.field.typ.Elem().Kind() == reflect.String {
		return stringMapToAny(cmd.StringMap(bound.name)), nil
	}
	return cmd.Value(bound.name), nil
}

func scalarSliceFlagValue(cmd *cli.Command, name string, typ reflect.Type) (any, error) {
	switch typ.Elem().Kind() { //nolint:exhaustive // unsupported slices fail during flag construction
	case reflect.String:
		return sliceToAny(cmd.StringSlice(name)), nil
	case reflect.Int:
		return sliceToAny(cmd.IntSlice(name)), nil
	case reflect.Int8:
		return sliceToAny(cmd.Int8Slice(name)), nil
	case reflect.Int16:
		return sliceToAny(cmd.Int16Slice(name)), nil
	case reflect.Int32:
		return sliceToAny(cmd.Int32Slice(name)), nil
	case reflect.Int64:
		return sliceToAny(cmd.Int64Slice(name)), nil
	case reflect.Uint16:
		return sliceToAny(cmd.Uint16Slice(name)), nil
	case reflect.Uint32:
		return sliceToAny(cmd.Uint32Slice(name)), nil
	case reflect.Uint:
		return sliceToAny(cmd.UintSlice(name)), nil
	case reflect.Uint8:
		return sliceToAny(cmd.Uint8Slice(name)), nil
	case reflect.Uint64:
		return sliceToAny(cmd.Uint64Slice(name)), nil
	case reflect.Float32:
		return sliceToAny(cmd.Float32Slice(name)), nil
	case reflect.Float64:
		return sliceToAny(cmd.Float64Slice(name)), nil
	}
	return nil, fmt.Errorf("unsupported slice type %s", typ)
}

func sliceToAny[T any](values []T) []any {
	out := make([]any, len(values))
	for index, value := range values {
		out[index] = value
	}
	return out
}

func stringMapToAny(values map[string]string) map[string]any {
	out := make(map[string]any, len(values))
	for key, value := range values {
		out[key] = value
	}
	return out
}

type schemaFieldKind uint8

const (
	schemaFieldScalar schemaFieldKind = iota
	schemaFieldStructSlice
)

type schemaField struct {
	path    string
	typ     reflect.Type
	desc    string
	index   []int
	kind    schemaFieldKind
	aliases []string
}

type schemaModel struct {
	rootType reflect.Type
	fields   []schemaField
	paths    map[string]reflect.Type
	structs  map[string]bool
	fieldSet map[string]bool
	codecs   map[reflect.Type]valueCodec
	active   map[reflect.Type]bool
}

func buildSchemaModel(typ reflect.Type, codecs map[reflect.Type]valueCodec) *schemaModel {
	if typ == nil {
		panic("cfgm: config root must be a struct, got nil")
	}
	if typ.Kind() != reflect.Struct {
		panic(fmt.Errorf("cfgm: config root must be a struct, got %s", typ))
	}
	model := &schemaModel{
		rootType: typ,
		paths:    make(map[string]reflect.Type),
		structs:  make(map[string]bool),
		fieldSet: make(map[string]bool),
		codecs:   codecs,
		active:   make(map[reflect.Type]bool),
	}
	model.collect(typ, "", nil)
	model.validateEnvironmentNames()
	return model
}

func (m *schemaModel) collect(typ reflect.Type, prefix string, parentIndex []int) {
	typ = normalizeStructType(typ)
	m.enterType(typ)
	defer m.leaveType(typ)
	for field := range typ.Fields() {
		if field.PkgPath != "" {
			continue
		}
		key := configTagName(field)
		if key == "" {
			continue
		}
		m.validateNewPath(prefix, key)
		path := joinSchemaPath(prefix, key)
		index := append(append([]int(nil), parentIndex...), field.Index...)
		m.paths[path] = field.Type
		_, hasCodec := m.codecs[field.Type]
		if isStructType(field.Type) && !hasCodec {
			m.structs[path] = true
			m.collect(field.Type, path, index)
			continue
		}
		kind := schemaFieldScalar
		if isStructSlice(field.Type) {
			kind = schemaFieldStructSlice
			m.collectCompositePaths(field.Type, path)
		}
		m.fields = append(m.fields, schemaField{path: path, typ: field.Type, desc: field.Tag.Get("desc"), index: index, kind: kind})
		m.fieldSet[path] = true
	}
}

func (m *schemaModel) validateNewPath(prefix, key string) {
	if strings.Contains(key, ".") {
		panic(fmt.Errorf("cfgm: config key %q must not contain dots", key))
	}
	path := joinSchemaPath(prefix, key)
	if _, exists := m.paths[path]; exists {
		panic(fmt.Errorf("cfgm: duplicate config path %q", path))
	}
}

func (m *schemaModel) collectCompositePaths(typ reflect.Type, prefix string) {
	for typ.Kind() == reflect.Pointer || typ.Kind() == reflect.Slice || typ.Kind() == reflect.Array {
		typ = typ.Elem()
	}
	if typ.Kind() == reflect.Map {
		typ = typ.Elem()
		for typ.Kind() == reflect.Pointer {
			typ = typ.Elem()
		}
	}
	if typ.Kind() != reflect.Struct || typ == timeType {
		return
	}
	if _, hasCodec := m.codecs[typ]; hasCodec {
		return
	}
	m.enterType(typ)
	defer m.leaveType(typ)
	for field := range typ.Fields() {
		if field.PkgPath != "" {
			continue
		}
		key := configTagName(field)
		if key == "" {
			continue
		}
		m.validateNewPath(prefix, key)
		path := joinSchemaPath(prefix, key)
		m.paths[path] = field.Type
		m.collectCompositePaths(field.Type, path)
	}
}

func (m *schemaModel) enterType(typ reflect.Type) {
	if m.active[typ] {
		panic(fmt.Errorf("cfgm: recursive config type %s is not supported", typ))
	}
	m.active[typ] = true
}

func (m *schemaModel) leaveType(typ reflect.Type) { delete(m.active, typ) }

func (m *schemaModel) validateEnvironmentNames() {
	seen := make(map[string]string, len(m.fields))
	for _, field := range m.fields {
		name := envName(field.path)
		if previous, ok := seen[name]; ok {
			panic(fmt.Errorf("cfgm: config paths %q and %q map to the same environment name %s", previous, field.path, name))
		}
		seen[name] = field.path
	}
}

func (m *schemaModel) validateData(
	data map[string]any,
	codecs map[reflect.Type]valueCodec,
	allowUnknownKeys bool,
) error {
	var unknown []string
	if err := validateConfigValue(data, m.rootType, "", false, codecs, &unknown); err != nil {
		return err
	}
	if allowUnknownKeys || len(unknown) == 0 {
		return nil
	}
	slices.Sort(unknown)
	return fmt.Errorf("unknown config keys:\n  - %s", strings.Join(unknown, "\n  - "))
}

func validateConfigValue(
	value any,
	typ reflect.Type,
	path string,
	nullable bool,
	codecs map[reflect.Type]valueCodec,
	unknown *[]string,
) error {
	if typ.Kind() == reflect.Pointer {
		if value == nil {
			return nil
		}
		return validateConfigValue(value, typ.Elem(), path, true, codecs, unknown)
	}
	if value == nil {
		if nullable || typ.Kind() == reflect.Slice || typ.Kind() == reflect.Map {
			return nil
		}
		return fmt.Errorf("config key %q cannot be null", path)
	}
	if _, ok := codecs[typ]; ok {
		if _, stringValue := value.(string); !stringValue {
			return fmt.Errorf("config key %q must be a string for codec %s", path, typ)
		}
		return nil
	}
	if typ == durationType || typ == timeType {
		return nil
	}
	switch typ.Kind() { //nolint:exhaustive // scalar values need no structural validation
	case reflect.Struct:
		object, ok := value.(map[string]any)
		if !ok {
			return fmt.Errorf("config key %q must be an object", path)
		}
		fields := make(map[string]reflect.Type, typ.NumField())
		for field := range typ.Fields() {
			if field.PkgPath == "" {
				if key := configTagName(field); key != "" {
					fields[key] = field.Type
				}
			}
		}
		for key, child := range object {
			childPath := joinSchemaPath(path, key)
			fieldType, ok := fields[key]
			if !ok {
				*unknown = append(*unknown, childPath)
				continue
			}
			if err := validateConfigValue(child, fieldType, childPath, false, codecs, unknown); err != nil {
				return err
			}
		}
	case reflect.Slice, reflect.Array:
		items, ok := value.([]any)
		if !ok {
			return fmt.Errorf("config key %q must be an array", path)
		}
		for _, item := range items {
			if err := validateConfigValue(item, typ.Elem(), path, false, codecs, unknown); err != nil {
				return err
			}
		}
	case reflect.Map:
		object, ok := value.(map[string]any)
		if !ok {
			return fmt.Errorf("config key %q must be an object", path)
		}
		for key, child := range object {
			if err := validateConfigValue(child, typ.Elem(), joinSchemaPath(path, key), false, codecs, unknown); err != nil {
				return err
			}
		}
	}
	return nil
}

func (m *schemaModel) hasPath(path string) bool {
	if _, ok := m.paths[path]; ok {
		return true
	}
	for fieldPath, typ := range m.paths {
		if typ.Kind() == reflect.Map && strings.HasPrefix(path, fieldPath+".") {
			return true
		}
	}
	return false
}

func (m *schemaModel) isStructPath(path string) bool { return m.structs[path] }

func (m *schemaModel) isFieldPath(path string) bool { return m.fieldSet[path] }

func isStructSlice(typ reflect.Type) bool {
	if typ.Kind() != reflect.Slice {
		return false
	}
	elem := typ.Elem()
	if elem.Kind() == reflect.Pointer {
		elem = elem.Elem()
	}
	return elem.Kind() == reflect.Struct && elem != timeType && elem != durationType
}

type structSliceValue struct {
	typ     reflect.Type
	codecs  map[reflect.Type]valueCodec
	items   []map[string]any
	cleared bool
}

func newStructSliceValue(typ reflect.Type, codecs map[reflect.Type]valueCodec) *structSliceValue {
	return &structSliceValue{typ: typ, codecs: codecs}
}

func (v *structSliceValue) Set(raw string) error {
	if strings.TrimSpace(raw) == "[]" {
		if len(v.items) > 0 {
			return errors.New("clear value [] cannot be combined with structured values")
		}
		v.cleared = true
		return nil
	}
	if v.cleared {
		return errors.New("structured values cannot be combined with clear value []")
	}
	var item map[string]any
	decoder := json.NewDecoder(strings.NewReader(raw))
	targetType := v.typ.Elem()
	if targetType.Kind() == reflect.Pointer {
		targetType = targetType.Elem()
	}
	if err := decoder.Decode(&item); err != nil {
		return fmt.Errorf("parse JSON object: %w", err)
	}
	if item == nil {
		return errors.New("value must be a JSON object")
	}
	if err := ensureJSONEOF(decoder); err != nil {
		return err
	}
	if err := validateJSONObject(item, targetType, "", v.codecs); err != nil {
		return err
	}
	v.items = append(v.items, item)
	return nil
}

func (v *structSliceValue) String() string {
	if v.cleared {
		return "[]"
	}
	encoded, err := json.Marshal(v.items)
	if err != nil {
		return "[]"
	}
	return string(encoded)
}

func (v *structSliceValue) Get() any { return v }

func (v *structSliceValue) configValue() any {
	if v.cleared {
		return []any{}
	}
	items := make([]any, len(v.items))
	for index := range v.items {
		items[index] = v.items[index]
	}
	return items
}

type configLoader[T any] struct {
	defaults          T
	schema            *schemaModel
	sources           []Source
	logger            *slog.Logger
	expandDefaults    bool
	strictUnknownKeys bool
	codecs            map[reflect.Type]valueCodec
}

func (l *configLoader[T]) load(ctx context.Context) (*T, *Report, error) {
	if ctx == nil {
		return nil, nil, errors.New("cfgm: nil context")
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
		data, err := source.Load(ctx, Schema{model: l.schema, codecs: l.codecs, expandTemplates: l.expandDefaults})
		if err != nil {
			return nil, report, fmt.Errorf("%s: %w", source.Name(), err)
		}
		keys := flattenSchemaKeys(data)
		slices.Sort(keys)
		if err := l.schema.validateData(data, l.codecs, !l.strictUnknownKeys); err != nil {
			return nil, report, fmt.Errorf("%s: %w", source.Name(), err)
		}
		mergeMaps(configMap, data)
		report.Sources = append(report.Sources, SourceReport{Name: source.Name(), Keys: keys})
		l.logger.DebugContext(ctx, "Loaded config source", "source", source.Name(), "keys", keys)
	}
	var config T
	if err := decodeConfigMapWithCodecs(configMap, &config, l.codecs); err != nil {
		return nil, report, fmt.Errorf("failed to unmarshal config: %w", err)
	}
	return &config, report, nil
}

func decodeConfigMapWithCodecs(data map[string]any, out any, codecs map[reflect.Type]valueCodec) error {
	hooks := []mapstructure.DecodeHookFunc{
		func(from reflect.Type, to reflect.Type, value any) (any, error) {
			codec, ok := codecs[to]
			if !ok || from == nil || from.Kind() != reflect.String {
				return value, nil
			}
			text, ok := value.(string)
			if !ok {
				return value, nil
			}
			return codec.parse(text)
		},
		mapstructure.StringToTimeDurationHookFunc(),
		mapstructure.TextUnmarshallerHookFunc(),
	}
	decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		DecodeHook:       mapstructure.ComposeDecodeHookFunc(hooks...),
		Result:           out,
		WeaklyTypedInput: true,
		TagName:          "json",
	})
	if err != nil {
		return err
	}
	return decoder.Decode(data)
}

func cleanConfigPath(path string) string {
	return strings.Trim(strings.TrimSpace(path), ".")
}

func bindingIncludes(path, scope string, includes []string) bool {
	if scope == "" && len(includes) == 0 {
		return true
	}
	if pathWithin(path, scope) {
		return true
	}
	for _, include := range includes {
		if pathWithin(path, include) {
			return true
		}
	}
	return false
}

func bindingExcluded(path string, exclusions map[string]bool) bool {
	for exclusion := range exclusions {
		if pathWithin(path, exclusion) {
			return true
		}
	}
	return false
}

func pathWithin(path, prefix string) bool {
	return prefix != "" && (path == prefix || strings.HasPrefix(path, prefix+"."))
}

func bindingFlagName(path, scope string) string {
	if scope != "" {
		if name, ok := strings.CutPrefix(path, scope+"."); ok {
			return name
		}
	}
	return path
}

func joinSchemaPath(prefix, key string) string {
	if prefix == "" {
		return key
	}
	return prefix + "." + key
}

func valueAtPath(root reflect.Value, index []int) (reflect.Value, bool) {
	if root.Kind() == reflect.Pointer {
		if root.IsNil() {
			return reflect.Value{}, false
		}
		root = root.Elem()
	}
	for _, fieldIndex := range index {
		if root.Kind() == reflect.Pointer {
			if root.IsNil() {
				return reflect.Value{}, false
			}
			root = root.Elem()
		}
		root = root.Field(fieldIndex)
	}
	return root, true
}

func ensureJSONEOF(decoder *json.Decoder) error {
	var trailing any
	err := decoder.Decode(&trailing)
	if errors.Is(err, io.EOF) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("parse JSON object: %w", err)
	}
	return errors.New("JSON object must contain exactly one value")
}

func validateJSONObject(
	item map[string]any,
	typ reflect.Type,
	prefix string,
	codecs map[reflect.Type]valueCodec,
) error {
	var unknown []string
	if err := validateConfigValue(item, typ, prefix, false, codecs, &unknown); err != nil {
		return err
	}
	if len(unknown) > 0 {
		slices.Sort(unknown)
		return fmt.Errorf("unknown field %q", unknown[0])
	}
	return nil
}

func flattenSchemaKeys(data map[string]any) []string {
	var keys []string
	flattenSchemaValue(data, "", &keys)
	slices.Sort(keys)
	return slices.Compact(keys)
}

func flattenSchemaValue(value any, prefix string, keys *[]string) {
	switch typed := value.(type) {
	case map[string]any:
		if len(typed) == 0 && prefix != "" {
			*keys = append(*keys, prefix)
			return
		}
		for key, child := range typed {
			path := joinSchemaPath(prefix, key)
			flattenSchemaValue(child, path, keys)
		}
	case []any:
		if len(typed) == 0 {
			*keys = append(*keys, prefix)
			return
		}
		for _, child := range typed {
			flattenSchemaValue(child, prefix, keys)
		}
	default:
		*keys = append(*keys, prefix)
	}
}

func mapsClone[K comparable, V any](source map[K]V) map[K]V {
	if source == nil {
		return nil
	}
	out := make(map[K]V, len(source))
	maps.Copy(out, source)
	return out
}
