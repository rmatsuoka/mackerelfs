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

type VarFS interface {
	FS(base string) (fs.FS, bool)
	All() (Seq[string], error)
}

type FS struct {
	files map[string]File
	fs    map[string]fs.FS
	varFS VarFS
}

func NewFS() *FS {
	return &FS{
		files: make(map[string]File),
		fs:    make(map[string]fs.FS),
	}
}

type File func(o *openArgs) (fs.File, error)

func (fsys *FS) FS(base string, f fs.FS) {
	if strings.ContainsRune(base, '/') {
		panic("base contains '/'")
	}
	fsys.fs[base] = f
}

func (fsys *FS) VarFS(c VarFS) {
	fsys.varFS = c
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
		ents, err := fsys.rootEnts()
		return &root{ents: ents}, err
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

func (fsys *FS) rootEnts() ([]fs.DirEntry, error) {
	var ents []fs.DirEntry
	for name, open := range fsys.files {
		name := name
		open := open
		ents = append(ents, &rootDirEntry{
			name: name,
			info: func() (fs.FileInfo, error) {
				f, err := open(&openArgs{
					name: name,
					flag: os.O_RDONLY,
					perm: 0,
				})
				if err != nil {
					return nil, err
				}
				return f.Stat()
			},
			typ: 0,
		})
	}

	iter, err := fsys.allFS()
	if err != nil {
		return nil, err
	}

	iter(func(name string) bool {
		ents = append(ents, &rootDirEntry{
			name: name,
			info: func() (fs.FileInfo, error) {
				f, err := fsys.lookupFS(name)
				if err != nil {
					return nil, err
				}
				s, err := fs.Stat(f, ".")
				if err != nil {
					return nil, err
				}
				return &fixedFileInfo{name: name, FileInfo: s}, nil
			},
			typ: fs.ModeDir,
		})
		return true
	})

	return ents, nil
}

func (fsys *FS) lookup(name string) (func(o *openArgs) (fs.File, error), error) {
	// name must not be "." (root).
	if !fs.ValidPath(name) {
		return nil, fs.ErrInvalid
	}

	if file, ok := fsys.files[name]; ok {
		return file, nil
	}

	prefix := firstNode(name)
	f, err := fsys.lookupFS(prefix)
	if err != nil {
		return nil, err
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

func (fsys *FS) lookupFS(key string) (fs.FS, error) {
	f, ok := fsys.fs[key]
	if ok {
		return f, nil
	}

	if fsys.varFS == nil {
		return nil, fs.ErrNotExist
	}
	f, ok = fsys.varFS.FS(key)
	if !ok {
		return nil, fs.ErrNotExist
	}
	return f, nil
}

func (fsys *FS) allFS() (Seq[string], error) {
	var (
		iter Seq[string]
		err  error
	)
	if fsys.varFS != nil {
		iter, err = fsys.varFS.All()
	}
	return func(yield func(string) bool) {
		for k := range fsys.fs {
			if !yield(k) {
				return
			}
		}
		if iter == nil {
			return
		}
		iter(yield)
	}, err
}

func stripPrefix(path, prefix string) string {
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

type rootDirEntry struct {
	name string
	info func() (fs.FileInfo, error)
	typ  fs.FileMode
}

var _ fs.DirEntry = &rootDirEntry{}

func (e *rootDirEntry) IsDir() bool                { return e.typ.IsDir() }
func (e *rootDirEntry) Name() string               { return e.name }
func (e *rootDirEntry) Type() fs.FileMode          { return e.typ }
func (e *rootDirEntry) Info() (fs.FileInfo, error) { return e.info() }
