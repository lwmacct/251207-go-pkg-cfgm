// Package cfgm provides generic config loading for Go applications.
//
// Config structs use json tags as stable config keys. Callers explicitly
// declare sources with Load options, and later sources override earlier ones.
//
// # Basic Loading
//
//	cfg, err := cfgm.Load(ctx,
//	    DefaultConfig(),
//	    cfgm.File("config/config.yaml", cfgm.Optional()),
//	    cfgm.Env("APP_"),
//	)
//
// Load validates source keys against the config schema by default. Use
// AllowUnknownKeys when a source intentionally contains extra keys. Template
// expansion is enabled for defaults and built-in file sources by default; use
// NoTemplateExpansion to keep raw ${...} strings.
//
// # CLI Integration
//
// For urfave/cli commands, Command applies cfgm's standard CLI profile:
// explicitly set --config file, env prefix from --env-prefix or root command
// name, then explicitly set CLI flags.
//
//	func action(ctx context.Context, cmd *cli.Command) error {
//	    cfg := cfgm.MustLoad(ctx,
//	        DefaultConfig(),
//	        cfgm.Command(cmd),
//	    )
//	    return run(ctx, cfg)
//	}
//
// # Diagnostics
//
// LoadReport returns the same config as Load plus a Report showing which keys
// each source contributed.
//
// # Generated Helpers
//
// Schema derives field metadata for CLI usage text and coverage tests.
// ExampleYAML, MarshalYAML, MarshalJSON, and ConfigFiles support generated
// config examples and runtime config validation.
package cfgm
