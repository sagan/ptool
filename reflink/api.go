package reflink

import (
	"fmt"
	"io"
	"io/fs"
	"io/ioutil"
	"os"
	"path/filepath"
)

// Always will perform a reflink operation and fail on error.
//
// This is equivalent to command cp --reflink=always
func Always(src, dst string) error {
	return reflinkFile(src, dst, false)
}

// Auto will attempt to perform a reflink operation and fallback to normal data
// copy if reflink is not supported.
//
// This is equivalent to cp --reflink=auto
func Auto(src, dst string) error {
	return reflinkFile(src, dst, true)
}

// reflinkFile perform the reflink operation in order to copy src into dst using
// the underlying filesystem's copy-on-write reflink system. If this fails (for
// example the filesystem does not support reflink) and fallback is true, then
// copy_file_range will be used, and if that fails too io.Copy will be used to
// copy the data.
func reflinkFile(src, dst string, fallback bool) error {
	s, err := os.Open(src)
	if err != nil {
		return err
	}
	defer s.Close()

	// generate temporary file for output
	tmp, err := ioutil.TempFile(filepath.Dir(dst), "")
	if err != nil {
		return err
	}

	// copy to temp file
	err = reflinkInternal(tmp, s)

	// if reflink failed but we allow fallback, first attempt using copyFileRange (will actually clone bytes on some filesystems)
	if (err != nil) && fallback {
		var st fs.FileInfo
		st, err = s.Stat()
		if err == nil {
			_, err = copyFileRange(tmp, s, 0, 0, st.Size())
		}
	}

	// if everything failed and we fallback, attempt io.Copy
	if (err != nil) && fallback {
		// reflink failed but fallback enabled, perform a normal copy instead
		_, err = io.Copy(tmp, s)
	}
	tmp.Close() // we're not writing to this anymore

	// if an error happened, remove temp file and signal error
	if err != nil {
		os.Remove(tmp.Name())
		return err
	}

	// keep src file mode if possible
	if st, err := s.Stat(); err == nil {
		tmp.Chmod(st.Mode())
	}

	// replace dst file
	err = os.Rename(tmp.Name(), dst)
	if err != nil {
		// failed to rename (dst is not writable?)
		os.Remove(tmp.Name())
		return err
	}

	return nil
}

// Reflink performs the reflink operation on the passed files, replacing
// dst's contents with src. If fallback is true and reflink fails,
// copy_file_range will be used first, and if that fails too io.Copy will
// be used to copy the data.
func Reflink(dst, src *os.File, fallback bool) error {
	err := reflinkInternal(dst, src)
	if (err != nil) && fallback {
		// reflink failed, but we can fallback, but first we need to know the file's size
		var st fs.FileInfo
		st, err = src.Stat()
		if err != nil {
			// couldn't stat source, this can't be helped
			return fmt.Errorf("failed to stat source: %w", err)
		}
		_, err = copyFileRange(dst, src, 0, 0, st.Size())
		if err != nil {
			// copyFileRange failed too, switch to simple io copy
			reader := io.NewSectionReader(src, 0, st.Size())
			writer := &sectionWriter{w: dst}
			dst.Truncate(0) // assuming any error in trucate will result in copy error
			_, err = io.Copy(writer, reader)
		}
	}
	return err
}

// Partial performs a range reflink operation on the passed files, replacing
// part of dst's contents with data from src. If fallback is true and reflink
// fails, copy_file_range will be used first, and if that fails too io.CopyN
// will be used to copy the data.
func Partial(dst, src *os.File, dstOffset, srcOffset, n int64, fallback bool) error {
	err := reflinkRangeInternal(dst, src, dstOffset, srcOffset, n)
	if (err != nil) && fallback {
		_, err = copyFileRange(dst, src, dstOffset, srcOffset, n)
	}

	if (err != nil) && fallback {
		// seek both src & dst
		reader := io.NewSectionReader(src, srcOffset, n)
		writer := &sectionWriter{w: dst, base: dstOffset}
		_, err = io.CopyN(writer, reader, n)
	}
	return err
}
