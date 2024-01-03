package bukumafs

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"path"
	"strings"
	"sync"
	"time"
)

type openFileFS interface {
	OpenFile(name string, flag int, perm fs.FileMode) (fs.File, error)
}

type MkdirFS interface {
	Mkdir(name string, perm fs.FileMode) error
}

func iofs(fsys fs.FS) openFileFS {
	return fsFunc(func(name string, flag int, perm fs.FileMode) (fs.File, error) {
		if flag == os.O_RDONLY {
			return fsys.Open(name)
		}
		return nil, &fs.PathError{Op: "openfile", Path: name, Err: errors.New("not implemented")}
	})
}

func stat(fsys openFileFS, name string) (fs.FileInfo, error) {
	f, err := fsys.OpenFile(name, os.O_RDONLY, 0)
	if err != nil {
		return nil, err
	}
	return f.Stat()
}

type fsFunc func(name string, flag int, perm fs.FileMode) (fs.File, error)

func (f fsFunc) Open(name string) (fs.File, error) { return f(name, os.O_RDONLY, 0) }
func (f fsFunc) OpenFile(name string, flag int, perm fs.FileMode) (fs.File, error) {
	return f(name, flag, perm)
}

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
	return nil, &fs.PathError{Op: "readdir", Path: f.name, Err: errors.New("not implemented")}
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

func stripPrefixFS(fsys openFileFS, prefix string) openFileFS {
	return fsFunc(func(name string, flag int, perm fs.FileMode) (fs.File, error) {
		if !fs.ValidPath(name) {
			return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrInvalid}
		}

		println("stripPrefixFS:", name, prefix)
		var stripped string
		if prefix == name {
			stripped = "."
		} else if suffix, found := strings.CutPrefix(name, prefix+"/"); found {
			stripped = suffix
		} else {
			return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
		}

		println("stripPrefixFS:", stripped)
		f, err := fsys.OpenFile(stripped, flag, perm)
		if stripped == "." {
			return &nameFile{f, path.Base(name)}, fixError(err, name)
		}
		return f, fixError(err, name)
	})
}

type mountFS struct {
	m sync.Map
}

func (mfs *mountFS) OpenFile(name string, flag int, perm fs.FileMode) (fs.File, error) {
	fsys, prefix, err := mfs.lookup(name)
	if err == notFound {
		println("fallbackOpen:", name)
		f, err := mfs.fallbackOpen(name)
		return f, &fs.PathError{Op: "open", Path: name, Err: err}
	}
	if err != nil {
		return nil, &fs.PathError{Op: "open", Path: name, Err: err}
	}
	return stripPrefixFS(fsys, prefix).OpenFile(name, flag, perm)
}

func (mfs *mountFS) Open(name string) (fs.File, error) {
	return mfs.OpenFile(name, os.O_RDONLY, 0)
}

func (mfs *mountFS) Mount(name string, fsys openFileFS) error {
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

func (mfs *mountFS) fallbackOpen(name string) (fs.File, error) {
	if !fs.ValidPath(name) {
		return nil, fs.ErrInvalid
	}

	prefix := name + "/"
	if name == "." {
		prefix = ""
	}

	var ents []fs.DirEntry
	mfs.m.Range(func(k, v any) bool {
		suffix, found := strings.CutPrefix(k.(string), prefix)
		if !found {
			return true
		}

		elems := strings.Split(suffix, "/") // len(name + "/")+1 == len(name)+2
		switch len(elems) {
		case 0:
			panic("unreachable")
		case 1:
			// name/basename
			info, err := stat(v.(openFileFS), ".")
			if err == nil {
				// If err is non-nil,  pretends that the file does not exist.
				ents = append(ents, fs.FileInfoToDirEntry(&nameFileInfo{info, elems[0]}))
			}
		default:
			// name/dir...
			ents = append(ents, fs.FileInfoToDirEntry(fallbackInfo(elems[0])))
		}
		return true
	})

	println("fallbackOpen:", name, len(ents))
	if len(ents) == 0 && name != "." {
		// The root file "." always exists.
		return nil, fs.ErrNotExist
	}
	return &fallbackDir{name: name, ents: ents}, nil

}

var notFound = errors.New("not found")

func (mfs *mountFS) lookup(name string) (fsys openFileFS, prefix string, err error) {
	if !fs.ValidPath(name) {
		return nil, "", fs.ErrInvalid
	}
	elems := strings.Split(name, "/")

	for i := len(elems); i > 0; i-- {
		var a any
		prefix := strings.Join(elems[:i], "/")
		a, ok := mfs.m.Load(prefix)
		println("lookup:", prefix, ok)
		if ok {
			println("lookup found:", prefix)
			return a.(openFileFS), prefix, nil
		}
	}
	return nil, "", notFound
}

type fallbackDir struct {
	name   string
	ents   []fs.DirEntry
	offset int
}

func (f *fallbackDir) Read(_ []byte) (int, error) {
	return 0, &fs.PathError{Op: "read", Path: f.name, Err: errors.New("is a directory")}
}

func (f *fallbackDir) Stat() (fs.FileInfo, error) {
	return fallbackInfo(f.name), nil
}

func (f *fallbackDir) ReadDir(n int) ([]fs.DirEntry, error) {
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

func (*fallbackDir) Close() error { return nil }

var _ fs.ReadDirFile = &fallbackDir{}

type fallbackInfo string

func (fallbackInfo) IsDir() bool        { return true }
func (fallbackInfo) ModTime() time.Time { return time.Time{} }
func (fallbackInfo) Mode() fs.FileMode  { return fs.ModeDir | 0555 }
func (f fallbackInfo) Name() string     { return string(f) }
func (fallbackInfo) Size() int64        { return 0 }
func (fallbackInfo) Sys() any           { return nil }

var _ fs.FileInfo = fallbackInfo("")
