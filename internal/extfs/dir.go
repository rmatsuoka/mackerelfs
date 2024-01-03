package extfs

import (
	"errors"
	"io"
	"io/fs"
	"path"
	"strings"
	"time"
)

type DirFS struct {
	children map[string]*DirFS
	perm     fs.FileMode
}

var _ MkdirFS = &DirFS{}

func NewDirFS(perm fs.FileMode) *DirFS {
	return &DirFS{
		children: make(map[string]*DirFS),
		perm:     perm,
	}
}

func (d *DirFS) Open(name string) (fs.File, error) {
	dir, err := d.lookup(name)
	if err != nil {
		return nil, &fs.PathError{Op: "open", Path: name, Err: err}
	}
	ents := make([]fs.DirEntry, 0, len(d.children))
	for k, v := range dir.children {
		ents = append(ents, dirInfo{name: k, perm: v.perm})
	}
	return &dirFile{name: path.Base(name), ents: ents, perm: dir.perm}, nil
}

func (d *DirFS) Mkdir(name string, perm fs.FileMode) error {
	prefix, base := path.Split(name)
	if prefix == "" {
		prefix = "."
	} else {
		prefix = prefix[:len(prefix)-1]
	}
	dir, err := d.lookup(prefix)
	if err != nil {
		return &fs.PathError{Op: "mkdir", Path: name, Err: err}
	}
	dir.children[base] = &DirFS{children: make(map[string]*DirFS), perm: perm}
	return nil
}

func (d *DirFS) lookup(name string) (*DirFS, error) {
	if !fs.ValidPath(name) {
		return nil, fs.ErrInvalid
	}
	if name == "." {
		return d, nil
	}

	dir := d
	for _, elem := range strings.Split(name, "/") {
		v, ok := dir.children[elem]
		if !ok {
			return nil, fs.ErrNotExist
		}
		dir = v
	}
	return dir, nil
}

type dirFile struct {
	name   string
	perm   fs.FileMode
	ents   []fs.DirEntry
	offset int
}

func (f *dirFile) Read(_ []byte) (int, error) {
	return 0, &fs.PathError{Op: "read", Path: f.name, Err: errors.New("is a directory")}
}

func (f *dirFile) Stat() (fs.FileInfo, error) {
	return dirInfo{f.name, f.perm}, nil
}

func (f *dirFile) ReadDir(n int) ([]fs.DirEntry, error) {
	l := len(f.ents) - f.offset
	if n < l && n > 0 {
		l = n
	}
	if l == 0 {
		if n <= 0 {
			return nil, nil
		}
		return nil, io.EOF
	}
	ents := f.ents[f.offset : f.offset+l]
	f.offset += l
	return ents, nil
}

func (*dirFile) Close() error { return nil }

var _ fs.ReadDirFile = &dirFile{}

type dirInfo struct {
	name string
	perm fs.FileMode
}

func (dirInfo) IsDir() bool                  { return true }
func (dirInfo) ModTime() time.Time           { return time.Time{} }
func (d dirInfo) Mode() fs.FileMode          { return fs.ModeDir | d.perm }
func (dirInfo) Type() fs.FileMode            { return fs.ModeDir }
func (d dirInfo) Name() string               { return d.name }
func (dirInfo) Size() int64                  { return 0 }
func (d dirInfo) Info() (fs.FileInfo, error) { return d, nil }
func (dirInfo) Sys() any                     { return nil }

var _ fs.FileInfo = dirInfo{}
var _ fs.DirEntry = dirInfo{}
