package extfs

import (
	"errors"
	"io/fs"
	"os"
)

var (
	ErrNotImplemented = errors.New("not implemented")
)

type OpenFileFS interface {
	fs.FS
	OpenFile(name string, flag int, perm fs.FileMode) (fs.File, error)
}

func OpenFile(fsys fs.FS, name string, flag int, perm fs.FileMode) (fs.File, error) {
	if flag == os.O_RDONLY {
		return fsys.Open(name)
	}

	if fsys, ok := fsys.(OpenFileFS); ok {
		return fsys.OpenFile(name, flag, perm)
	}
	return nil, &fs.PathError{Op: "openfile", Path: name, Err: ErrNotImplemented}
}

type MkdirFS interface {
	fs.FS
	Mkdir(name string, perm fs.FileMode) error
}

func Mkdir(fsys fs.FS, name string, perm fs.FileMode) error {
	if fsys, ok := fsys.(MkdirFS); ok {
		return fsys.Mkdir(name, perm)
	}
	return &fs.PathError{Op: "mkdir", Path: name, Err: ErrNotImplemented}
}
