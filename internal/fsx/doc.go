// Package fsx contains filesystem helpers, in particular the
// [AtomicWriteFile] stage-and-rename writer used when generating workspace
// artifacts. Atomic writes guarantee that readers never observe a
// partially-written file even if the process is interrupted mid-write.
package fsx
