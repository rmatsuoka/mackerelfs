package mackerelfs

import (
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"time"

	"github.com/mackerelio/mackerel-client-go"
	"github.com/rmatsuoka/mackerelfs/internal/muxfs"
)

type metricsFetcher interface {
	ListNames() ([]string, error)
	Fetch(name string, from, to int64) ([]mackerel.MetricValue, error)
}

func metricFS(m metricsFetcher) fs.FS {
	mfs := muxfs.NewFS()
	v := &metricsVarFS{
		m:       m,
		metrics: make(map[string]bool),
	}
	mfs.VarFS(v)
	mfs.File("ctl", muxfs.CtlFile(func(s string) error {
		if s != "" {
			return v.reload()
		}
		return nil
	}))
	return mfs
}

type metricsVarFS struct {
	m       metricsFetcher
	metrics map[string]bool
}

func (v *metricsVarFS) reload() error {
	l, err := v.m.ListNames()
	if err != nil {
		return err
	}
	clear(v.metrics)
	for _, name := range l {
		v.metrics[name] = true
	}
	return nil
}

func (v *metricsVarFS) All() (muxfs.Seq[string], error) {
	if len(v.metrics) == 0 {
		v.reload()
	}
	return func(yield func(string) bool) {
		for name := range v.metrics {
			if !yield(name) {
				return
			}
		}
	}, nil
}

func (v *metricsVarFS) FS(name string) (fs.FS, bool) {
	if len(v.metrics) == 0 {
		v.reload()
	}

	if _, ok := v.metrics[name]; !ok {
		return nil, false
	}

	m := muxfs.NewFS()
	m.File("1hour", muxfs.ReaderFile(func() (io.Reader, error) {
		now := time.Now()
		values, err := v.m.Fetch(name, now.Add(-time.Hour).Unix(), now.Unix())
		if err != nil {
			return nil, err
		}
		b := new(bytes.Buffer)
		for _, v := range values {
			fmt.Fprintf(b, "%s\t%f\t%d\n", name, v.Value, v.Time)
		}
		return b, err
	}))
	return m, true
}
