package api

import (
	"context"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/RISHABH1270/PodOptix/internal/auth"
	"github.com/RISHABH1270/PodOptix/internal/collector"
	"github.com/RISHABH1270/PodOptix/internal/metrics"
	"github.com/RISHABH1270/PodOptix/internal/recommender"
	"github.com/RISHABH1270/PodOptix/pkg/models"
	"github.com/gin-gonic/gin"
)

// listAllRecommendations returns every recommendation across every cluster,
// joined with the cluster name. Used by the cross-cluster overview page.
// Ordered by biggest CPU delta first — surfaces the biggest wins on top.
func (s *Server) listAllRecommendations(c *gin.Context) {
	requestID := c.GetString("request_id")

	recs, err := s.store.ListAllWithClusterName(c.Request.Context())
	if err != nil {
		log.Printf("ERROR [%s] listAllRecommendations db: %v", requestID, err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":      "Failed to fetch recommendations",
			"request_id": requestID,
		})
		return
	}
	c.JSON(http.StatusOK, recs)
}

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
			metrics.CacheHitsTotal.WithLabelValues("recommendations").Inc()
			c.JSON(http.StatusOK, cached)
			return
		}
		metrics.CacheMissesTotal.WithLabelValues("recommendations").Inc()
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

// deleteRecommendation hard-deletes a single recommendation row.
// Used by the dashboard per-row trash button on an orphaned workload.
// Invalidates the cluster's cached recommendation list.
func (s *Server) deleteRecommendation(c *gin.Context) {
	requestID := c.GetString("request_id")
	clusterID := c.Param("id")
	recID := c.Param("recId")

	if err := s.store.DeleteRecommendation(c.Request.Context(), recID); err != nil {
		// The store returns a "not found" error with that literal substring.
		if containsNotFound(err) {
			c.JSON(http.StatusNotFound, gin.H{
				"error":      "Recommendation not found",
				"request_id": requestID,
			})
			return
		}
		log.Printf("ERROR [%s] deleteRecommendation cluster=%s rec=%s: %v", requestID, clusterID, recID, err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":      "Failed to delete recommendation",
			"request_id": requestID,
		})
		return
	}

	if s.cache != nil {
		s.cache.InvalidateRecommendations(c.Request.Context(), clusterID)
	}
	c.JSON(http.StatusOK, gin.H{"deleted": 1, "recommendation_id": recID})
}

// deleteOrphanedRecommendations bulk-deletes every orphaned row for a cluster.
// Gated by ?orphaned=true so the endpoint can't accidentally wipe all rows —
// a non-orphaned bulk delete would be a huge footgun, so we force the query
// parameter to make the intent explicit.
func (s *Server) deleteOrphanedRecommendations(c *gin.Context) {
	requestID := c.GetString("request_id")
	clusterID := c.Param("id")

	if c.Query("orphaned") != "true" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "bulk delete requires ?orphaned=true",
			"request_id": requestID,
		})
		return
	}

	n, err := s.store.DeleteOrphaned(c.Request.Context(), clusterID)
	if err != nil {
		log.Printf("ERROR [%s] deleteOrphanedRecommendations cluster=%s: %v", requestID, clusterID, err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":      "Failed to delete orphaned recommendations",
			"request_id": requestID,
		})
		return
	}

	if s.cache != nil {
		s.cache.InvalidateRecommendations(c.Request.Context(), clusterID)
	}
	c.JSON(http.StatusOK, gin.H{"deleted": n})
}

// containsNotFound is a tiny helper so handlers can distinguish the store's
// "not found" sentinel without pulling in errors.Is boilerplate for every call site.
func containsNotFound(err error) bool {
	return err != nil && strings.Contains(err.Error(), "not found")
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
			if err := s.store.UpdateClusterHealth(ctx, clusterID, models.ClusterStatusDisconnected, time.Now()); err != nil {
				log.Printf("WARN  recalculate health update cluster=%s: %v", clusterID, err)
			}
			return
		}

		recs, err := recommender.GenerateAll(clusterID, metrics)
		if err != nil {
			log.Printf("ERROR recalculate recommend cluster=%s: %v", clusterID, err)
			return
		}

		seenKeys := make([]models.WorkloadKey, 0, len(recs))
		for _, rec := range recs {
			if err = s.store.UpsertRecommendation(ctx, rec); err != nil {
				log.Printf("ERROR recalculate upsert cluster=%s: %v", clusterID, err)
				continue
			}
			seenKeys = append(seenKeys, models.WorkloadKey{
				Namespace:     rec.Namespace,
				WorkloadKind:  rec.WorkloadKind,
				WorkloadName:  rec.WorkloadName,
				ContainerName: rec.ContainerName,
			})
		}

		if orphaned, err := s.store.MarkOrphaned(ctx, clusterID, seenKeys); err != nil {
			log.Printf("WARN  recalculate mark orphaned cluster=%s: %v", clusterID, err)
		} else if orphaned > 0 {
			log.Printf("INFO  recalculate marked %d workloads as orphaned cluster=%s", orphaned, clusterID)
		}

		if err := s.store.UpdateClusterHealth(ctx, clusterID, models.ClusterStatusConnected, time.Now()); err != nil {
			log.Printf("WARN  recalculate health update cluster=%s: %v", clusterID, err)
		}

		if s.cache != nil {
			s.cache.InvalidateRecommendations(ctx, clusterID)
		}

		log.Printf("INFO  recalculate completed cluster=%s saved=%d", clusterID, len(recs))
	}()

	log.Printf("INFO  recalculate accepted cluster=%s req=%s", clusterID, requestID)
	c.JSON(http.StatusAccepted, gin.H{
		"message":    "Recalculation started. Check recommendations in a few minutes.",
		"cluster_id": clusterID,
		"request_id": requestID,
	})
}
