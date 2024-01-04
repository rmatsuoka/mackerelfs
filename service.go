package mackerelfs

import (
	"io/fs"

	"github.com/mackerelio/mackerel-client-go"
	"github.com/rmatsuoka/mackerelfs/internal/muxfs"
)

func servicesFS(c *mackerel.Client) fs.FS {
	return loadFS(func() (Seq2[string, fs.FS], error) {
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
	m.FS("roles", rolesFS(c, name))
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

func rolesFS(c *mackerel.Client, serviceName string) fs.FS {
	return loadFS(func() (Seq2[string, fs.FS], error) {
		roles, err := c.FindRoles(serviceName)
		return func(yield func(string, fs.FS) bool) {
			for _, r := range roles {
				if !yield(r.Name, roleFS(c, serviceName, r.Name)) {
					return
				}
			}
		}, err
	})
}

func roleFS(c *mackerel.Client, serviceName, roleName string) fs.FS {
	return loadFS(func() (Seq2[string, fs.FS], error) {
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
}
