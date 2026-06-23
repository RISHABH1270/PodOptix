package api

import (
	"net/http"
	"time"

	"github.com/RISHABH1270/podoptix/pkg/models"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// CreateClusterRequest defines the expected JSON body for registering a cluster.
type CreateClusterRequest struct {
	Name           string `json:"name"            binding:"required"`
	PrometheusURL  string `json:"prometheus_url"  binding:"required"`
	Token          string `json:"token"           binding:"required"`
	LookbackWindow string `json:"lookback_window"`
}

// listClusters returns all registered clusters.
func (s *Server) listClusters(c *gin.Context) {
	clusters, err := s.store.ListClusters(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch clusters"})
		return
	}
	c.JSON(http.StatusOK, clusters)
}

// createCluster registers a new cluster.
func (s *Server) createCluster(c *gin.Context) {
	// step 1 — read and validate request body
	var req CreateClusterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// step 2 — default lookback_window to "7d" if not provided
	if req.LookbackWindow == "" {
		req.LookbackWindow = "7d"
	}

	// step 3 — build cluster object
	var cluster *models.Cluster
	cluster = &models.Cluster{
		ID:             uuid.New().String(),
		Name:           req.Name,
		PrometheusURL:  req.PrometheusURL,
		Token:          req.Token,
		LookbackWindow: req.LookbackWindow,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	// step 4 — save to database
	if err := s.store.SaveCluster(c.Request.Context(), cluster); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save cluster"})
		return
	}

	// step 5 — return the created cluster
	c.JSON(http.StatusCreated, cluster)
}

// getCluster returns a single cluster by ID.
func (s *Server) getCluster(c *gin.Context) {
	var id string
	id = c.Param("id")

	cluster, err := s.store.GetCluster(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "cluster not found"})
		return
	}
	c.JSON(http.StatusOK, cluster)
}

// deleteCluster removes a cluster by ID.
func (s *Server) deleteCluster(c *gin.Context) {
	var id string
	id = c.Param("id")

	if err := s.store.DeleteCluster(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete cluster"})
		return
	}

	// 204 No Content — success, nothing to return
	c.Status(http.StatusNoContent)
}
