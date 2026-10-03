package collector

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"time"
)

// ContainerMetrics holds raw CPU/memory usage + current resource requests/limits for a single
// container within a WORKLOAD (a PodController — Deployment, StatefulSet, DaemonSet, or bare Pod).
//
// CPUValues/MemValues are the MAX-across-replicas aggregate per timestamp — if a Deployment has
// 3 replicas, each timestamp holds the loudest replica's usage. Request/Limit are MAX across
// replicas too (if any replica has a stale higher limit, that's what the current state is).
type ContainerMetrics struct {
	Namespace     string
	WorkloadKind  string    // Deployment | StatefulSet | DaemonSet | Pod
	WorkloadName  string    // e.g. "auth-service" — resolved from pod owner chain
	ContainerName string
	ReplicaCount  int       // how many replicas aggregated — 1 for bare pods, N for scaled workloads
	CPUValues     []float64 // millicores — usage over lookback window (max across replicas per timestamp)
	MemValues     []float64 // MiB       — usage over lookback window (max across replicas per timestamp)
	CPURequest    int       // millicores — current request from kube_pod_container_resource_requests (0 if unset)
	CPULimit      int       // millicores — current limit from kube_pod_container_resource_limits (0 if unset)
	MemRequest    int       // MiB       — current request from kube_pod_container_resource_requests (0 if unset)
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
	startedAt := time.Now()
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
	log.Printf("INFO  collector ping ok prometheus=%s took=%s", c.prometheusURL, time.Since(startedAt).Truncate(time.Millisecond))
	return nil
}

