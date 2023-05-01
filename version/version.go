package version

var (
	VersionSuffix = "" // eg. DEV
	VersionTag    = "v0.1.0"
	Version       string
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
