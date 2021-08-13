// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package kubespan

import (
	"github.com/prometheus/client_golang/prometheus"
)

var (
	metricPeerCount = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "network",
		Subsystem: "kubespan",
		Name:      "peers",
		Help:      "Total known peer count.",
	})

	metricPeerUpCount = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "network",
		Subsystem: "kubespan",
		Name:      "peers_up",
		Help:      "Total connected peer count.",
	})

	metricRouteCount = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "network",
		Subsystem: "kubespan",
		Name:      "routes",
		Help:      "Total number of routes to Wireguard peers.",
	}, []string{"family"})
)

func init() {
	prometheus.MustRegister(metricPeerCount)

	prometheus.MustRegister(metricPeerUpCount)

	prometheus.MustRegister(metricRouteCount)
}
