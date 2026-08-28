package api

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/RISHABH1270/PodOptix/internal/auth"
	"github.com/RISHABH1270/PodOptix/internal/collector"
	"github.com/RISHABH1270/PodOptix/internal/recommender"
	"github.com/RISHABH1270/PodOptix/pkg/models"
	"github.com/gin-gonic/gin"
)

// listRecommendations returns all recommendations for a cluster.
// Checks Redis cache first — falls back to PostgreSQL on miss.
func (s *Server) listRecommendations(c *gin.Context) {
	requestID := c.GetString("request_id")
	clusterID := c.Param("id")

	// try Redis cache first
	if s.cache != nil {
		var cached []*models.Recommendation
		hit, err := s.cache.GetRecommendations(c.Request.Context(), clusterID, &cached)
		if err != nil {
			log.Printf("WARN  [%s] listRecommendations cache get: %v", requestID, err)
		}
		if hit {
			c.JSON(http.StatusOK, cached)
			return
		}
	}

	// cache miss — fetch from PostgreSQL
	recommendations, err := s.store.ListByCluster(c.Request.Context(), clusterID)
	if err != nil {
		log.Printf("ERROR [%s] listRecommendations db: %v", requestID, err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":      "Failed to fetch recommendations",
			"request_id": requestID,
		})
		return
	}

	// cache for next request
	if s.cache != nil {
		if err = s.cache.SetRecommendations(c.Request.Context(), clusterID, recommendations); err != nil {
			log.Printf("WARN  [%s] listRecommendations cache set: %v", requestID, err)
		}
	}

	c.JSON(http.StatusOK, recommendations)
}

// recalculate triggers a manual recommendation recalculation for a cluster.
// Uses a distributed lock to prevent duplicate jobs.
// Returns 202 Accepted immediately — runs in background with 10 min timeout.
func (s *Server) recalculate(c *gin.Context) {
	requestID := c.GetString("request_id")
	clusterID := c.Param("id")

	// verify cluster exists
	cluster, err := s.store.GetCluster(c.Request.Context(), clusterID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error":      "Cluster not found",
			"request_id": requestID,
		})
		return
	}

	// try to acquire distributed lock — prevents duplicate jobs
	if s.cache != nil {
		locked, err := s.cache.AcquireRecalculateLock(c.Request.Context(), clusterID)
		if err != nil {
			log.Printf("WARN  [%s] recalculate lock error cluster=%s: %v", requestID, clusterID, err)
		}
		if !locked {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error":      "Recalculation already in progress for this cluster. Try again in 10 minutes.",
				"request_id": requestID,
			})
			return
		}
	}

	// decrypt token before using for Prometheus
	plainToken, err := auth.Decrypt(cluster.PrometheusToken, s.encryptionKey)
	if err != nil {
		log.Printf("ERROR [%s] recalculate decrypt token cluster=%s: %v", requestID, clusterID, err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":      "Failed to start recalculation",
			"request_id": requestID,
		})
		return
	}

	// run in background — return 202 immediately
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()
		defer func() {
			if s.cache != nil {
				s.cache.ReleaseRecalculateLock(ctx, clusterID)
			}
		}()

		log.Printf("INFO  recalculate started cluster=%s", clusterID)

		metrics, err := collector.New(cluster.PrometheusURL, plainToken).Collect(ctx, cluster.LookbackWindow)
		if err != nil {
			log.Printf("ERROR recalculate collect cluster=%s: %v", clusterID, err)
			s.store.UpdateClusterHealth(ctx, clusterID, models.ClusterStatusDisconnected, time.Now())
			return
		}

		recs, err := recommender.GenerateAll(clusterID, metrics)
		if err != nil {
			log.Printf("ERROR recalculate recommend cluster=%s: %v", clusterID, err)
			return
		}

		for _, rec := range recs {
			if err = s.store.UpsertRecommendation(ctx, rec); err != nil {
				log.Printf("ERROR recalculate upsert cluster=%s: %v", clusterID, err)
			}
		}

		s.store.UpdateClusterHealth(ctx, clusterID, models.ClusterStatusConnected, time.Now())

		if s.cache != nil {
			s.cache.InvalidateRecommendations(ctx, clusterID)
		}

		log.Printf("INFO  recalculate completed cluster=%s saved=%d", clusterID, len(recs))
	}()

	c.JSON(http.StatusAccepted, gin.H{
		"message":    "Recalculation started. Check recommendations in a few minutes.",
		"cluster_id": clusterID,
		"request_id": requestID,
	})
}
