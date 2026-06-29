package scheduler

import (
	"context"
	"log"
	"time"

	"github.com/RISHABH1270/PodOptix/internal/collector"
	"github.com/RISHABH1270/PodOptix/internal/recommender"
	"github.com/RISHABH1270/PodOptix/internal/store"
)

// Scheduler runs the collection pipeline once per day for every registered cluster.
type Scheduler struct {
	store    *store.Store
	interval time.Duration
}

// New creates a new Scheduler.
// interval is how often to run — use 24 * time.Hour for production.
func New(st *store.Store, interval time.Duration) *Scheduler {
	return &Scheduler{
		store:    st,
		interval: interval,
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

// runAll fetches all clusters and runs the full pipeline for each one.
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

	for _, cluster := range clusters {
		s.runForCluster(ctx, cluster.ClusterID, cluster.PrometheusURL, cluster.Token, cluster.LookbackWindow)
	}
}

// runForCluster runs the full collect → recommend → upsert pipeline for one cluster.
func (s *Scheduler) runForCluster(ctx context.Context, clusterID, prometheusURL, token, lookbackWindow string) {
	log.Printf("INFO  scheduler collecting cluster=%s", clusterID)

	// step 1 — collect raw metrics from Prometheus
	c := collector.New(prometheusURL, token)
	metrics, err := c.Collect(ctx, lookbackWindow)
	if err != nil {
		log.Printf("ERROR scheduler collect cluster=%s: %v", clusterID, err)
		return
	}

	log.Printf("INFO  scheduler collected %d containers from cluster=%s", len(metrics), clusterID)

	// step 2 — generate recommendations
	recommendations, err := recommender.GenerateAll(clusterID, lookbackWindow, metrics)
	if err != nil {
		log.Printf("ERROR scheduler recommend cluster=%s: %v", clusterID, err)
		return
	}

	// step 3 — upsert recommendations to database
	var saved int
	for _, rec := range recommendations {
		if err = s.store.UpsertRecommendation(ctx, rec); err != nil {
			log.Printf("ERROR scheduler upsert cluster=%s pod=%s container=%s: %v",
				clusterID, rec.PodName, rec.ContainerName, err)
			continue
		}
		saved++
	}

	log.Printf("INFO  scheduler saved %d/%d recommendations for cluster=%s",
		saved, len(recommendations), clusterID)
}
