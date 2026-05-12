// Package selfupdatecli implements the `cocoon self-update` lifecycle
// verb.
//
// `cocoon self-update` checks GitHub Releases for a newer cocoon binary,
// downloads it under SHA256 verification, and atomically replaces the
// current binary. The full implementation lands in F5; this skeleton
// only registers the command and surface flags so that (a) the help tree
// is complete in v0.1.0 and (b) the install-source-aware suppression of
// the upgrade path on `go install` builds has a stable home.
package selfupdatecli

import "errors"

// ErrUsage signals a bad invocation; mapped to exit 2 at the binary boundary.
var ErrUsage = errors.New("usage error")

// ErrFailure signals a runtime failure of `cocoon self-update`.
var ErrFailure = errors.New("self-update failed")

// errAssetMissing is wrapped when the SHA256SUMS file does not list
// the asset we just downloaded.
var errAssetMissing = errors.New("asset checksum missing")
