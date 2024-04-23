package version

import "strings"

var (
	VersionSuffix = "DEV" // e.g. DEV
	VersionTag    = "v0.1.10"
	Version       = ""
	Commit        = "none"
	Date          = "unknown"
)

func init() {
	if Version == "" {
		if VersionSuffix == "" {
			Version = VersionTag
		} else {
			Version = VersionTag + "-" + VersionSuffix
		}
	} else if !strings.HasPrefix(Version, "v") {
		Version = "v" + Version
	}
}
