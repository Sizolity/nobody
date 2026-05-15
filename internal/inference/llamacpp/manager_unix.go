//go:build !windows

package llamacpp

import "syscall"

// syscallSignalZero is signal 0, the POSIX "is alive" probe used by
// processAlive. Lives in this file behind //go:build !windows so a
// future Windows port (manager_windows.go) can supply its own
// equivalent without a duplicate-symbol error.
var syscallSignalZero = syscall.Signal(0)
