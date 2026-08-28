package scheduler

import (
	"context"
	"log"
	"time"

	"github.com/RISHABH1270/PodOptix/internal/auth"
	"github.com/RISHABH1270/PodOptix/internal/collector"
	"github.com/RISHABH1270/PodOptix/internal/recommender"
	"github.com/RISHABH1270/PodOptix/internal/store"
	"github.com/RISHABH1270/PodOptix/pkg/models"
)

// Scheduler runs the collection pipeline once per day for every registered cluster.
type Scheduler struct {
	store         *store.Store
	interval      time.Duration
	encryptionKey string
}

// New creates a new Scheduler.
func New(st *store.Store, interval time.Duration, encryptionKey string) *Scheduler {
	return &Scheduler{
		store:         st,
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

// runAll fetches all clusters and runs the full pipeline for each one sequentially.
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
		plainToken, err := auth.Decrypt(cluster.PrometheusToken, s.encryptionKey)
		if err != nil {
			log.Printf("ERROR scheduler decrypt token cluster=%s: %v", cluster.ClusterID, err)
			continue
		}
		s.RunForCluster(ctx, cluster.ClusterID, cluster.PrometheusURL, plainToken, cluster.LookbackWindow)
	}
}

// RunForCluster runs the full collect → recommend → upsert pipeline for one cluster.
// Called by the scheduler loop and directly after cluster registration for immediate first sync.
// Uses a 10 minute timeout so a hanging Prometheus never blocks the full run.
func (s *Scheduler) RunForCluster(ctx context.Context, clusterID, prometheusURL, token, lookbackWindow string) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()

	log.Printf("INFO  scheduler collecting cluster=%s", clusterID)

	metrics, err := collector.New(prometheusURL, token).Collect(ctx, lookbackWindow)
	if err != nil {
		log.Printf("ERROR scheduler collect cluster=%s: %v", clusterID, err)
		s.store.UpdateClusterHealth(ctx, clusterID, models.ClusterStatusDisconnected, time.Now())
		return
	}

	log.Printf("INFO  scheduler collected %d containers from cluster=%s", len(metrics), clusterID)

	recommendations, err := recommender.GenerateAll(clusterID, metrics)
	if err != nil {
		log.Printf("ERROR scheduler recommend cluster=%s: %v", clusterID, err)
		return
	}

	var saved int
	for _, rec := range recommendations {
		if err = s.store.UpsertRecommendation(ctx, rec); err != nil {
			log.Printf("ERROR scheduler upsert cluster=%s pod=%s container=%s: %v",
				clusterID, rec.PodName, rec.ContainerName, err)
			continue
		}
		saved++
	}

	log.Printf("INFO  scheduler saved %d/%d recommendations for cluster=%s", saved, len(recommendations), clusterID)

	s.store.UpdateClusterHealth(ctx, clusterID, models.ClusterStatusConnected, time.Now())
}
