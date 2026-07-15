package cfgm

import (
	"os"
	"path/filepath"
)

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
