package exec

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"sync"
)

// Call captures a single invocation made against a [RecordingRunner].
type Call struct {
	Name string
	Args []string
	// Method is one of "Run", "Output", "CombinedOutput", "RunWithIO".
	Method string
	// Opts is set when Method == "RunWithIO".
	Opts RunOptions
}

// Stub is a canned response keyed by command name + args.
type Stub struct {
	Stdout         []byte
	Stderr         []byte // written to opts.Stderr or merged into CombinedOutput
	CombinedOutput []byte // overrides Stdout+Stderr concat for CombinedOutput
	Err            error
}

// RecordingRunner records every invocation in [Calls] and returns canned
// responses set via [Stub]. It is safe for concurrent use.
type RecordingRunner struct {
	mu    sync.Mutex
	Calls []Call
	stubs map[string]Stub
}

// NewRecordingRunner returns an empty recorder with no stubs registered.
func NewRecordingRunner() *RecordingRunner {
	return &RecordingRunner{
		mu:    sync.Mutex{},
		Calls: nil,
		stubs: map[string]Stub{},
	}
}

// Stub registers a canned response for the given command + args.
func (r *RecordingRunner) Stub(name string, args []string, stub Stub) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.stubs[stubKey(name, args)] = stub
}

// Run implements [Runner].
func (r *RecordingRunner) Run(_ context.Context, name string, args ...string) error {
	r.mu.Lock()
	r.Calls = append(r.Calls, Call{
		Name: name, Args: append([]string(nil), args...), Method: "Run",
		Opts: RunOptions{}, //nolint:exhaustruct // not applicable to Run
	})
	stub := r.stubs[stubKey(name, args)]
	r.mu.Unlock()
	return stub.Err
}

// Output implements [Runner].
func (r *RecordingRunner) Output(_ context.Context, name string, args ...string) ([]byte, error) {
	r.mu.Lock()
	r.Calls = append(r.Calls, Call{
		Name: name, Args: append([]string(nil), args...), Method: "Output",
		Opts: RunOptions{}, //nolint:exhaustruct // not applicable to Output
	})
	stub := r.stubs[stubKey(name, args)]
	r.mu.Unlock()
	return stub.Stdout, stub.Err
}

// CombinedOutput implements [Runner].
func (r *RecordingRunner) CombinedOutput(_ context.Context, name string, args ...string) ([]byte, error) {
	r.mu.Lock()
	r.Calls = append(r.Calls, Call{
		Name: name, Args: append([]string(nil), args...), Method: "CombinedOutput",
		Opts: RunOptions{}, //nolint:exhaustruct // not applicable to CombinedOutput
	})
	stub := r.stubs[stubKey(name, args)]
	r.mu.Unlock()
	if stub.CombinedOutput != nil {
		return stub.CombinedOutput, stub.Err
	}
	var buf bytes.Buffer
	buf.Write(stub.Stdout)
	buf.Write(stub.Stderr)
	return buf.Bytes(), stub.Err
}

// RunWithIO implements [Runner].
func (r *RecordingRunner) RunWithIO(_ context.Context, opts RunOptions) error {
	r.mu.Lock()
	r.Calls = append(r.Calls, Call{
		Name:   opts.Name,
		Args:   append([]string(nil), opts.Args...),
		Method: "RunWithIO",
		Opts:   opts,
	})
	stub := r.stubs[stubKey(opts.Name, opts.Args)]
	r.mu.Unlock()
	if opts.Stdout != nil && len(stub.Stdout) > 0 {
		if _, err := io.Copy(opts.Stdout, bytes.NewReader(stub.Stdout)); err != nil {
			return fmt.Errorf("recording runner: copy stdout: %w", err)
		}
	}
	if opts.Stderr != nil && len(stub.Stderr) > 0 {
		if _, err := io.Copy(opts.Stderr, bytes.NewReader(stub.Stderr)); err != nil {
			return fmt.Errorf("recording runner: copy stderr: %w", err)
		}
	}
	return stub.Err
}

func stubKey(name string, args []string) string {
	if len(args) == 0 {
		return name
	}
	return fmt.Sprintf("%s\x00%s", name, joinArgs(args))
}

func joinArgs(args []string) string {
	var b bytes.Buffer
	for i, a := range args {
		if i > 0 {
			b.WriteByte(0x00)
		}
		b.WriteString(a)
	}
	return b.String()
}
