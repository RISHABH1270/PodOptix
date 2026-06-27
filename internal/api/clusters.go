package api

import (
	"log"
	"net/http"
	"time"

	"github.com/RISHABH1270/PodOptix/pkg/models"
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
	var requestID string
	requestID = c.GetString("request_id")

	clusters, err := s.store.ListClusters(c.Request.Context())
	if err != nil {
		log.Printf("ERROR [%s] listClusters: %v", requestID, err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":      "No cluster records found",
			"request_id": requestID,
		})
		return
	}
	if clusters == nil {
		clusters = []*models.Cluster{}
	}
	c.JSON(http.StatusOK, clusters)
}

// createCluster registers a new cluster.
func (s *Server) createCluster(c *gin.Context) {
	var requestID string
	requestID = c.GetString("request_id")

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
		req.LookbackWindow = "7d"
	}

	var cluster *models.Cluster
	cluster = &models.Cluster{
		ClusterID:      uuid.New().String(),
		Name:           req.Name,
		PrometheusURL:  req.PrometheusURL,
		Token:          req.Token,
		LookbackWindow: req.LookbackWindow,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	if err := s.store.SaveCluster(c.Request.Context(), cluster); err != nil {
		log.Printf("ERROR [%s] createCluster save: %v", requestID, err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":      "Failed to register cluster, please try again",
			"request_id": requestID,
		})
		return
	}

	c.JSON(http.StatusCreated, cluster)
}

// getCluster returns a single cluster by ID.
func (s *Server) getCluster(c *gin.Context) {
	var requestID string
	requestID = c.GetString("request_id")

	var id string
	id = c.Param("id")

	cluster, err := s.store.GetCluster(c.Request.Context(), id)
	if err != nil {
		log.Printf("ERROR [%s] getCluster id=%s: %v", requestID, id, err)
		c.JSON(http.StatusNotFound, gin.H{
			"error":      "Cluster not found",
			"request_id": requestID,
		})
		return
	}
	c.JSON(http.StatusOK, cluster)
}

// deleteCluster removes a cluster by ID.
func (s *Server) deleteCluster(c *gin.Context) {
	var requestID string
	requestID = c.GetString("request_id")

	var id string
	id = c.Param("id")

	if err := s.store.DeleteCluster(c.Request.Context(), id); err != nil {
		log.Printf("ERROR [%s] deleteCluster id=%s: %v", requestID, id, err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":      "Failed to delete cluster, please try again",
			"request_id": requestID,
		})
		return
	}

	c.Status(http.StatusNoContent)
}
