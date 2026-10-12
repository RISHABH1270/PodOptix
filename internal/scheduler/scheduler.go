package scheduler

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/RISHABH1270/PodOptix/internal/auth"
	"github.com/RISHABH1270/PodOptix/internal/cache"
	"github.com/RISHABH1270/PodOptix/internal/collector"
	"github.com/RISHABH1270/PodOptix/internal/metrics"
	"github.com/RISHABH1270/PodOptix/internal/recommender"
	"github.com/RISHABH1270/PodOptix/internal/store"
	"github.com/RISHABH1270/PodOptix/pkg/models"
)

// maxConcurrentClusterRuns caps how many clusters the scheduler processes in
// parallel per tick. Picked to balance speed against Prometheus / DB load:
// too low wastes the ticker window; too high can hammer every cluster's
// Prometheus simultaneously (we query 8 PromQL endpoints per run × N clusters).
// Hardcoded for now — expose as env var if someone deploys with 50+ clusters.
const maxConcurrentClusterRuns = 5

// Scheduler runs the collection pipeline once per day for every registered cluster.
// Shares the per-cluster recalculate lock with the API's /recalculate handler
// so a scheduled tick + a manual click can never run concurrently on the same cluster.
type Scheduler struct {
	store         *store.Store
	cache         *cache.Cache // nil in tests — lock acquisition is skipped when nil
	interval      time.Duration
	encryptionKey string
}

// New creates a new Scheduler. cache may be nil (lock skipped — fine for tests).
func New(st *store.Store, ca *cache.Cache, interval time.Duration, encryptionKey string) *Scheduler {
	return &Scheduler{
		store:         st,
		cache:         ca,
		interval:      interval,
		encryptionKey: encryptionKey,
	}
}

// Start begins the scheduler loop. Runs once immediately on startup,
// then repeats every interval. Stops when ctx is cancelled.
func (s *Scheduler) Start(ctx context.Context) {
	log.Printf("INFO  scheduler started — interval: %s", s.interval)

	// run immediately on startup — don't make users wait 24h for first data
	s.runAll(ctx)

	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.runAll(ctx)
		case <-ctx.Done():
			log.Printf("INFO  scheduler stopped")
			return
		}
	}
}

// runAll fetches all clusters and runs the full pipeline for each one in parallel,
// capped at maxConcurrentClusterRuns. Returns only after every cluster is done
// (or ctx is cancelled) so the next ticker tick never overlaps the current one.
func (s *Scheduler) runAll(ctx context.Context) {
	log.Printf("INFO  scheduler running collection for all clusters")

	clusters, err := s.store.ListClusters(ctx)
	if err != nil {
		log.Printf("ERROR scheduler list clusters: %v", err)
		return
	}

	if len(clusters) == 0 {
		log.Printf("INFO  scheduler no clusters registered — skipping")
		return
	}

	log.Printf("INFO  scheduler processing %d clusters (max %d in parallel)", len(clusters), maxConcurrentClusterRuns)

	// Buffered channel acts as a counting semaphore — send to acquire a slot,
	// receive to release. WaitGroup blocks runAll until every goroutine finishes.
	sem := make(chan struct{}, maxConcurrentClusterRuns)
	var wg sync.WaitGroup
	for _, cluster := range clusters {
		wg.Add(1)
		sem <- struct{}{} // acquire — blocks here once maxConcurrent are in flight
		go func(cluster *models.Cluster) {
			defer wg.Done()
			defer func() { <-sem }() // release

			plainToken, err := auth.Decrypt(cluster.PrometheusToken, s.encryptionKey)
			if err != nil {
				log.Printf("ERROR scheduler decrypt token cluster=%s: %v", cluster.ClusterID, err)
				return
			}
			s.RunForCluster(ctx, cluster.ClusterID, cluster.PrometheusURL, plainToken, cluster.LookbackWindow)
		}(cluster)
	}
	wg.Wait()
}

