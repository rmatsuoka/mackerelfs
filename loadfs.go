package mackerelfs

import (
	"io/fs"
	"strings"

	"github.com/rmatsuoka/mackerelfs/internal/muxfs"
)

type Seq2[K, V any] func(yield func(K, V) bool)

func loadFS(f func() (Seq2[string, fs.FS], error)) fs.FS {
	m := muxfs.NewFS()
	varFS := &reloadVarFS{load: f, m: make(map[string]fs.FS)}
	m.VarFS(varFS)
	m.File("ctl", muxfs.CtlFile(func(s string) error {
		f := strings.Fields(s)
		if len(f) > 0 && f[0] == "reload" {
			return varFS.reload()
		}
		return nil
	}))
	return m
}

type reloadVarFS struct {
	load func() (Seq2[string, fs.FS], error)
	m    map[string]fs.FS
}

func (f *reloadVarFS) All() (muxfs.Seq[string], error) {
	if len(f.m) == 0 {
		return nil, f.reload()
	}
	return func(yield func(string) bool) {
		for k := range f.m {
			if !yield(k) {
				return
			}
		}
	}, nil
}

func (f *reloadVarFS) FS(name string) (fs.FS, bool) {
	if len(f.m) == 0 {
		f.reload()
	}
	fsys, ok := f.m[name]
	if !ok {
		return nil, false
	}
	return fsys, true
}

func (f *reloadVarFS) reload() error {
	iter, err := f.load()
	if err != nil {
		return err
	}
	clear(f.m)
	iter(func(name string, fsys fs.FS) bool {
		f.m[name] = fsys
		return true
	})
	return nil
}
