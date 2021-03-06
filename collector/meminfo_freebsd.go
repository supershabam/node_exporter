// +build !nomeminfo

package collector

import (
	"errors"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/log"
)

/*
#include <stddef.h>
#include <sys/sysctl.h>

int _sysctl(const char* name) {
        int val;
        size_t size = sizeof(val);
        int res = sysctlbyname(name, &val, &size, NULL, 0);
        if (res == -1) {
                return -1;
        }
        if (size != sizeof(val)) {
                return -2;
        }
        return val;
}
*/
import "C"

const (
	memInfoSubsystem = "memory"
)

type meminfoCollector struct {
	metrics map[string]prometheus.Gauge
}

func init() {
	Factories["meminfo"] = NewMeminfoCollector
}

// Takes a prometheus registry and returns a new Collector exposing
// Memory stats.
func NewMeminfoCollector() (Collector, error) {
	return &meminfoCollector{
		metrics: map[string]prometheus.Gauge{},
	}, nil
}

func (c *meminfoCollector) Update(ch chan<- prometheus.Metric) (err error) {
	var pages map[string]C.int
	pages = make(map[string]C.int)

	size := C._sysctl(C.CString("vm.stats.vm.v_page_size"))
	if size == -1 {
		return errors.New("sysctl(vm.stats.vm.v_page_size) failed")
	}
	if size == -2 {
		return errors.New("sysctl(vm.stats.vm.v_page_size) failed, wrong buffer size")
	}

	pages["active"] = C._sysctl(C.CString("vm.stats.vm.v_active_count"))
	pages["inactive"] = C._sysctl(C.CString("vm.stats.vm.v_inactive_count"))
	pages["wire"] = C._sysctl(C.CString("vm.stats.vm.v_wire_count"))
	pages["cache"] = C._sysctl(C.CString("vm.stats.vm.v_cache_count"))
	pages["free"] = C._sysctl(C.CString("vm.stats.vm.v_free_count"))
	pages["swappgsin"] = C._sysctl(C.CString("vm.stats.vm.v_swappgsin"))
	pages["swappgsout"] = C._sysctl(C.CString("vm.stats.vm.v_swappgsout"))
	pages["total"] = C._sysctl(C.CString("vm.stats.vm.v_page_count"))

	for key := range pages {
		if pages[key] == -1 {
			return errors.New("sysctl() failed for " + key)
		}
		if pages[key] == -2 {
			return errors.New("sysctl() failed for " + key + ", wrong buffer size")
		}
	}

	log.Debugf("Set node_mem: %#v", pages)
	for k, v := range pages {
		if _, ok := c.metrics[k]; !ok {
			c.metrics[k] = prometheus.NewGauge(prometheus.GaugeOpts{
				Namespace: Namespace,
				Subsystem: memInfoSubsystem,
				Name:      k,
				Help:      k + " from sysctl()",
			})
		}
		// Convert metrics to kB (same as Linux meminfo).
		c.metrics[k].Set(float64(v) * float64(size))
		c.metrics[k].Collect(ch)
	}
	return err
}
