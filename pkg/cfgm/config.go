package cfgm

import (
	"os"
	"path/filepath"
	"reflect"
	"time"

	"github.com/urfave/cli/v3"
)

// DefaultPaths returns conventional config file search paths.
//
// Load adds these paths as an optional low-priority file source by default.
// Pass an app name to include app-specific paths before generic paths. For each
// location, paths are searched in .yaml, .yml, then .json order.
func DefaultPaths(appName ...string) []string {
	var paths []string

	if len(appName) > 0 && appName[0] != "" {
		name := appName[0]
		paths = appendConfigFormats(paths, "."+name)
		if home, err := os.UserHomeDir(); err == nil {
			paths = appendConfigFormats(paths, filepath.Join(home, "."+name))
		}
		paths = appendConfigFormats(paths, "/etc/"+name+"/config")
	}

	paths = appendConfigFormats(paths, "config")
	paths = appendConfigFormats(paths, filepath.Join("config", "config"))

	return paths
}

func appendConfigFormats(paths []string, base string) []string {
	return append(paths, base+".yaml", base+".yml", base+".json")
}

func setCLIFlagValue(cmd *cli.Command, config map[string]any, configPath, cliFlag string, fieldType reflect.Type) bool {
	switch fieldType {
	case reflect.TypeFor[time.Duration]():
		setByPath(config, configPath, cmd.Duration(cliFlag))

		return true
	case reflect.TypeFor[time.Time]():
		setByPath(config, configPath, cmd.Timestamp(cliFlag))

		return true
	}

	switch fieldType.Kind() { //nolint:exhaustive // unsupported CLI flag kinds return false
	case reflect.String:
		setByPath(config, configPath, cmd.String(cliFlag))
		return true
	case reflect.Bool:
		setByPath(config, configPath, cmd.Bool(cliFlag))
		return true
	case reflect.Int:
		setByPath(config, configPath, cmd.Int(cliFlag))
		return true
	case reflect.Int8:
		setByPath(config, configPath, cmd.Int8(cliFlag))
		return true
	case reflect.Int16:
		setByPath(config, configPath, cmd.Int16(cliFlag))
		return true
	case reflect.Int32:
		setByPath(config, configPath, cmd.Int32(cliFlag))
		return true
	case reflect.Int64:
		setByPath(config, configPath, cmd.Int64(cliFlag))
		return true
	case reflect.Uint:
		setByPath(config, configPath, cmd.Uint(cliFlag))
		return true
	case reflect.Uint8:
		setByPath(config, configPath, uint8(cmd.Uint(cliFlag))) //nolint:gosec // CLI value expected to be in uint8 range
		return true
	case reflect.Uint16:
		setByPath(config, configPath, cmd.Uint16(cliFlag))
		return true
	case reflect.Uint32:
		setByPath(config, configPath, cmd.Uint32(cliFlag))
		return true
	case reflect.Uint64:
		setByPath(config, configPath, cmd.Uint64(cliFlag))
		return true
	case reflect.Float32:
		setByPath(config, configPath, cmd.Float32(cliFlag))
		return true
	case reflect.Float64:
		setByPath(config, configPath, cmd.Float64(cliFlag))
		return true
	case reflect.Slice:
		return setSliceFlagValue(cmd, config, configPath, cliFlag, fieldType)
	case reflect.Map:
		if isStringMapType(fieldType) {
			setByPath(config, configPath, cmd.StringMap(cliFlag))
			return true
		}
	}

	return false
}

func setSliceFlagValue(cmd *cli.Command, config map[string]any, configPath, cliFlag string, fieldType reflect.Type) bool {
	elemType := fieldType.Elem()

	if elemType == reflect.TypeFor[time.Time]() {
		setByPath(config, configPath, cmd.TimestampArgs(cliFlag))

		return true
	}

	switch elemType.Kind() { //nolint:exhaustive // unsupported slice element kinds return false
	case reflect.String:
		setByPath(config, configPath, cmd.StringSlice(cliFlag))
		return true
	case reflect.Int:
		setByPath(config, configPath, cmd.IntSlice(cliFlag))
		return true
	case reflect.Int8:
		setByPath(config, configPath, cmd.Int8Slice(cliFlag))
		return true
	case reflect.Int16:
		setByPath(config, configPath, cmd.Int16Slice(cliFlag))
		return true
	case reflect.Int32:
		setByPath(config, configPath, cmd.Int32Slice(cliFlag))
		return true
	case reflect.Int64:
		setByPath(config, configPath, cmd.Int64Slice(cliFlag))
		return true
	case reflect.Uint16:
		setByPath(config, configPath, cmd.Uint16Slice(cliFlag))
		return true
	case reflect.Uint32:
		setByPath(config, configPath, cmd.Uint32Slice(cliFlag))
		return true
	case reflect.Float32:
		setByPath(config, configPath, cmd.Float32Slice(cliFlag))
		return true
	case reflect.Float64:
		setByPath(config, configPath, cmd.Float64Slice(cliFlag))
		return true
	}

	return false
}
