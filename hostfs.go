package mackerelfs

import (
	"bytes"
	"encoding/json"
	"io"
	"io/fs"

	"github.com/mackerelio/mackerel-client-go"

	"github.com/rmatsuoka/mackerelfs/internal/muxfs"
)

type devNull struct{}

func (devNull) Read(_ []byte) (int, error) { return 0, io.EOF }

func HostFS(client *mackerel.Client) fs.FS {
	m := muxfs.NewFS()
	h := &hosts{Client: client, cache: make(map[string]string)}
	m.VarFS(h)
	m.File("reload", muxfs.ReaderFile(func() (io.Reader, error) {
		h.reload()
		return devNull{}, nil
	}))
	return m
}

type hosts struct {
	cache map[string]string
	*mackerel.Client
}

func (h *hosts) All() (muxfs.Seq[string], error) {
	err := h.reload()
	if err != nil {
		return nil, err
	}
	return func(yield func(string) bool) {
		for k := range h.cache {
			if !yield(k) {
				return
			}
		}
	}, nil
}

func (h *hosts) FS(name string) (fs.FS, bool) {
	id, ok := h.cache[name]
	if !ok {
		return nil, false
	}
	return newHostFS(h.Client, id), true
}

func (h *hosts) reload() error {
	hosts, err := h.FindHosts(&mackerel.FindHostsParam{})
	if err != nil {
		return err
	}
	for _, host := range hosts {
		h.cache[host.Name] = host.ID
	}
	return nil
}

func newHostFS(client *mackerel.Client, id string) fs.FS {
	fsys := muxfs.NewFS()
	h := &host{Client: client, id: id}
	fsys.File("info", muxfs.ReaderFile(h.hostInfo))
	fsys.FS("metrics", metricFS(hostMetrics{id: id, Client: client}))
	return fsys
}

type host struct {
	*mackerel.Client
	id string
}

func (h *host) hostInfo() (io.Reader, error) {
	host, err := h.FindHost(h.id)
	if err != nil {
		return nil, err
	}
	b := new(bytes.Buffer)
	enc := json.NewEncoder(b)
	enc.SetIndent("", "  ")
	if err := enc.Encode(host); err != nil {
		return nil, err
	}
	return b, nil
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

var _ metrics = &hostMetrics{}
