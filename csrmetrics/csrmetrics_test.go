package csrmetrics

import (
	"fmt"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
)

func assertFloat(t *testing.T, left float64, right float64, tol float64) {
	assert.True(t, math.Abs(left-right) < tol, fmt.Sprintf("abs(Left %v - Right %v) < Tol %v", left, right, tol))
}

const CountFloatTol float64 = 1e-6

func updateLatency(latencySeconds float64) {
	timeSinceSeconds = func(_ time.Time) float64 {
		return latencySeconds
	}
}

func TestOutboundRPCStartRecorder(t *testing.T) {

	recorder := OutboundRPCStartRecorder("test_kind_1")
	updateLatency(2)

	recorder(OutboundRPCStatus("test_status_1"))

	totalCount := testutil.CollectAndCount(outboundRPCCount)

	assert.Equal(t, 1, totalCount)
	// check the expected values using the ToFloat64 function
	assertFloat(t, 1, testutil.ToFloat64(outboundRPCCount.WithLabelValues("test_status_1", "test_kind_1")), CountFloatTol)

	expectedCountMetric := `
	# HELP outbound_rpc_count Count of outbound RPCs to GCP
    # TYPE outbound_rpc_count counter
    outbound_rpc_count{kind="test_kind_1",status="test_status_1"} 1
	`

	if err := testutil.CollectAndCompare(outboundRPCCount, strings.NewReader(expectedCountMetric)); err != nil {
		t.Errorf("unexpected collecting result:\n%s", err)
	}

	expectedLatencyHistogram := `
	# HELP outbound_rpc_latency Latency of outbound RPCs to GCP (in seconds)
	# TYPE outbound_rpc_latency histogram
	outbound_rpc_latency_bucket{kind="test_kind_1",status="test_status_1",le="0.005"} 0
	outbound_rpc_latency_bucket{kind="test_kind_1",status="test_status_1",le="0.01"} 0
	outbound_rpc_latency_bucket{kind="test_kind_1",status="test_status_1",le="0.025"} 0
	outbound_rpc_latency_bucket{kind="test_kind_1",status="test_status_1",le="0.05"} 0
	outbound_rpc_latency_bucket{kind="test_kind_1",status="test_status_1",le="0.1"} 0
	outbound_rpc_latency_bucket{kind="test_kind_1",status="test_status_1",le="0.25"} 0
	outbound_rpc_latency_bucket{kind="test_kind_1",status="test_status_1",le="0.5"} 0
	outbound_rpc_latency_bucket{kind="test_kind_1",status="test_status_1",le="1"} 0
	outbound_rpc_latency_bucket{kind="test_kind_1",status="test_status_1",le="2.5"} 1
	outbound_rpc_latency_bucket{kind="test_kind_1",status="test_status_1",le="5"} 1
	outbound_rpc_latency_bucket{kind="test_kind_1",status="test_status_1",le="10"} 1
	outbound_rpc_latency_bucket{kind="test_kind_1",status="test_status_1",le="+Inf"} 1
	outbound_rpc_latency_sum{kind="test_kind_1",status="test_status_1"} 2
	outbound_rpc_latency_count{kind="test_kind_1",status="test_status_1"} 1
	`

	if err := testutil.CollectAndCompare(outboundRPCLatency, strings.NewReader(expectedLatencyHistogram)); err != nil {
		t.Errorf("unexpected collecting result:\n%s", err)
	}

	type metricsLables struct {
		status string
		kind   string
	}

	metricsLabels := []metricsLables{
		{"test_status_1", "test_kind_1"},
		{"test_status_1", "test_kind_1"},
		{"test_status_2", "test_kind_1"},
		{"test_status_2", "test_kind_1"},
		{"test_status_2", "test_kind_2"},
		{"test_status_2", "test_kind_2"},
		{"test_status_2", "test_kind_1"},
		{"test_status_2", "test_kind_1"},
		{"test_status_2", "test_kind_2"},
		{"test_status_2", "test_kind_2"},
		{"test_status_1", "test_kind_2"},
		{"test_status_2", "test_kind_2"},
	}

	latencyArrayDTObjects := []time.Time{
		time.Date(2024, time.May, 1, 0, 0, 0, 1000, time.UTC),
		time.Date(2024, time.May, 1, 0, 0, 0, 10000000, time.UTC),
		time.Date(2024, time.May, 1, 0, 0, 0, 20000000, time.UTC),
		time.Date(2024, time.May, 1, 0, 0, 0, 50000000, time.UTC),
		time.Date(2024, time.May, 1, 0, 0, 0, 90000000, time.UTC),
		time.Date(2024, time.May, 1, 0, 0, 0, 200000000, time.UTC),
		time.Date(2024, time.May, 1, 0, 0, 0, 500000000, time.UTC),
		time.Date(2024, time.May, 1, 0, 0, 0, 900000000, time.UTC),
		time.Date(2024, time.May, 1, 0, 0, 2, 0, time.UTC),
		time.Date(2024, time.May, 1, 0, 0, 4, 0, time.UTC),
		time.Date(2024, time.May, 1, 0, 0, 9, 0, time.UTC),
		time.Date(2024, time.May, 1, 0, 0, 19, 0, time.UTC),
	}

	latencyArrayDTObjectsSeconds := []float64{
		0.0049,
		0.009,
		0.024,
		0.04,
		0.09,
		0.24,
		0.4,
		0.9,
		2.4,
		4,
		9,
		19,
	}

	for i := range latencyArrayDTObjects {
		metricsLable := metricsLabels[i]
		recorder := OutboundRPCStartRecorder(metricsLable.kind)
		updateLatency(latencyArrayDTObjectsSeconds[i])

		recorder(OutboundRPCStatus(metricsLable.status))
	}

	expectedCountMetric = `
	# HELP outbound_rpc_count Count of outbound RPCs to GCP
    # TYPE outbound_rpc_count counter
    outbound_rpc_count{kind="test_kind_1",status="test_status_1"} 3
    outbound_rpc_count{kind="test_kind_1",status="test_status_2"} 4
    outbound_rpc_count{kind="test_kind_2",status="test_status_1"} 1
    outbound_rpc_count{kind="test_kind_2",status="test_status_2"} 5
	`

	if err := testutil.CollectAndCompare(outboundRPCCount, strings.NewReader(expectedCountMetric)); err != nil {
		t.Errorf("unexpected collecting result:\n%s", err)
	}

	exppectedLatencyHistograms := `
	# HELP outbound_rpc_latency Latency of outbound RPCs to GCP (in seconds)
    # TYPE outbound_rpc_latency histogram
    outbound_rpc_latency_bucket{kind="test_kind_1",status="test_status_1",le="0.005"} 1
    outbound_rpc_latency_bucket{kind="test_kind_1",status="test_status_1",le="0.01"} 2
    outbound_rpc_latency_bucket{kind="test_kind_1",status="test_status_1",le="0.025"} 2
    outbound_rpc_latency_bucket{kind="test_kind_1",status="test_status_1",le="0.05"} 2
    outbound_rpc_latency_bucket{kind="test_kind_1",status="test_status_1",le="0.1"} 2
    outbound_rpc_latency_bucket{kind="test_kind_1",status="test_status_1",le="0.25"} 2
    outbound_rpc_latency_bucket{kind="test_kind_1",status="test_status_1",le="0.5"} 2
    outbound_rpc_latency_bucket{kind="test_kind_1",status="test_status_1",le="1"} 2
    outbound_rpc_latency_bucket{kind="test_kind_1",status="test_status_1",le="2.5"} 3
    outbound_rpc_latency_bucket{kind="test_kind_1",status="test_status_1",le="5"} 3
    outbound_rpc_latency_bucket{kind="test_kind_1",status="test_status_1",le="10"} 3
    outbound_rpc_latency_bucket{kind="test_kind_1",status="test_status_1",le="+Inf"} 3
    outbound_rpc_latency_sum{kind="test_kind_1",status="test_status_1"} 2.0139
    outbound_rpc_latency_count{kind="test_kind_1",status="test_status_1"} 3
    outbound_rpc_latency_bucket{kind="test_kind_1",status="test_status_2",le="0.005"} 0
    outbound_rpc_latency_bucket{kind="test_kind_1",status="test_status_2",le="0.01"} 0
    outbound_rpc_latency_bucket{kind="test_kind_1",status="test_status_2",le="0.025"} 1
    outbound_rpc_latency_bucket{kind="test_kind_1",status="test_status_2",le="0.05"} 2
    outbound_rpc_latency_bucket{kind="test_kind_1",status="test_status_2",le="0.1"} 2
    outbound_rpc_latency_bucket{kind="test_kind_1",status="test_status_2",le="0.25"} 2
    outbound_rpc_latency_bucket{kind="test_kind_1",status="test_status_2",le="0.5"} 3
    outbound_rpc_latency_bucket{kind="test_kind_1",status="test_status_2",le="1"} 4
    outbound_rpc_latency_bucket{kind="test_kind_1",status="test_status_2",le="2.5"} 4
    outbound_rpc_latency_bucket{kind="test_kind_1",status="test_status_2",le="5"} 4
    outbound_rpc_latency_bucket{kind="test_kind_1",status="test_status_2",le="10"} 4
    outbound_rpc_latency_bucket{kind="test_kind_1",status="test_status_2",le="+Inf"} 4
    outbound_rpc_latency_sum{kind="test_kind_1",status="test_status_2"} 1.364
    outbound_rpc_latency_count{kind="test_kind_1",status="test_status_2"} 4
    outbound_rpc_latency_bucket{kind="test_kind_2",status="test_status_1",le="0.005"} 0
    outbound_rpc_latency_bucket{kind="test_kind_2",status="test_status_1",le="0.01"} 0
    outbound_rpc_latency_bucket{kind="test_kind_2",status="test_status_1",le="0.025"} 0
    outbound_rpc_latency_bucket{kind="test_kind_2",status="test_status_1",le="0.05"} 0
    outbound_rpc_latency_bucket{kind="test_kind_2",status="test_status_1",le="0.1"} 0
    outbound_rpc_latency_bucket{kind="test_kind_2",status="test_status_1",le="0.25"} 0
    outbound_rpc_latency_bucket{kind="test_kind_2",status="test_status_1",le="0.5"} 0
    outbound_rpc_latency_bucket{kind="test_kind_2",status="test_status_1",le="1"} 0
    outbound_rpc_latency_bucket{kind="test_kind_2",status="test_status_1",le="2.5"} 0
    outbound_rpc_latency_bucket{kind="test_kind_2",status="test_status_1",le="5"} 0
    outbound_rpc_latency_bucket{kind="test_kind_2",status="test_status_1",le="10"} 1
    outbound_rpc_latency_bucket{kind="test_kind_2",status="test_status_1",le="+Inf"} 1
    outbound_rpc_latency_sum{kind="test_kind_2",status="test_status_1"} 9
    outbound_rpc_latency_count{kind="test_kind_2",status="test_status_1"} 1
    outbound_rpc_latency_bucket{kind="test_kind_2",status="test_status_2",le="0.005"} 0
    outbound_rpc_latency_bucket{kind="test_kind_2",status="test_status_2",le="0.01"} 0
    outbound_rpc_latency_bucket{kind="test_kind_2",status="test_status_2",le="0.025"} 0
    outbound_rpc_latency_bucket{kind="test_kind_2",status="test_status_2",le="0.05"} 0
    outbound_rpc_latency_bucket{kind="test_kind_2",status="test_status_2",le="0.1"} 1
    outbound_rpc_latency_bucket{kind="test_kind_2",status="test_status_2",le="0.25"} 2
    outbound_rpc_latency_bucket{kind="test_kind_2",status="test_status_2",le="0.5"} 2
    outbound_rpc_latency_bucket{kind="test_kind_2",status="test_status_2",le="1"} 2
    outbound_rpc_latency_bucket{kind="test_kind_2",status="test_status_2",le="2.5"} 3
    outbound_rpc_latency_bucket{kind="test_kind_2",status="test_status_2",le="5"} 4
    outbound_rpc_latency_bucket{kind="test_kind_2",status="test_status_2",le="10"} 4
    outbound_rpc_latency_bucket{kind="test_kind_2",status="test_status_2",le="+Inf"} 5
    outbound_rpc_latency_sum{kind="test_kind_2",status="test_status_2"} 25.73
    outbound_rpc_latency_count{kind="test_kind_2",status="test_status_2"} 5
	`

	if err := testutil.CollectAndCompare(outboundRPCLatency, strings.NewReader(exppectedLatencyHistograms)); err != nil {
		t.Errorf("unexpected collecting result:\n%s", err)
	}

}
