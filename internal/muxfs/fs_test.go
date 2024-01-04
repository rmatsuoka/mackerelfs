package muxfs

import (
	"io"
	"io/fs"
	"strings"
	"testing"
	"testing/fstest"
)

type mapChildren map[string]fs.FS

func (m mapChildren) All() Seq[string] {
	return func(yield func(string) bool) {
		for k := range m {
			if !yield(k) {
				return
			}
		}
	}
}

func (m mapChildren) FS(name string) (fs.FS, bool) {
	f, ok := m[name]
	return f, ok
}

func TestFS(t *testing.T) {
	t.Run("TestFS with Children", func(t *testing.T) {
		f := NewFS()
		m := make(mapChildren)
		m["dir1"] = fstest.MapFS{
			"d1_file1":         {},
			"d1_dir2/d1_file2": {},
		}
		m["dir2"] = fstest.MapFS{
			"d2_file1": {},
		}
		f.VarFS(m)
		if err := fstest.TestFS(
			f,
			"dir1",
			"dir1/d1_file1",
			"dir1/d1_dir2/d1_file2",
			"dir2",
			"dir2/d2_file1",
		); err != nil {
			t.Error(err)
		}
	})
	t.Run("TestFS with File", func(t *testing.T) {
		f := NewFS()
		f.File("file1", ReaderFile(func() (io.Reader, error) {
			return strings.NewReader("hello"), nil
		}))
		f.File("file2", CtlFile(func(s string) error { return nil }))
		if err := fstest.TestFS(f, "file1", "file2"); err != nil {
			t.Error(err)
		}
	})
	t.Run("TestFS with File and Children", func(t *testing.T) {
		f := NewFS()
		m := make(mapChildren)
		m["chi1"] = fstest.MapFS{
			"c1_file1":         {},
			"c1_dir1/c1_file2": {},
		}
		f.VarFS(m)
		f.File("file1", ReaderFile(func() (io.Reader, error) {
			return strings.NewReader("hello"), nil
		}))
		if err := fstest.TestFS(
			f,
			"chi1",
			"chi1/c1_file1",
			"chi1/c1_dir1/c1_file2",
			"file1",
		); err != nil {
			t.Error(err)
		}
	})
}
