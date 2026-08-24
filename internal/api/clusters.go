package api

import (
	"context"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/RISHABH1270/PodOptix/internal/auth"
	"github.com/RISHABH1270/PodOptix/internal/collector"
	"github.com/RISHABH1270/PodOptix/internal/recommender"
	"github.com/RISHABH1270/PodOptix/pkg/models"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// ClusterResponse is the API response shape for a cluster.
// LastSyncedAt is always a string — "not yet synced" if no sync has happened, RFC3339 timestamp otherwise.
type ClusterResponse struct {
	ClusterID      string `json:"cluster_id"`
	ClusterName    string `json:"cluster_name"`
	PrometheusURL  string `json:"prometheus_url"`
	LookbackWindow string `json:"lookback_window"`
	Status         string `json:"status"`
	CreatedBy      string `json:"created_by"`
	LastSyncedAt   string `json:"last_synced_at"`
	CreatedAt      string `json:"created_at"`
	UpdatedAt      string `json:"updated_at"`
}

func toClusterResponse(c *models.Cluster) ClusterResponse {
	lastSynced := "not yet synced"
	if c.LastSyncedAt != nil {
		lastSynced = c.LastSyncedAt.UTC().Format(time.RFC3339)
	}
	return ClusterResponse{
		ClusterID:      c.ClusterID,
		ClusterName:    c.ClusterName,
		PrometheusURL:  c.PrometheusURL,
		LookbackWindow: c.LookbackWindow,
		Status:         c.Status,
		CreatedBy:      c.CreatedBy,
		LastSyncedAt:   lastSynced,
		CreatedAt:      c.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:      c.UpdatedAt.UTC().Format(time.RFC3339),
	}
}

// CreateClusterRequest defines the expected JSON body for registering a cluster.
type CreateClusterRequest struct {
	ClusterName     string `json:"cluster_name"     binding:"required,max=100"`
	PrometheusURL   string `json:"prometheus_url"   binding:"required"`
	PrometheusToken string `json:"prometheus_token" binding:"required"`
	LookbackWindow  string `json:"lookback_window"`
}

// UpdateClusterRequest defines the expected JSON body for updating a cluster.
// All fields optional — only provided fields are updated.
type UpdateClusterRequest struct {
	ClusterName     string `json:"cluster_name"     binding:"max=100"`
	PrometheusURL   string `json:"prometheus_url"`
	PrometheusToken string `json:"prometheus_token"`
	LookbackWindow  string `json:"lookback_window"`
}

// createCluster registers a new cluster.
func (s *Server) createCluster(c *gin.Context) {
	requestID := c.GetString("request_id")

	var req CreateClusterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("ERROR [%s] createCluster invalid request: %v", requestID, err)
		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid request — name, prometheus_url and token are required",
			"request_id": requestID,
		})
		return
	}

	if req.LookbackWindow == "" {
		req.LookbackWindow = models.DefaultLookbackWindow
	}
	switch req.LookbackWindow {
	case models.LookbackWindow7d, models.LookbackWindow10d, models.LookbackWindow30d: // valid
	default:
		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid lookback_window — allowed values: 7d, 10d, 30d",
			"request_id": requestID,
		})
		return
	}

	// check Prometheus connectivity immediately — status is connected or disconnected, never pending
	pingCtx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	status := models.ClusterStatusConnected
	if err := collector.New(req.PrometheusURL, req.PrometheusToken).Ping(pingCtx); err != nil {
		log.Printf("INFO [%s] createCluster connectivity check failed for %s: %v", requestID, req.PrometheusURL, err)
		status = models.ClusterStatusDisconnected
	}

	// encrypt token before storing — never save plain text to database
	encryptedToken, err := auth.Encrypt(req.PrometheusToken, s.encryptionKey)
	if err != nil {
		log.Printf("ERROR [%s] createCluster encrypt token: %v", requestID, err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":      "Failed to register cluster, please try again",
			"request_id": requestID,
		})
		return
	}

	cluster := &models.Cluster{
		ClusterID:       uuid.New().String(),
		ClusterName:     req.ClusterName,
		PrometheusURL:   req.PrometheusURL,
		PrometheusToken: encryptedToken,
		LookbackWindow:  req.LookbackWindow,
		Status:          status,
		CreatedBy:       c.GetString("user_id"),
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}

	if err := s.store.SaveCluster(c.Request.Context(), cluster); err != nil {
		log.Printf("ERROR [%s] createCluster save: %v", requestID, err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":      "Failed to register cluster, please try again",
			"request_id": requestID,
		})
		return
	}

	// kick off initial sync in background — only if Prometheus is reachable
	// plain token available here without decryption, timeout prevents goroutine running forever
	if status == models.ClusterStatusConnected {
		clusterID := cluster.ClusterID
		plainToken := req.PrometheusToken
		prometheusURL := cluster.PrometheusURL
		lookbackWindow := cluster.LookbackWindow

		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
			defer cancel()
			log.Printf("INFO  initial sync started cluster=%s", clusterID)

			metrics, err := collector.New(prometheusURL, plainToken).Collect(ctx, lookbackWindow)
			if err != nil {
				log.Printf("ERROR initial sync collect cluster=%s: %v", clusterID, err)
				s.store.UpdateClusterHealth(ctx, clusterID, models.ClusterStatusDisconnected, time.Now())
				return
			}

			recs, err := recommender.GenerateAll(clusterID, lookbackWindow, metrics)
			if err != nil {
				log.Printf("ERROR initial sync recommend cluster=%s: %v", clusterID, err)
				return
			}

			for _, rec := range recs {
				if err = s.store.UpsertRecommendation(ctx, rec); err != nil {
					log.Printf("ERROR initial sync upsert cluster=%s: %v", clusterID, err)
				}
			}

			s.store.UpdateClusterHealth(ctx, clusterID, models.ClusterStatusConnected, time.Now())
			log.Printf("INFO  initial sync completed cluster=%s saved=%d", clusterID, len(recs))
		}()
	}

	c.JSON(http.StatusCreated, toClusterResponse(cluster))
}

