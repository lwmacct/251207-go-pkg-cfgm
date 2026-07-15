// Package cfgm provides Schema-driven configuration for Go applications.
//
// A Definition owns defaults, validation, codecs, file and environment
// loading, generated urfave/cli flags, and config examples:
//
//	definition := cfgm.New(DefaultConfig(), cfgm.AppName("app"))
//	config, err := definition.Load(ctx, cfgm.Env("APP_"))
//
// Later sources replace earlier values. Definition.Load searches optional
// DefaultPaths before caller-provided sources unless WithoutDefaultPaths is
// set. Unknown keys are rejected by default.
//
// # CLI Integration
//
// RootFlags returns the root --config/-c and --env-prefix/-e flags. Command
// declares the CLI command lineage whose matching config subtree Bind exposes:
//
//	binding := definition.Bind(
//	    cfgm.Command("server"),
//	    cfgm.Alias("addr", "a"),
//	    cfgm.NoCLI("redis.password"),
//	)
//
//	command := &cli.Command{
//	    Name:  "server",
//	    Flags: binding.Flags(),
//	    Action: func(ctx context.Context, cmd *cli.Command) error {
//	        config, err := binding.Load(ctx, cmd)
//	        return run(ctx, config, err)
//	    },
//	}
//
// Command paths map directly to json-tagged config structs. The example maps
// Config.Server to the server command, so option paths are relative to server.
// Binding.Load verifies the actual urfave command lineage and then applies
// defaults, default paths, an explicit config file, the selected environment
// prefix, and explicitly set CLI flags in that order.
//
// # Composite Values
//
// Environment slices and maps use complete JSON values. Scalar slices use
// urfave's repeatable typed flags. Struct slices use repeatable strict JSON
// objects; [] explicitly clears the collection and cannot be mixed with object
// occurrences. A CLI collection replaces lower-priority sources as a whole.
//
// WithCodec registers parsing for custom leaf types across files, environment
// variables, and CLI flags. ConfigFiles validates runtime files with the same
// Definition Schema used by loading.
package cfgm
