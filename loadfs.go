package mackerelfs

import (
	"io/fs"
	"strings"

	"github.com/rmatsuoka/mackerelfs/internal/muxfs"
)

type Seq2[K, V any] func(yield func(K, V) bool)

func itemFS(fetch func() (Seq2[string, fs.FS], error)) fs.FS {
	m := muxfs.NewFS()
	varFS := &itemVarFS{fetch: fetch, m: make(map[string]fs.FS)}
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

func newItemVarFS(fetch func() (Seq2[string, fs.FS], error)) *itemVarFS {
	return &itemVarFS{fetch: fetch, m: make(map[string]fs.FS)}
}

type itemVarFS struct {
	fetch func() (Seq2[string, fs.FS], error)
	m     map[string]fs.FS
}

func (f *itemVarFS) All() (muxfs.Seq[string], error) {
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

func (f *itemVarFS) FS(name string) (fs.FS, bool) {
	if len(f.m) == 0 {
		f.reload()
	}
	fsys, ok := f.m[name]
	if !ok {
		return nil, false
	}
	return fsys, true
}

func (f *itemVarFS) reload() error {
	iter, err := f.fetch()
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
