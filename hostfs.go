package mackerelfs

import (
	"bytes"
	"encoding/json"
	"io"
	"io/fs"

	"github.com/mackerelio/mackerel-client-go"

	"github.com/rmatsuoka/mackerelfs/internal/muxfs"
)

func hostsFS(client *mackerel.Client) fs.FS {
	m := muxfs.NewFS()
	h := &hosts{Client: client, host: make(map[string]*hostFS)}
	m.VarFS(h)
	m.File("ctl", muxfs.CtlFile(func(s string) error {
		if s != "" {
			return h.reload()
		}
		return nil
	}))
	return m
}

type hosts struct {
	host map[string]*hostFS
	*mackerel.Client
}

type hostFS struct {
	fsys fs.FS
	id   string
}

func (h *hosts) All() (muxfs.Seq[string], error) {
	if len(h.host) == 0 {
		return nil, h.reload()
	}
	return func(yield func(string) bool) {
		for k := range h.host {
			if !yield(k) {
				return
			}
		}
	}, nil
}

func (h *hosts) FS(name string) (fs.FS, bool) {
	if len(h.host) == 0 {
		h.reload()
	}
	fsys, ok := h.host[name]
	if !ok {
		return nil, false
	}
	return fsys.fsys, true
}

func (h *hosts) reload() error {
	hosts, err := h.FindHosts(&mackerel.FindHostsParam{})
	if err != nil {
		return err
	}
	clear(h.host)
	for _, host := range hosts {
		h.host[host.Name] = &hostFS{
			id:   host.ID,
			fsys: newHostFS(h.Client, host.ID),
		}
	}
	return nil
}

func newHostFS(client *mackerel.Client, id string) fs.FS {
	fsys := muxfs.NewFS()
	h := &host{Client: client, id: id}
	fsys.File("info", muxfs.ReaderFile(func() (io.Reader, error) {
		var err error
		if len(h.info) == 0 {
			err = h.reload()
		}
		return bytes.NewReader(h.info), err
	}))
	fsys.File("ctl", muxfs.CtlFile(func(s string) error {
		if s != "" {
			return h.reload()
		}
		return nil
	}))
	fsys.FS("metrics", metricFS(hostMetrics{id: id, Client: client}))
	return fsys
}

type host struct {
	*mackerel.Client
	id   string
	info []byte
}

func (h *host) reload() error {
	host, err := h.FindHost(h.id)
	if err != nil {
		return err
	}
	b := new(bytes.Buffer)
	enc := json.NewEncoder(b)
	enc.SetIndent("", "  ")
	if err := enc.Encode(host); err != nil {
		return err
	}
	h.info = b.Bytes()
	return nil
}

type hostMetrics struct {
	id string
	*mackerel.Client
}

func (h hostMetrics) ListNames() ([]string, error) {
	return h.ListHostMetricNames(h.id)
}

func (h hostMetrics) Fetch(name string, from, to int64) ([]mackerel.MetricValue, error) {
	return h.FetchHostMetricValues(h.id, name, from, to)
}

var _ metricsFetcher = &hostMetrics{}
