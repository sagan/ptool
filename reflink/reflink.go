//go:build !linux

package reflink

import "os"

func reflinkInternal(d, s *os.File) error {
	return ErrReflinkUnsupported
}

func reflinkRangeInternal(dst, src *os.File, dstOffset, srcOffset, n int64) error {
	return ErrReflinkUnsupported
}

func copyFileRange(dst, src *os.File, dstOffset, srcOffset, n int64) (int64, error) {
	return 0, ErrReflinkUnsupported // @fix
}