// Collect queries Prometheus for CPU/memory usage and current resource limits for all containers.
func (c *Collector) Collect(ctx context.Context, lookbackWindow string) ([]*ContainerMetrics, error) {
	startedAt := time.Now()
	end := startedAt
	duration, err := ParseDuration(lookbackWindow)
	if err != nil {
		return nil, fmt.Errorf("parse lookback window: %w", err)
	}
	start := end.Add(-duration)
	log.Printf("INFO  collector fetching prometheus=%s lookback=%s", c.prometheusURL, lookbackWindow)

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

	// query current resource requests + limits from kube-state-metrics — gracefully returns empty if not installed
	cpuRequests, _ := c.queryInstant(ctx,
		`kube_pod_container_resource_requests{resource="cpu",container!="",container!="POD"} * 1000`,
	)
	cpuLimits, _ := c.queryInstant(ctx,
		`kube_pod_container_resource_limits{resource="cpu",container!="",container!="POD"} * 1000`,
	)
	memRequests, _ := c.queryInstant(ctx,
		`kube_pod_container_resource_requests{resource="memory",container!="",container!="POD"} / 1048576`,
	)
	memLimits, _ := c.queryInstant(ctx,
		`kube_pod_container_resource_limits{resource="memory",container!="",container!="POD"} / 1048576`,
	)

	// Resolve the pod → workload mapping (ReplicaSet → Deployment collapse).
	// Degrades gracefully: if kube_pod_owner isn't available (kube-state-metrics missing),
	// every pod resolves to ("Pod", pod-name) and we behave like Phase 1.
	podOwnerResults, err := c.queryInstant(ctx, `kube_pod_owner`)
	if err != nil {
		log.Printf("WARN  collector kube_pod_owner unavailable, falling back to per-pod: %v", err)
	}
	rsOwnerResults, err := c.queryInstant(ctx, `kube_replicaset_owner{owner_kind="Deployment"}`)
	if err != nil {
		log.Printf("WARN  collector kube_replicaset_owner unavailable, ReplicaSet names will not be collapsed: %v", err)
	}
	podOwners := buildOwnerMap(podOwnerResults, rsOwnerResults)

	metrics := mergeMetrics(cpuData, memData, cpuRequests, cpuLimits, memRequests, memLimits, podOwners)
	log.Printf("INFO  collector done workloads=%d took=%s", len(metrics), time.Since(startedAt).Truncate(time.Millisecond))
	return metrics, nil
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

// podKey identifies a pod within a cluster snapshot: (namespace, pod-name).
type podKey struct {
	namespace, pod string
}

// workloadKey identifies a container within a workload: (namespace, kind, name, container).
// One ContainerMetrics is produced per workloadKey after aggregating across replicas.
type workloadKey struct {
	namespace, kind, name, container string
}

// ownerRef is the resolved owner of a pod — the K8s object that should receive the recommendation.
type ownerRef struct {
	kind, name string
}

// buildOwnerMap resolves every pod to its owning WORKLOAD using kube-state-metrics data.
//
// Chain: pod → kube_pod_owner (ReplicaSet | StatefulSet | DaemonSet | Job | <none>)
//        ReplicaSet → kube_replicaset_owner (Deployment) — so pods owned by a ReplicaSet
//        bubble up to their Deployment and all replicas across RS revisions collapse together.
//
// Pods missing from the map (fallback) are treated as bare Pods by resolve().
func buildOwnerMap(podOwnerResults, rsOwnerResults []prometheusInstantResult) map[podKey]ownerRef {
	podOwners := make(map[podKey]ownerRef, len(podOwnerResults))
	for _, r := range podOwnerResults {
		ns := r.Metric["namespace"]
		pod := r.Metric["pod"]
		kind := r.Metric["owner_kind"]
		name := r.Metric["owner_name"]
		if ns == "" || pod == "" || kind == "" || name == "" {
			continue
		}
		podOwners[podKey{ns, pod}] = ownerRef{kind, name}
	}

	// ReplicaSet → Deployment collapse (same keyspace as pods — namespace + rs name).
	rsToDeployment := make(map[podKey]string, len(rsOwnerResults))
	for _, r := range rsOwnerResults {
		ns := r.Metric["namespace"]
		rs := r.Metric["replicaset"]
		dep := r.Metric["owner_name"]
		if ns == "" || rs == "" || dep == "" {
			continue
		}
		rsToDeployment[podKey{ns, rs}] = dep
	}
	for pk, own := range podOwners {
		if own.kind == "ReplicaSet" {
			if dep, ok := rsToDeployment[podKey{pk.namespace, own.name}]; ok {
				podOwners[pk] = ownerRef{"Deployment", dep}
			}
			// else: orphan RS (no Deployment parent) — keep RS as the workload
		}
	}
	return podOwners
}

// resolveWorkload maps (ns, pod) → (workloadKind, workloadName).
// Falls back to ("Pod", pod) when the owner isn't known.
func resolveWorkload(owners map[podKey]ownerRef, ns, pod string) (string, string) {
	if o, ok := owners[podKey{ns, pod}]; ok {
		return o.kind, o.name
	}
	return "Pod", pod
}

// timeSeries maps unix timestamp → value. Enables MAX-across-replicas aggregation
// by merging multiple pods' series on shared timestamps (step=3600 keeps them aligned).
type timeSeries map[int64]float64

// mergeMax merges other into ts, taking max per timestamp.
func (ts timeSeries) mergeMax(other timeSeries) {
	for t, v := range other {
		if ex, ok := ts[t]; !ok || v > ex {
			ts[t] = v
		}
	}
}

// flatten returns the values ordered by timestamp ascending.
func (ts timeSeries) flatten() []float64 {
	keys := make([]int64, 0, len(ts))
	for k := range ts {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
	out := make([]float64, 0, len(keys))
	for _, k := range keys {
		out = append(out, ts[k])
	}
	return out
}

// extractTimeSeries parses Prometheus [[ts, "val"]] pairs into a timeSeries map.
func extractTimeSeries(values [][]interface{}) timeSeries {
	out := make(timeSeries, len(values))
	for _, v := range values {
		if len(v) != 2 {
			continue
		}
		tsF, ok := v[0].(float64)
		if !ok {
			continue
		}
		valStr, ok := v[1].(string)
		if !ok {
			continue
		}
		val, err := strconv.ParseFloat(valStr, 64)
		if err != nil {
			continue
		}
		out[int64(tsF)] = val
	}
	return out
}

// mergeMetrics combines CPU/memory usage + current requests/limits across REPLICAS
// into one ContainerMetrics per workload-container.
//
//   usage(t) → MAX of each replica's value at timestamp t
//   request/limit → MAX across replicas (recommend against the loudest current config)
//   ReplicaCount → number of distinct pods observed for the workload-container
func mergeMetrics(
	cpuResults, memResults []prometheusResult,
	cpuRequestResults, cpuLimitResults, memRequestResults, memLimitResults []prometheusInstantResult,
	podOwners map[podKey]ownerRef,
) []*ContainerMetrics {
	cpuSeries := make(map[workloadKey]timeSeries)
	memSeries := make(map[workloadKey]timeSeries)
	replicas := make(map[workloadKey]map[string]struct{})

	track := func(wk workloadKey, pod string) {
		if replicas[wk] == nil {
			replicas[wk] = make(map[string]struct{})
		}
		replicas[wk][pod] = struct{}{}
	}

	aggRange := func(results []prometheusResult, dst map[workloadKey]timeSeries) {
		for _, r := range results {
			ns := r.Metric["namespace"]
			pod := r.Metric["pod"]
			ctr := r.Metric["container"]
			if ns == "" || pod == "" || ctr == "" {
				continue
			}
			kind, name := resolveWorkload(podOwners, ns, pod)
			wk := workloadKey{ns, kind, name, ctr}
			if dst[wk] == nil {
				dst[wk] = make(timeSeries)
			}
			dst[wk].mergeMax(extractTimeSeries(r.Values))
			track(wk, pod)
		}
	}
	aggRange(cpuResults, cpuSeries)
	aggRange(memResults, memSeries)

	// MAX across replicas for scalar request/limit values.
	toMaxIntMap := func(results []prometheusInstantResult) map[workloadKey]int {
		out := make(map[workloadKey]int)
		for _, r := range results {
			ns := r.Metric["namespace"]
			pod := r.Metric["pod"]
			ctr := r.Metric["container"]
			if ns == "" || pod == "" || ctr == "" || len(r.Value) != 2 {
				continue
			}
			s, ok := r.Value[1].(string)
			if !ok {
				continue
			}
			f, err := strconv.ParseFloat(s, 64)
			if err != nil {
				continue
			}
			v := int(math.Ceil(f))
			kind, name := resolveWorkload(podOwners, ns, pod)
			wk := workloadKey{ns, kind, name, ctr}
			if ex, ok := out[wk]; !ok || v > ex {
				out[wk] = v
			}
		}
		return out
	}
	cpuReq := toMaxIntMap(cpuRequestResults)
	cpuLim := toMaxIntMap(cpuLimitResults)
	memReq := toMaxIntMap(memRequestResults)
	memLim := toMaxIntMap(memLimitResults)

	// Driven by memSeries (as before — same semantics for the "has memory data" filter).
	var metrics []*ContainerMetrics
	for wk, ms := range memSeries {
		metrics = append(metrics, &ContainerMetrics{
			Namespace:     wk.namespace,
			WorkloadKind:  wk.kind,
			WorkloadName:  wk.name,
			ContainerName: wk.container,
			ReplicaCount:  len(replicas[wk]),
			CPUValues:     cpuSeries[wk].flatten(),
			MemValues:     ms.flatten(),
			CPURequest:    cpuReq[wk],
			CPULimit:      cpuLim[wk],
			MemRequest:    memReq[wk],
			MemLimit:      memLim[wk],
		})
	}
	return metrics
}

// extractValues converts Prometheus [[timestamp, "value"]] pairs to []float64.
func ExtractValues(values [][]interface{}) []float64 {
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
func ParseDuration(s string) (time.Duration, error) {
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
