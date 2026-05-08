package version

// Version is overridden at build time via:
//
//	go build -ldflags "-X github.com/sukekyo26/cocoon/internal/version.Version=$(cat VERSION)"
var Version = "dev"

// Get returns the cocoon version string.
func Get() string { return Version }
