//go:build linux

package reflink

import (
	"errors"
	"os"

	"golang.org/x/sys/unix"
)

// reflinkInternal performs the actual reflink action without worrying about fallback
func reflinkInternal(d, s *os.File) error {
	ss, err := s.SyscallConn()
	if err != nil {
		return err
	}
	sd, err := d.SyscallConn()
	if err != nil {
		return err
	}

	var err2, err3 error

	err = sd.Control(func(dfd uintptr) {
		err2 = ss.Control(func(sfd uintptr) {
			// int ioctl(int dest_fd, FICLONE, int src_fd);
			err3 = unix.IoctlFileClone(int(dfd), int(sfd))
		})
	})

	if err != nil {
		// sd.Control failed
		return err
	}
	if err2 != nil {
		// ss.Control failed
		return err2
	}

	if err3 != nil && errors.Is(err3, unix.ENOTSUP) {
		return ErrReflinkFailed
	}

	// err3 is ioctl() response
	return err3
}

func reflinkRangeInternal(dst, src *os.File, dstOffset, srcOffset, n int64) error {
	ss, err := src.SyscallConn()
	if err != nil {
		return err
	}
	sd, err := dst.SyscallConn()
	if err != nil {
		return err
	}

	var err2, err3 error

	err = sd.Control(func(dfd uintptr) {
		err2 = ss.Control(func(sfd uintptr) {
			req := &unix.FileCloneRange{
				Src_fd:      int64(sfd),
				Src_offset:  uint64(srcOffset),
				Src_length:  uint64(n),
				Dest_offset: uint64(dstOffset),
			}

			// int ioctl(int dest_fd, FICLONE, int src_fd);
			err3 = unix.IoctlFileCloneRange(int(dfd), req)
		})
	})

	if err != nil {
		// sd.Control failed
		return err
	}
	if err2 != nil {
		// ss.Control failed
		return err2
	}
	if err3 != nil && errors.Is(err3, unix.ENOTSUP) {
		return ErrReflinkFailed
	}

	// err3 is ioctl() response
	return err3
}

func copyFileRange(dst, src *os.File, dstOffset, srcOffset, n int64) (int64, error) {
	ss, err := src.SyscallConn()
	if err != nil {
		return 0, err
	}
	sd, err := dst.SyscallConn()
	if err != nil {
		return 0, err
	}

	var resN int
	var err2, err3 error

	err = sd.Control(func(dfd uintptr) {
		err2 = ss.Control(func(sfd uintptr) {
			// call syscall
			resN, err3 = unix.CopyFileRange(int(sfd), &srcOffset, int(dfd), &dstOffset, int(n), 0)
		})
	})

	if err != nil {
		// sd.Control failed
		return int64(resN), err
	}
	if err2 != nil {
		// ss.Control failed
		return int64(resN), err2
	}

	// err3 is ioctl() response
	return int64(resN), err3

}
