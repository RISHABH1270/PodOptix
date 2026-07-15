package collector

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// ContainerMetrics holds raw CPU and memory data points for a single container.
type ContainerMetrics struct {
	Namespace     string
	PodName       string
	ContainerName string
	CPUValues     []float64 // millicores — collected over lookback window
	MemValues     []float64 // MiB — collected over lookback window
}

// Collector queries a Prometheus endpoint and returns raw metrics per container.
type Collector struct {
	prometheusURL string
	token         string
	httpClient    *http.Client
}

// New creates a new Collector for the given Prometheus endpoint.
func New(prometheusURL string, token string) *Collector {
	return &Collector{
		prometheusURL: prometheusURL,
		token:         token,
		httpClient: &http.Client{
			Timeout: 30 * time.Second, // fail fast if Prometheus is unresponsiv - 30 seconds
		},
	}
}

// Collect queries Prometheus for CPU and memory usage of all containers
func (c *Collector) Collect(ctx context.Context, lookbackWindow string) ([]*ContainerMetrics, error) {
	// calculate time range
	end := time.Now()
	duration, err := parseDuration(lookbackWindow)
	if err != nil {
		return nil, fmt.Errorf("parse lookback window: %w", err)
	}
	start := end.Add(-duration)

	// query CPU and memory
	cpuData, err := c.queryRange(ctx,
		`rate(container_cpu_usage_seconds_total{container!="",container!="POD"}[5m]) * 1000`,
		start, end,
	)
	if err != nil {
		return nil, fmt.Errorf("query cpu: %w", err)
	}

	memData, err := c.queryRange(ctx,
		`container_memory_working_set_bytes{container!="",container!="POD"} / 1048576`,
		start, end,
	)
	if err != nil {
		return nil, fmt.Errorf("query memory: %w", err)
	}

	// merge CPU and memory data by container identity
	return mergeMetrics(cpuData, memData), nil
}

// prometheusResult represents a single time series returned by Prometheus.
type prometheusResult struct {
	Metric map[string]string `json:"metric"`
	Values [][]interface{}   `json:"values"` // [[timestamp, "value"], ...]
}

// prometheusResponse is the full JSON response from /api/v1/query_range.
type prometheusResponse struct {
	Status string `json:"status"`
	Data   struct {
		Result []prometheusResult `json:"result"`
	} `json:"data"`
}

// queryRange calls Prometheus /api/v1/query_range and returns raw results.
func (c *Collector) queryRange(ctx context.Context, query string, start, end time.Time) ([]prometheusResult, error) {
	// build the request URL with query parameters
	endpoint := c.prometheusURL + "/api/v1/query_range"
	params := url.Values{}
	params.Set("query", query)
	params.Set("start", start.UTC().Format(time.RFC3339))
	params.Set("end", end.UTC().Format(time.RFC3339))
	params.Set("step", "3600") // one data point per hour

	var req *http.Request
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint+"?"+params.Encode(), nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}

	// attach auth token if provided
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	// execute the request
	var resp *http.Response
	resp, err = c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("prometheus returned status %d", resp.StatusCode)
	}

	// parse the JSON response
	var body []byte
	body, err = io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	var promResp prometheusResponse
	if err = json.Unmarshal(body, &promResp); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	if promResp.Status != "success" {
		return nil, fmt.Errorf("prometheus query failed: status=%s", promResp.Status)
	}

	return promResp.Data.Result, nil
}

// mergeMetrics combines CPU and memory results into ContainerMetrics per container.
func mergeMetrics(cpuResults, memResults []prometheusResult) []*ContainerMetrics {
	// index CPU results by container identity key
	type containerKey struct {
		namespace, pod, container string
	}

	cpuMap := make(map[containerKey][]float64)
	for _, r := range cpuResults {
		key := containerKey{
			namespace: r.Metric["namespace"],
			pod:       r.Metric["pod"],
			container: r.Metric["container"],
		}
		cpuMap[key] = extractValues(r.Values)
	}

	// build ContainerMetrics by matching CPU and memory
	var metrics []*ContainerMetrics
	for _, r := range memResults {
		key := containerKey{
			namespace: r.Metric["namespace"],
			pod:       r.Metric["pod"],
			container: r.Metric["container"],
		}
		metrics = append(metrics, &ContainerMetrics{
			Namespace:     key.namespace,
			PodName:       key.pod,
			ContainerName: key.container,
			CPUValues:     cpuMap[key],
			MemValues:     extractValues(r.Values),
		})
	}
	return metrics
}

// extractValues converts Prometheus [[timestamp, "value"]] pairs to []float64.
func extractValues(values [][]interface{}) []float64 {
	var result []float64
	for _, v := range values {
		if len(v) != 2 {
			continue
		}
		// value is a string like "0.120" — parse to float64
		str, ok := v[1].(string)
		if !ok {
			continue
		}
		f, err := strconv.ParseFloat(str, 64)
		if err != nil {
			continue
		}
		result = append(result, f)
	}
	return result
}

// parseDuration converts "7d", "24h" etc. to time.Duration.
func parseDuration(s string) (time.Duration, error) {
	if len(s) < 2 {
		return 0, fmt.Errorf("invalid duration: %s", s)
	}
	value, err := strconv.Atoi(s[:len(s)-1])
	if err != nil {
		return 0, fmt.Errorf("invalid duration value: %s", s)
	}
	unit := s[len(s)-1]
	switch unit {
	case 'd':
		return time.Duration(value) * 24 * time.Hour, nil
	case 'h':
		return time.Duration(value) * time.Hour, nil
	case 'm':
		return time.Duration(value) * time.Minute, nil
	default:
		return 0, fmt.Errorf("unknown duration unit: %c (use d, h or m)", unit)
	}
}
