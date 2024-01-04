package muxfs

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"path"
	"strings"

	"github.com/rmatsuoka/mackerelfs/internal/extfs"
)

type Seq[T any] func(yield func(T) bool)

type Children interface {
	FS(base string) (fs.FS, bool)
	All() Seq[fs.DirEntry]
}

type FS struct {
	files    map[string]File
	children Children
}

type File func(o *openArgs) (fs.File, error)

func (f *FS) Children(c Children) {
	f.children = c
}

type openArgs struct {
	name string
	flag int
	perm fs.FileMode
}

func (o *openArgs) base() string { return path.Base(o.name) }

func (fsys *FS) File(base string, f File) {
	if strings.ContainsRune(base, '/') {
		panic("base contains '/', base must be a file")
	}
	fsys.files[base] = f
}

func firstNode(path string) string {
	i := strings.IndexByte(path, '/')
	if i == -1 {
		return path
	}
	return path[:i]
}

func (fsys *FS) OpenFile(name string, flag int, perm fs.FileMode) (fs.File, error) {
	if name == "." {
		return &root{ents: fsys.rootEnts()}, nil
	}

	open, err := fsys.lookup(name)
	if err != nil {
		return nil, &fs.PathError{Op: "open", Path: name, Err: err}
	}
	return open(&openArgs{name: name, flag: flag, perm: perm})
}

func (fsys *FS) Open(name string) (fs.File, error) {
	return fsys.OpenFile(name, os.O_RDONLY, 0)
}

func (fsys *FS) rootEnts() []fs.DirEntry {
	var ents []fs.DirEntry
	for k := range fsys.files {
		ents = append(ents, &fileInfo{name: k})
	}
	if fsys.children == nil {
		return ents
	}

	fsys.children.All()(func(e fs.DirEntry) bool {
		ents = append(ents, e)
		return true
	})

	return ents
}

func (fsys *FS) lookup(name string) (func(o *openArgs) (fs.File, error), error) {
	// name must not be "." (root).
	if !fs.ValidPath(name) {
		return nil, fs.ErrInvalid
	}

	if file, ok := fsys.files[name]; ok {
		return file, nil
	}

	if fsys.children == nil {
		return nil, fs.ErrNotExist
	}

	prefix := firstNode(name)
	f, ok := fsys.children.FS(prefix)
	if !ok {
		return nil, fs.ErrNotExist
	}

	return func(o *openArgs) (fs.File, error) {
		// o.name must not be "." or "" (empty).
		stripped := stripPrefix(o.name, prefix)
		f, err := extfs.OpenFile(f, stripped, o.flag, o.perm)
		if stripped == "." {
			return &fixedFile{f, path.Base(name)}, fixError(err, o.name)
		}
		return f, fixError(err, o.name)
	}, nil
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

type fixedFileInfo struct {
	fs.FileInfo
	name string
}

func (info *fixedFileInfo) Name() string { return info.name }

type fixedFile struct {
	fs.File
	name string
}

func (f *fixedFile) Stat() (fs.FileInfo, error) {
	i, err := f.File.Stat()
	return &fixedFileInfo{i, f.name}, fixError(err, f.name)
}

func (f *fixedFile) ReadDir(n int) ([]fs.DirEntry, error) {
	if d, ok := f.File.(fs.ReadDirFile); ok {
		e, err := d.ReadDir(n)
		return e, fixError(err, f.name)
	}
	return nil, &fs.PathError{Op: "readdir", Path: f.name, Err: extfs.ErrNotImplemented}
}

var _ fs.ReadDirFile = &fixedFile{}

func fixError(err error, name string) error {
	var perr *fs.PathError
	if errors.As(err, &perr) {
		perr.Path = name
		return perr
	}
	return err
}

type root struct {
	ents   []fs.DirEntry
	offset int
}

func (r *root) Read(_ []byte) (int, error) {
	return 0, &fs.PathError{Op: "read", Path: ".", Err: errors.New("is a directory")}
}

func (r *root) Stat() (fs.FileInfo, error) {
	return fileInfo{name: ".", mode: fs.ModeDir | 0555}, nil
}

func (r *root) ReadDir(n int) ([]fs.DirEntry, error) {
	l := len(r.ents) - r.offset
	if n < l && n > 0 {
		l = n
	}
	if l == 0 {
		if n <= 0 {
			return nil, nil
		}
		return nil, io.EOF
	}
	ents := r.ents[r.offset : r.offset+l]
	r.offset += l
	return ents, nil
}

func (*root) Close() error { return nil }

var _ fs.ReadDirFile = &root{}