// RunForCluster runs the full collect → recommend → upsert pipeline for one cluster.
// Called by the scheduler loop and directly after cluster registration for immediate first sync.
// Uses a 10 minute timeout so a hanging Prometheus never blocks the full run.
//
// Acquires the shared per-cluster recalculate lock. If another run (scheduler tick
// or manual recalculate) is already in progress, logs and returns silently — no
// double Prometheus load, no racing on MarkOrphaned.
func (s *Scheduler) RunForCluster(ctx context.Context, clusterID, prometheusURL, token, lookbackWindow string) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()

	// Lock acquisition — skipped when cache is nil (test setup).
	// Captures the fencing token so Release can CAS against it (prevents us from
	// deleting someone else's lock if ours expired while the pipeline ran).
	if s.cache != nil {
		token, locked, err := s.cache.AcquireRecalculateLock(ctx, clusterID)
		if err != nil {
			log.Printf("WARN  scheduler lock error cluster=%s: %v", clusterID, err)
		}
		if !locked {
			log.Printf("INFO  scheduler skipped cluster=%s — another run in progress", clusterID)
			return
		}
		defer func() {
			if err := s.cache.ReleaseRecalculateLock(ctx, clusterID, token); err != nil {
				log.Printf("WARN  scheduler release lock cluster=%s: %v", clusterID, err)
			}
		}()
	}

	start := time.Now()
	log.Printf("INFO  scheduler collecting cluster=%s", clusterID)

	containerMetrics, err := collector.New(prometheusURL, token).Collect(ctx, lookbackWindow)
	if err != nil {
		log.Printf("ERROR scheduler collect cluster=%s: %v", clusterID, err)
		metrics.SchedulerRunsTotal.WithLabelValues("failure").Inc()
		metrics.SchedulerRunDuration.Observe(time.Since(start).Seconds())
		if err := s.store.UpdateClusterHealth(ctx, clusterID, models.ClusterStatusDisconnected, time.Now()); err != nil {
			log.Printf("WARN  scheduler health update cluster=%s: %v", clusterID, err)
		}
		return
	}

	log.Printf("INFO  scheduler collected %d containers from cluster=%s", len(containerMetrics), clusterID)
	metrics.SchedulerContainersScanned.Add(float64(len(containerMetrics)))

	recommendations, err := recommender.GenerateAll(clusterID, containerMetrics)
	if err != nil {
		log.Printf("ERROR scheduler recommend cluster=%s: %v", clusterID, err)
		metrics.SchedulerRunsTotal.WithLabelValues("failure").Inc()
		metrics.SchedulerRunDuration.Observe(time.Since(start).Seconds())
		return
	}

	// IMPORTANT: append to seenKeys BEFORE the upsert. If an upsert transiently fails
	// and we skipped the seenKey, MarkOrphaned would stamp the workload as missing
	// even though Prometheus just confirmed it exists → false orphan on dashboard.
	// Prometheus observation is authoritative; DB blips self-heal on next tick.
	var saved int
	seenKeys := make([]models.WorkloadKey, 0, len(recommendations))
	for _, rec := range recommendations {
		seenKeys = append(seenKeys, models.WorkloadKey{
			Namespace:     rec.Namespace,
			WorkloadKind:  rec.WorkloadKind,
			WorkloadName:  rec.WorkloadName,
			ContainerName: rec.ContainerName,
		})
		if err = s.store.UpsertRecommendation(ctx, rec); err != nil {
			log.Printf("ERROR scheduler upsert cluster=%s workload=%s/%s container=%s: %v",
				clusterID, rec.WorkloadKind, rec.WorkloadName, rec.ContainerName, err)
			continue
		}
		saved++
	}

	log.Printf("INFO  scheduler saved %d/%d recommendations for cluster=%s", saved, len(recommendations), clusterID)

	// Mark anything we didn't see this run as orphaned (safety-gated: no-op if seenKeys empty).
	// Rows aren't deleted — the operator reviews orphans in the dashboard and deletes explicitly.
	// If the workload comes back next run, UpsertRecommendation clears first_missed_at back to NULL.
	orphaned, err := s.store.MarkOrphaned(ctx, clusterID, seenKeys)
	if err != nil {
		log.Printf("WARN  scheduler mark orphaned cluster=%s: %v", clusterID, err)
	} else if orphaned > 0 {
		log.Printf("INFO  scheduler marked %d workloads as orphaned for cluster=%s", orphaned, clusterID)
	}

	if err := s.store.UpdateClusterHealth(ctx, clusterID, models.ClusterStatusConnected, time.Now()); err != nil {
		log.Printf("WARN  scheduler health update cluster=%s: %v", clusterID, err)
	}
	metrics.SchedulerRunsTotal.WithLabelValues("success").Inc()
	metrics.SchedulerRunDuration.Observe(time.Since(start).Seconds())
}
