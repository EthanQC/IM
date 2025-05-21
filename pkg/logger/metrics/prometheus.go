package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
)

type Metrics struct {
	logCounter *prometheus.CounterVec
}

func (m *Metrics) IncLogCounter(level string) {
	m.logCounter.WithLabelValues(level).Inc()
}
