package metricfs

import (
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"time"

	"github.com/mackerelio/mackerel-client-go"
	"github.com/rmatsuoka/mackerelfs/internal/muxfs"
)

type Metrics interface {
	ListNames() ([]string, error)
	Fetch(name string, from, to int64) ([]mackerel.MetricValue, error)
}

func FS(m Metrics) fs.FS {
	mfs := muxfs.NewFS()
	mfs.VarFS(&metricsVarFS{m})
	return mfs
}

type metricsVarFS struct {
	m Metrics
}

func (v *metricsVarFS) All() (muxfs.Seq[string], error) {
	l, err := v.m.ListNames()
	if err != nil {
		return nil, err
	}
	return func(yield func(string) bool) {
		for _, name := range l {
			if !yield(name) {
				return
			}
		}
	}, nil
}

func (v *metricsVarFS) FS(name string) (fs.FS, bool) {
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
