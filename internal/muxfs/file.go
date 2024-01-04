package muxfs

import (
	"bufio"
	"io"
	"io/fs"

	"github.com/rmatsuoka/mackerelfs/internal/extfs"
)

func ReaderFile(f func() (io.Reader, error)) File {
	return func(o *openArgs) (fs.File, error) {
		r, err := f()
		if err != nil {
			return nil, &fs.PathError{Op: "open", Path: o.base(), Err: err}
		}
		return &readerFile{Reader: r, name: o.base()}, nil
	}
}

type readerFile struct {
	name string
	io.Reader
}

var _ fs.File = &readerFile{}

func (f *readerFile) Stat() (fs.FileInfo, error) {
	return &fileInfo{name: f.name, mode: 0444}, nil
}

func (f *readerFile) Close() error {
	if closer, ok := f.Reader.(io.Closer); ok {
		return closer.Close()
	}
	return nil
}

func CtlFile(fn func(s string) error) File {
	return func(o *openArgs) (fs.File, error) {
		return newCtlFile(o.base(), fn), nil
	}
}

type ctlFile struct {
	name string
	w    *io.PipeWriter
}

func newCtlFile(base string, fn func(s string) error) *ctlFile {
	r, w := io.Pipe()

	go func() {
		s := bufio.NewScanner(r)
		for s.Scan() {
			if err := fn(s.Text()); err != nil {
				r.CloseWithError(err)
				return
			}
		}
		r.CloseWithError(s.Err())
	}()
	return &ctlFile{name: base, w: w}
}

func (f *ctlFile) Stat() (fs.FileInfo, error) {
	return &fileInfo{name: f.name, mode: 0222}, nil
}

func (f *ctlFile) Write(p []byte) (int, error) {
	return f.w.Write(p)
}

func (f *ctlFile) Close() error { return f.w.Close() }

func (f *ctlFile) Read(_ []byte) (int, error) {
	return 0, &fs.PathError{Op: "read", Path: f.name, Err: extfs.ErrNotImplemented}
}
