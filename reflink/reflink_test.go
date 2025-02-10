package reflink_test

import (
	"bytes"
	"crypto/rand"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/KarpelesLab/reflink"
)

func TestReflink(t *testing.T) {
	d, err := os.MkdirTemp("", "reflinktest*")
	if err != nil {
		t.Fatalf("failed to create temporary directory: %s", err)
		return
	}
	defer os.RemoveAll(d)

	buf := make([]byte, 1024*1024) // 1MB
	_, err = io.ReadFull(rand.Reader, buf)
	if err != nil {
		t.Errorf("failed to fill test buffer with random bytes: %s", err)
	}

	err = os.WriteFile(filepath.Join(d, "src.bin"), buf, 0666)
	if err != nil {
		t.Fatalf("failed to create initial test file: %s", err)
		return
	}

	// perform reflink
	err = reflink.Always(filepath.Join(d, "src.bin"), filepath.Join(d, "test1.bin"))
	if err != nil {
		if errors.Is(err, reflink.ErrReflinkUnsupported) {
			t.Logf("cannot test reflink on this OS: %s", err)
		} else if errors.Is(err, reflink.ErrReflinkFailed) {
			t.Logf("cannot test reflink on this configuration: %s", err)
		} else {
			t.Errorf("failed to reflink.Always: %s", err)
		}
	}

	// perform reflink auto
	err = reflink.Auto(filepath.Join(d, "src.bin"), filepath.Join(d, "test2.bin"))
	if err != nil {
		t.Errorf("failed to reflink.Auto: %s", err)
	}
	err = testFile(filepath.Join(d, "test2.bin"), buf)
	if err != nil {
		t.Errorf("bad output file for reflink.Auto: %s", err)
	}

	in, err := os.Open(filepath.Join(d, "src.bin"))
	if err != nil {
		t.Fatalf("failed to open source file for reading: %s", err)
		return
	}
	defer in.Close()

	out, err := os.Create(filepath.Join(d, "test3.bin"))
	if err != nil {
		t.Fatalf("failed to create target file for writing: %s", err)
		return
	}
	defer out.Close()

	err = reflink.Reflink(out, in, true)
	if err != nil {
		t.Errorf("reflink on file failed: %s", err)
	}
	err = testOsFile(out, buf)
	if err != nil {
		t.Errorf("reflink target file content fails: %s", err)
	}

	out.Truncate(0)

	err = reflink.Partial(out, in, 0, 512*1024, 256*1024, true)
	if err != nil {
		t.Errorf("failed to reflink.Partial(fallback=true): %s", err)
	}
	err = testOsFile(out, buf[512*1024:(512+256)*1024])
	if err != nil {
		t.Errorf("reflink target file content fails: %s", err)
	}

}

func testFile(fn string, target []byte) error {
	data, err := os.ReadFile(fn)
	if err != nil {
		return err
	}
	if !bytes.Equal(data, target) {
		return errors.New("file content does not match")
	}
	return nil
}

func testOsFile(f *os.File, target []byte) error {
	st, err := f.Stat()
	if err != nil {
		return err
	}
	r := io.NewSectionReader(f, 0, st.Size())
	buf, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	if !bytes.Equal(buf, target) {
		return errors.New("file content does not match")
	}
	return nil
}
