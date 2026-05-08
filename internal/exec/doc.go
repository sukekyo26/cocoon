// Package exec wraps invocations of external commands (docker, git, …)
// behind a [Runner] interface so the rest of the codebase can be unit-tested
// without requiring those tools on the host.
//
// Production code uses [New] to obtain a real runner that delegates to
// os/exec. Tests use [RecordingRunner] to capture call history and stub
// canned responses.
//
// Higher-level typed wrappers live in subpackages dockerx and gitx.
package exec
