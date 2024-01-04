package mackerelfs

import (
	"io"
	"io/fs"
	"strings"

	"github.com/mackerelio/mackerel-client-go"
	"github.com/rmatsuoka/mackerelfs/internal/muxfs"
)

func servicesFS(c *mackerel.Client) fs.FS {
	return itemFS(func() (Seq2[string, fs.FS], error) {
		services, err := c.FindServices()
		return func(yield func(string, fs.FS) bool) {
			for _, v := range services {
				if !yield(v.Name, serviceFS(c, v.Name)) {
					return
				}
			}
		}, err
	})

}

func serviceFS(c *mackerel.Client, name string) fs.FS {
	m := muxfs.NewFS()
	m.FS("metrics", metricFS(&serviceMetricFetcher{name: name, Client: c}))
	varFS := newItemVarFS(func() (Seq2[string, fs.FS], error) {
		roles, err := c.FindRoles(name)
		return func(yield func(string, fs.FS) bool) {
			for _, r := range roles {
				if !yield(r.Name, roleFS(c, name, r.Name, r.Memo)) {
					return
				}
			}
		}, err
	})
	m.File("ctl", muxfs.CtlFile(func(s string) error {
		f := strings.Fields(s)
		if len(f) > 0 && f[0] == "reload" {
			if err := varFS.reload(); err != nil {
				return err
			}
		}
		return nil
	}))
	m.VarFS(varFS)
	return m
}

type serviceMetricFetcher struct {
	name string
	*mackerel.Client
}

func (s *serviceMetricFetcher) ListNames() ([]string, error) {
	return s.ListServiceMetricNames(s.name)
}

func (s *serviceMetricFetcher) Fetch(name string, from, to int64) ([]mackerel.MetricValue, error) {
	return s.FetchServiceMetricValues(s.name, name, from, to)
}

func roleFS(c *mackerel.Client, serviceName, roleName, memo string) fs.FS {
	m := muxfs.NewFS()
	varFS := newItemVarFS(func() (Seq2[string, fs.FS], error) {
		hosts, err := c.FindHosts(&mackerel.FindHostsParam{
			Service: serviceName,
			Roles:   []string{roleName},
		})
		return func(yield func(string, fs.FS) bool) {
			for _, host := range hosts {
				if !yield(host.Name, newHostFS(c, host.ID)) {
					return
				}
			}
		}, err
	})
	m.VarFS(varFS)
	m.File("ctl", muxfs.CtlFile(func(s string) error {
		f := strings.Fields(s)
		if len(f) > 0 && f[0] == "reload" {
			if err := varFS.reload(); err != nil {
				return err
			}
		}
		return nil
	}))
	m.File("memo", muxfs.ReaderFile(func() (io.Reader, error) {
		return strings.NewReader(memo), nil
	}))
	return m
}
