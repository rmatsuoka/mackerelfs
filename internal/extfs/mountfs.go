package extfs

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"sync"
)

type nameFileInfo struct {
	fs.FileInfo
	name string
}

func (info *nameFileInfo) Name() string { return info.name }

type nameFile struct {
	fs.File
	name string
}

func (f *nameFile) Stat() (fs.FileInfo, error) {
	i, err := f.File.Stat()
	return &nameFileInfo{i, f.name}, fixError(err, f.name)
}

func (f *nameFile) ReadDir(n int) ([]fs.DirEntry, error) {
	if d, ok := f.File.(fs.ReadDirFile); ok {
		e, err := d.ReadDir(n)
		return e, fixError(err, f.name)
	}
	return nil, &fs.PathError{Op: "readdir", Path: f.name, Err: ErrNotImplemented}
}

var _ fs.ReadDirFile = &nameFile{}

func fixError(err error, name string) error {
	var perr *fs.PathError
	if errors.As(err, &perr) {
		perr.Path = name
		return perr
	}
	return err
}

func stripPrefix(path, prefix string) string {
	println("stripPrefix", path, prefix)
	if prefix == "." {
		return path
	}
	if path == prefix {
		return "."
	}
	return path[len(prefix)+1:] // len(prefix)+1 == len(prefix+'/')
}

type mountFS struct {
	m sync.Map
}

func NewMountFS(root fs.FS) *mountFS {
	m := &mountFS{}
	m.m.Store(".", root)
	return m
}

func (mfs *mountFS) OpenFile(name string, flag int, perm fs.FileMode) (fs.File, error) {
	fsys, prefix, err := mfs.lookup(name)
	if err != nil {
		return nil, &fs.PathError{Op: "open", Path: name, Err: err}
	}

	stripped := stripPrefix(name, prefix)
	f, err := OpenFile(fsys, stripped, flag, perm)
	if stripped == "." {
		return &nameFile{f, path.Base(name)}, fixError(err, name)
	}

	return f, fixError(err, name)
}

func (mfs *mountFS) Open(name string) (fs.File, error) {
	return mfs.OpenFile(name, os.O_RDONLY, 0)
}

func (mfs *mountFS) Mkdir(name string, perm fs.FileMode) error {
	fsys, prefix, err := mfs.lookup(name)
	if err != nil {
		return &fs.PathError{Op: "mkdir", Path: name, Err: err}
	}
	return Mkdir(fsys, stripPrefix(name, prefix), perm)
}

func (mfs *mountFS) Mount(name string, fsys fs.FS) error {
	if !fs.ValidPath(name) {
		return &fs.PathError{Op: "mount", Path: name, Err: fs.ErrInvalid}
	}

	f, err := mfs.Open(name)
	if err != nil {
		return &fs.PathError{Op: "mount", Path: name, Err: err}
	}
	info, err := f.Stat()
	if err != nil {
		return &fs.PathError{Op: "mount", Path: name, Err: err}
	}
	if !info.IsDir() {
		return &fs.PathError{Op: "mount", Path: name, Err: errors.New("not a directory")}
	}
	mfs.m.Store(name, fsys)
	return nil
}

func (mfs *mountFS) lookup(name string) (fsys fs.FS, prefix string, err error) {
	if !fs.ValidPath(name) {
		return nil, "", fs.ErrInvalid
	}

	walkToRoot(name)(func(s string) bool {
		v, ok := mfs.m.Load(s)
		fmt.Printf("lookup %s: %s %v\n", name, s, ok)
		if ok {
			fsys = v.(fs.FS)
			prefix = s
			return false
		}
		return true
	})

	if fsys == nil {
		return nil, "", fs.ErrNotExist
	}

	return fsys, prefix, nil
}

type Seq[T any] func(yield func(T) bool)

// for example path is 'usr/bin/ls' then it yields 'usr/bin/ls', 'usr/bin', 'usr', '.'.
func walkToRoot(path string) Seq[string] {
	return func(yield func(string) bool) {
		if !yield(path) {
			return
		}

		for i := len(path) - 1; i >= 0; i-- {
			if path[i] == '/' {
				if !yield(path[:i]) {
					return
				}
			}
		}

		// root
		if !yield(".") {
			return
		}
	}
}
