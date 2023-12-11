package version

var (
	VersionSuffix = "DEV" // eg. DEV
	VersionTag    = "v0.1.9"
	Version       = ""
)

func init() {
	if Version == "" {
		if VersionSuffix == "" {
			Version = VersionTag
		} else {
			Version = VersionTag + "-" + VersionSuffix
		}
	}
}
