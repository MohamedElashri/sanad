package cli

// Report versions are compatibility boundaries for automation consumers.
// Removing fields, changing field types, or changing field meanings requires a
// new version and a corresponding schema.
const (
	checkReportVersion   = 1
	planReportVersion    = 1
	upgradeReportVersion = 2
)
