//go:build unix

package config

import (
	"os"
	"syscall"
)

// secretOpenFlags opens without blocking, so a path swapped for a FIFO after
// the type check cannot hang startup.
const secretOpenFlags = os.O_RDONLY | syscall.O_NONBLOCK
