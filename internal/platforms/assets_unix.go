//go:build unix

package platforms

import (
	"os"
	"syscall"
)

// assetOpenFlags opens without waiting for a writer, so a FIFO swapped in after the check fails the
// os.SameFile test instead of blocking.
const assetOpenFlags = os.O_RDONLY | syscall.O_NONBLOCK
