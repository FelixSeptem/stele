package app

// These variables are replaced by release builds with -ldflags. Development
// builds intentionally report bounded non-secret defaults.
var (
	BuildVersion   = "dev"
	BuildID        = "unknown"
	BuildTimestamp = "unknown"
)
