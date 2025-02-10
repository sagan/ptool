package reflink

import "errors"

// ErrReflinkUnsupported is returned by Always() if the operation is not
// supported on the current operating system. Auto() will never return this
// error.
var (
	ErrReflinkUnsupported = errors.New("reflink is not supported on this OS")
	ErrReflinkFailed      = errors.New("reflink is not supported on this OS or file")
)
