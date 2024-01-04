//go:build ignore
// +build ignore

package metricfs

import (
	"github.com/mackerelio/mackerel-client-go"
)

type Metrics interface {
	All() ([]string, error)
	Fetch(name string) ([]mackerel.MetricValue, error)
}
