package web

import (
	"gfep/utils"
	"runtime"
	"runtime/debug"
	"strings"
)

// computeBuildMeta 返回构建所用 Go 工具链版本与构建时间（尽量有值：ldflags > vcs.time）。
func computeBuildMeta() (goVersion, buildTime string) {
	goVersion = runtime.Version()
	buildTime = strings.TrimSpace(utils.BuildTime)
	if info, ok := debug.ReadBuildInfo(); ok {
		if gv := strings.TrimSpace(info.GoVersion); gv != "" {
			goVersion = gv
		}
		if buildTime == "" {
			for _, s := range info.Settings {
				if s.Key == "vcs.time" {
					if v := strings.TrimSpace(s.Value); v != "" {
						buildTime = v
					}
					break
				}
			}
		}
	}
	return goVersion, buildTime
}
