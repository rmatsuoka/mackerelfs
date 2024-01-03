package bukumafs

import (
	"fmt"
	"io/fs"
	"testing"
	"testing/fstest"
	"time"
)

func Test_mountFS(t *testing.T) {
	root := iofs(fstest.MapFS{
		"fsys0": {Mode: fs.ModeDir},
		"fsys1": {Mode: fs.ModeDir},
	})
	fsys0 := iofs(fstest.MapFS{
		"foo/bar": {
			Data:    []byte("foo/bar"),
			Mode:    0,
			ModTime: time.Time{},
			Sys:     nil,
		},
	})
	fsys1 := iofs(fstest.MapFS{
		"bar/foo": {
			Data:    []byte("bar/foo"),
			Mode:    0,
			ModTime: time.Time{},
			Sys:     nil,
		},
	})

	fsys := &mountFS{}
	if err := fsys.Mount(".", root); err != nil {
		t.Fatal(err)
	}
	if err := fsys.Mount("fsys0", fsys0); err != nil {
		t.Fatal(err)
	}
	if err := fsys.Mount("fsys1", fsys1); err != nil {
		t.Fatal(err)
	}
	e, err := fs.ReadDir(fsys, "fsys0/foo")
	if err != nil {
		t.Error(err)
	}
	for _, e := range e {
		fmt.Println(fs.FormatDirEntry(e))
	}
	if err := fstest.TestFS(fsys, "fsys0/foo/bar", "fsys1/bar/foo"); err != nil {
		t.Error(err)
	}
}
