package version

var (
	VersionSuffix = "" // eg. DEV
	VersionTag    = "v0.1.6"
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
