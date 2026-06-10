package version

// Version is the source-baked default returned by `cocoon version` and
// consulted by `cocoon self-update`. It is bumped in lockstep with the
// repository-root VERSION file so users who install via
// `go install github.com/sukekyo26/cocoon/cmd/cocoon@latest` (which does
// not run the justfile's ldflags step) still see the real release
// version instead of a generic sentinel.
//
// `just build` overrides this at link time via:
//
//	go build -ldflags "-X github.com/sukekyo26/cocoon/internal/version.Version=$(cat VERSION)"
var Version = "0.15.7"

func Get() string { return Version }
