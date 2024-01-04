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
	return loadFS(func() (Seq2[string, fs.FS], error) {
		names, err := m.ListNames()
		if err != nil {
			return nil, err
		}
		return func(yield func(string, fs.FS) bool) {
			for _, name := range names {
				if !yield(name, metricTSDBFS(m, name)) {
					return
				}
			}
		}, nil
	})
}

func metricTSDBFS(f metricsFetcher, name string) fs.FS {
	m := muxfs.NewFS()
	m.File("1hour", muxfs.ReaderFile(func() (io.Reader, error) {
		now := time.Now()
		values, err := f.Fetch(name, now.Add(-time.Hour).Unix(), now.Unix())
		if err != nil {
			return nil, err
		}
		b := new(bytes.Buffer)
		for _, v := range values {
			fmt.Fprintf(b, "%s\t%f\t%d\n", name, v.Value, v.Time)
		}
		return b, err
	}))
	return m
}
