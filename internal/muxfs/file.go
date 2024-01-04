package muxfs

import (
	"bufio"
	"io"
	"io/fs"
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
	done <-chan struct{}
}

func newCtlFile(base string, fn func(s string) error) *ctlFile {
	r, w := io.Pipe()
	done := make(chan struct{})
	go func() {
		defer close(done)
		s := bufio.NewScanner(r)
		for s.Scan() {
			if err := fn(s.Text()); err != nil {
				r.CloseWithError(err)
				return
			}
		}
		r.CloseWithError(s.Err())
	}()
	return &ctlFile{name: base, w: w, done: done}
}

func (f *ctlFile) Stat() (fs.FileInfo, error) {
	return &fileInfo{name: f.name, mode: 0222}, nil
}
func (f *ctlFile) Write(p []byte) (int, error) { return f.w.Write(p) }
func (f *ctlFile) Close() error {
	err := f.w.Close()
	<-f.done
	return err
}
func (f *ctlFile) Read(_ []byte) (int, error) { return 0, io.EOF }
