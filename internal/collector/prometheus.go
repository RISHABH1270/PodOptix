package collector

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// ContainerMetrics holds raw CPU/memory usage time series and current resource limits for a single container.
type ContainerMetrics struct {
	Namespace     string
	PodName       string
	ContainerName string
	CPUValues     []float64 // millicores — usage over lookback window
	MemValues     []float64 // MiB       — usage over lookback window
	CPULimit      int       // millicores — current limit from kube_pod_container_resource_limits (0 if unset)
	MemLimit      int       // MiB       — current limit from kube_pod_container_resource_limits (0 if unset)
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
			Timeout: 30 * time.Second, // fail fast if Prometheus is unresponsive
		},
	}
}

// Ping checks reachability and auth by running a lightweight instant query against Prometheus.
// Used during cluster registration and update to set initial connectivity status.
func (c *Collector) Ping(ctx context.Context) error {
	endpoint := c.prometheusURL + "/api/v1/query"
	params := url.Values{}
	params.Set("query", "up")
	params.Set("time", fmt.Sprintf("%d", time.Now().Unix()))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint+"?"+params.Encode(), nil)
	if err != nil {
		return fmt.Errorf("build ping request: %w", err)
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("prometheus unreachable: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("prometheus returned status %d", resp.StatusCode)
	}
	return nil
}

// Collect queries Prometheus for CPU/memory usage and current resource limits for all containers.
func (c *Collector) Collect(ctx context.Context, lookbackWindow string) ([]*ContainerMetrics, error) {
	end := time.Now()
	duration, err := parseDuration(lookbackWindow)
	if err != nil {
		return nil, fmt.Errorf("parse lookback window: %w", err)
	}
	start := end.Add(-duration)

	cpuData, err := c.queryRange(ctx,
		`rate(container_cpu_usage_seconds_total{container!="",container!="POD"}[5m]) * 1000`,
		start, end,
	)
	if err != nil {
		return nil, fmt.Errorf("query cpu usage: %w", err)
	}

	memData, err := c.queryRange(ctx,
		`container_memory_working_set_bytes{container!="",container!="POD"} / 1048576`,
		start, end,
	)
	if err != nil {
		return nil, fmt.Errorf("query memory usage: %w", err)
	}

	// query current resource limits from kube-state-metrics — gracefully returns empty if not installed
	cpuLimits, _ := c.queryInstant(ctx,
		`kube_pod_container_resource_limits{resource="cpu",container!="",container!="POD"} * 1000`,
	)
	memLimits, _ := c.queryInstant(ctx,
		`kube_pod_container_resource_limits{resource="memory",container!="",container!="POD"} / 1048576`,
	)

	return mergeMetrics(cpuData, memData, cpuLimits, memLimits), nil
}

// prometheusResult represents a single time series returned by /api/v1/query_range.
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

// prometheusInstantResult represents a single vector sample from /api/v1/query.
type prometheusInstantResult struct {
	Metric map[string]string `json:"metric"`
	Value  []interface{}     `json:"value"` // [timestamp, "value"]
}

// prometheusInstantResponse is the full JSON response from /api/v1/query.
type prometheusInstantResponse struct {
	Status string `json:"status"`
	Data   struct {
		Result []prometheusInstantResult `json:"result"`
	} `json:"data"`
}

// queryRange calls Prometheus /api/v1/query_range and returns raw results.
func (c *Collector) queryRange(ctx context.Context, query string, start, end time.Time) ([]prometheusResult, error) {
	params := url.Values{}
	params.Set("query", query)
	params.Set("start", start.UTC().Format(time.RFC3339))
	params.Set("end", end.UTC().Format(time.RFC3339))
	params.Set("step", "3600") // one data point per hour

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.prometheusURL+"/api/v1/query_range?"+params.Encode(), nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("prometheus returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
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

// queryInstant calls Prometheus /api/v1/query (instant query) and returns raw results.
// Used for current resource limits — a single value per container, not a time series.
func (c *Collector) queryInstant(ctx context.Context, query string) ([]prometheusInstantResult, error) {
	endpoint := c.prometheusURL + "/api/v1/query"
	params := url.Values{}
	params.Set("query", query)
	params.Set("time", fmt.Sprintf("%d", time.Now().Unix()))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint+"?"+params.Encode(), nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("prometheus returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	var promResp prometheusInstantResponse
	if err = json.Unmarshal(body, &promResp); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}
	if promResp.Status != "success" {
		return nil, fmt.Errorf("prometheus query failed: status=%s", promResp.Status)
	}
	return promResp.Data.Result, nil
}

type containerKey struct {
	namespace, pod, container string
}

// mergeMetrics combines CPU/memory usage time series and current resource limits into ContainerMetrics per container.
func mergeMetrics(
	cpuResults, memResults []prometheusResult,
	cpuLimitResults, memLimitResults []prometheusInstantResult,
) []*ContainerMetrics {
	cpuMap := make(map[containerKey][]float64)
	for _, r := range cpuResults {
		key := containerKey{r.Metric["namespace"], r.Metric["pod"], r.Metric["container"]}
		cpuMap[key] = extractValues(r.Values)
	}

	cpuLimitMap := make(map[containerKey]int)
	for _, r := range cpuLimitResults {
		key := containerKey{r.Metric["namespace"], r.Metric["pod"], r.Metric["container"]}
		if len(r.Value) == 2 {
			if s, ok := r.Value[1].(string); ok {
				if f, err := strconv.ParseFloat(s, 64); err == nil {
					cpuLimitMap[key] = int(math.Ceil(f))
				}
			}
		}
	}

	memLimitMap := make(map[containerKey]int)
	for _, r := range memLimitResults {
		key := containerKey{r.Metric["namespace"], r.Metric["pod"], r.Metric["container"]}
		if len(r.Value) == 2 {
			if s, ok := r.Value[1].(string); ok {
				if f, err := strconv.ParseFloat(s, 64); err == nil {
					memLimitMap[key] = int(math.Ceil(f))
				}
			}
		}
	}

	var metrics []*ContainerMetrics
	for _, r := range memResults {
		key := containerKey{r.Metric["namespace"], r.Metric["pod"], r.Metric["container"]}
		metrics = append(metrics, &ContainerMetrics{
			Namespace:     key.namespace,
			PodName:       key.pod,
			ContainerName: key.container,
			CPUValues:     cpuMap[key],
			MemValues:     extractValues(r.Values),
			CPULimit:      cpuLimitMap[key],
			MemLimit:      memLimitMap[key],
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
