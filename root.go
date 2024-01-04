package mackerelfs

import (
	"errors"
	"io/fs"
	"strings"

	"github.com/mackerelio/mackerel-client-go"
	"github.com/rmatsuoka/mackerelfs/internal/muxfs"
)

type root map[string]orgs

type orgs struct {
	api  string
	fsys fs.FS
}

func FS() fs.FS {
	r := root{}
	m := muxfs.NewFS()
	m.File("ctl", muxfs.CtlFile(r.ctlFile))
	m.VarFS(r)
	return m
}

func (r root) ctlFile(s string) error {
	f := strings.Fields(s)
	if len(f) < 1 {
		return nil
	}
	switch f[0] {
	case "new":
		if len(f) == 1 {
			return errors.New("missing arguments")
		}
		name, fsys, err := orgFS(newClient(f[1]))
		if err != nil {
			return err
		}
		r[name] = orgs{api: f[1], fsys: fsys}
	case "delete":
		if len(f) == 1 {
			return errors.New("missing arguments")
		}
		delete(r, f[1])
	}
	return nil
}

func (r root) All() (muxfs.Seq[string], error) {
	return func(yield func(string) bool) {
		for k := range r {
			if !yield(k) {
				return
			}
		}
	}, nil
}

func (r root) FS(name string) (fs.FS, bool) {
	f, ok := r[name]
	return f.fsys, ok
}

func orgFS(c *mackerel.Client) (name string, fsys fs.FS, err error) {
	org, err := c.GetOrg()
	if err != nil {
		return "", nil, err
	}
	m := muxfs.NewFS()
	m.FS("hosts", hostsFS(c))
	return org.Name, m, nil
}

func newClient(apikey string) *mackerel.Client {
	client, _ := mackerel.NewClientWithOptions(
		apikey,
		"https://api.mackerelio.com/",
		true,
	)
	return client
}