// listClusters returns all registered clusters.
func (s *Server) listClusters(c *gin.Context) {
	requestID := c.GetString("request_id")

	clusters, err := s.store.ListClusters(c.Request.Context())
	if err != nil {
		log.Printf("ERROR [%s] listClusters: %v", requestID, err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":      "Failed to fetch clusters",
			"request_id": requestID,
		})
		return
	}
	if clusters == nil {
		clusters = []*models.Cluster{}
	}
	resp := make([]ClusterResponse, len(clusters))
	for i, cl := range clusters {
		resp[i] = toClusterResponse(cl)
	}
	c.JSON(http.StatusOK, resp)
}

// getCluster returns a single cluster by ID.
func (s *Server) getCluster(c *gin.Context) {
	requestID := c.GetString("request_id")
	clusterID := c.Param("id")

	cluster, err := s.store.GetCluster(c.Request.Context(), clusterID)
	if err != nil {
		log.Printf("ERROR [%s] getCluster id=%s: %v", requestID, clusterID, err)
		c.JSON(http.StatusNotFound, gin.H{
			"error":      "Cluster not found",
			"request_id": requestID,
		})
		return
	}
	c.JSON(http.StatusOK, toClusterResponse(cluster))
}

// updateCluster updates an existing cluster's details.
func (s *Server) updateCluster(c *gin.Context) {
	requestID := c.GetString("request_id")
	clusterID := c.Param("id")

	// fetch existing cluster
	cluster, err := s.store.GetCluster(c.Request.Context(), clusterID)
	if err != nil {
		log.Printf("ERROR [%s] updateCluster not found id=%s: %v", requestID, clusterID, err)
		c.JSON(http.StatusNotFound, gin.H{
			"error":      "Cluster not found",
			"request_id": requestID,
		})
		return
	}

	var req UpdateClusterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("ERROR [%s] updateCluster invalid request: %v", requestID, err)
		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid request body",
			"request_id": requestID,
		})
		return
	}

	// apply only the fields that were provided — order matches models.Cluster
	if req.ClusterName != "" {
		cluster.ClusterName = req.ClusterName
	}
	if req.PrometheusURL != "" {
		cluster.PrometheusURL = req.PrometheusURL
	}
	if req.PrometheusToken != "" {
		encryptedToken, err := auth.Encrypt(req.PrometheusToken, s.encryptionKey)
		if err != nil {
			log.Printf("ERROR [%s] updateCluster encrypt token: %v", requestID, err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":      "Failed to update cluster, please try again",
				"request_id": requestID,
			})
			return
		}
		cluster.PrometheusToken = encryptedToken
	}
	if req.LookbackWindow != "" {
		switch req.LookbackWindow {
		case models.LookbackWindow7d, models.LookbackWindow10d, models.LookbackWindow30d:
			cluster.LookbackWindow = req.LookbackWindow
		default:
			c.JSON(http.StatusBadRequest, gin.H{
				"error":      "Invalid lookback_window — allowed values: 7d, 10d, 30d",
				"request_id": requestID,
			})
			return
		}
	}

	if err := s.store.UpdateCluster(c.Request.Context(), cluster); err != nil {
		log.Printf("ERROR [%s] updateCluster save id=%s: %v", requestID, clusterID, err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":      "Failed to update cluster, please try again",
			"request_id": requestID,
		})
		return
	}

	// if endpoint changed, re-ping and update status via UpdateClusterHealth
	if req.PrometheusURL != "" || req.PrometheusToken != "" {
		var pingToken string
		if req.PrometheusToken != "" {
			pingToken = req.PrometheusToken
		} else {
			decrypted, err := auth.Decrypt(cluster.PrometheusToken, s.encryptionKey)
			if err != nil {
				log.Printf("WARN [%s] updateCluster decrypt for ping failed: %v", requestID, err)
			} else {
				pingToken = decrypted
			}
		}

		pingCtx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
		defer cancel()

		newStatus := models.ClusterStatusConnected
		if err := collector.New(cluster.PrometheusURL, pingToken).Ping(pingCtx); err != nil {
			log.Printf("INFO [%s] updateCluster connectivity check failed: %v", requestID, err)
			newStatus = models.ClusterStatusDisconnected
		}
		s.store.UpdateClusterHealth(c.Request.Context(), clusterID, newStatus, time.Now())
		cluster.Status = newStatus
	}

	c.JSON(http.StatusOK, toClusterResponse(cluster))
}

// deleteCluster removes a cluster by ID.
func (s *Server) deleteCluster(c *gin.Context) {
	requestID := c.GetString("request_id")
	clusterID := c.Param("id")

	if err := s.store.DeleteCluster(c.Request.Context(), clusterID); err != nil {
		log.Printf("ERROR [%s] deleteCluster id=%s: %v", requestID, clusterID, err)
		if strings.Contains(err.Error(), "not found") {
			c.JSON(http.StatusNotFound, gin.H{
				"error":      "Cluster not found",
				"request_id": requestID,
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":      "Failed to delete cluster, please try again",
			"request_id": requestID,
		})
		return
	}

	c.Status(http.StatusNoContent)
}
