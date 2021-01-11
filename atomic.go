package fscache

import (
	"io"
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"
)

// atomicWriteFile atomically writes data to a file named by filename.
func atomicWriteFile(filename, tmpfile string, src io.Reader, perm os.FileMode) error {
	dst, err := newAtomicFileWriter(filename, tmpfile, perm)
	if err != nil {
		return err
	}
	if _, err := io.Copy(dst, src); err != nil {
		dst.(*atomicFileWriter).writeErr = err
	}
	return dst.Close()
}

type atomicFileWriter struct {
	f        *os.File
	fn       string
	writeErr error
	perm     os.FileMode
}

// newAtomicFileWriter returns WriteCloser so that writing to it writes to a
// temporary file and closing it atomically changes the temporary file to
// destination path. Writing and closing concurrently is not allowed.
// tmpdir and filename must be within the same filesystem.
func newAtomicFileWriter(filename, tmpfile string, perm os.FileMode) (io.WriteCloser, error) {
	f, err := os.OpenFile(tmpfile, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0664)
	if err != nil {
		return nil, err
	}
	if err = unix.Flock(int(f.Fd()), unix.LOCK_EX); err != nil {
		f.Close()
		return nil, err
	}
	abspath, err := filepath.Abs(filename)
	if err != nil {
		return nil, err
	}
	return &atomicFileWriter{
		f:    f,
		fn:   abspath,
		perm: perm,
	}, nil
}

func (w *atomicFileWriter) Write(dt []byte) (int, error) {
	n, err := w.f.Write(dt)
	if err != nil {
		w.writeErr = err
	}
	return n, err
}

func (w *atomicFileWriter) Close() (retErr error) {
	defer func() {
		if retErr != nil || w.writeErr != nil {
			os.Remove(w.f.Name())
		}
	}()
	if err := w.f.Sync(); err != nil {
		w.f.Close()
		return err
	}
	if err := w.f.Close(); err != nil {
		return err
	}
	if err := os.Chmod(w.f.Name(), w.perm); err != nil {
		return err
	}
	if w.writeErr == nil {
		return os.Rename(w.f.Name(), w.fn)
	}
	return nil
}