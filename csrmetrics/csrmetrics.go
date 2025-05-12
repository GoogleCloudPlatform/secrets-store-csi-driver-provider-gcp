// Copyright 2025 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package csrmetrics (Compute and Storage Resource Metrics)
// contains metrics definitions to be recorded for monitoring purposes
package csrmetrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// OutboundRPCStatus is a status string of the outbound RPC metric contains either status code or status string
type OutboundRPCStatus string

// Status constants for metrics
const (
	OutboundRPCStatusNotFound OutboundRPCStatus = "not_found"
	OutboundRPCStatusError    OutboundRPCStatus = "error"
	OutboundRPCStatusOK       OutboundRPCStatus = "ok"
)

var (
	// Observation function to observe delay
	// Update this method for unit tests
	timeSinceSeconds = func(start time.Time) float64 {
		return time.Since(start).Seconds()
	}
	outboundRPCCount = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "outbound_rpc_count",
		Help: "Count of outbound RPCs to GCP",
	}, []string{"status", "kind"})

	outboundRPCLatency = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name: "outbound_rpc_latency",
		Help: "Latency of outbound RPCs to GCP (in seconds)",
	}, []string{"status", "kind"})
)

func init() {
	prometheus.MustRegister(
		outboundRPCCount,
		outboundRPCLatency,
	)
}

// OutboundRPCStartRecorder marks the start of a outbound RPC operation. Caller is
// responsible for calling the returned function, which records Prometheus
// metrics for this operation.
func OutboundRPCStartRecorder(kind string) func(status OutboundRPCStatus) {
	start := time.Now()

	return func(status OutboundRPCStatus) {
		outboundRPCCount.WithLabelValues(string(status), kind).Inc()
		outboundRPCLatency.WithLabelValues(string(status), kind).Observe(timeSinceSeconds(start))
	}
}
