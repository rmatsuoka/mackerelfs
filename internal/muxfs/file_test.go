package muxfs

import (
	"bytes"
	"io"
	"strings"
	"testing"
)

func TestCtlFile(t *testing.T) {
	const msg = `
ls . .. /usr
cat /usr/share/man/man1/cat.1
echo hello world
`
	b := new(bytes.Buffer)

	file := CtlFile(func(s string) error {
		_, err := b.WriteString(s + "\n")
		return err
	})
	f, err := file(&openArgs{})
	if err != nil {
		t.Fatalf("failed on open: %v", err)
	}
	writer := f.(io.WriteCloser)
	if _, err := strings.NewReader(msg).WriteTo(writer); err != nil {
		t.Fatalf("failed on write: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("failed on close: %v", err)
	}

	got := b.String()
	if got != msg {
		t.Errorf(`got string is different from expetecd
got: %q
expected: %q`, got, msg)
	}
}
