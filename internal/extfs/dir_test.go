package extfs

import (
	"testing"
	"testing/fstest"
)

func TestDirFS(t *testing.T) {
	fsys := NewDirFS(0555)
	if err := fsys.Mkdir("foo", 0555); err != nil {
		t.Fatal(err)
	}
	if err := fsys.Mkdir("bar", 0555); err != nil {
		t.Fatal(err)
	}
	if err := fsys.Mkdir("bar/foo", 0555); err != nil {
		t.Fatal(err)
	}
	if err := fsys.Mkdir("foo/bar", 0555); err != nil {
		t.Fatal(err)
	}
	if err := fstest.TestFS(fsys, "foo/bar", "bar/foo"); err != nil {
		t.Error(err)
	}
}
