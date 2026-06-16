package workspace

import (
	"errors"
	"syscall"
)

// errCrossDevice lets tests force the copy-then-remove fallback portably
// (assign renameFunc to return it).
var errCrossDevice = errors.New("cross-device move")

// isCrossDevice reports whether err means src and dst are on different
// filesystems, so a plain rename can't work.
func isCrossDevice(err error) bool {
	return errors.Is(err, errCrossDevice) || errors.Is(err, syscall.EXDEV)
}
