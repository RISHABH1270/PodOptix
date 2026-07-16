package api

import (
	"log"
	"net/http"
	"time"

	"github.com/RISHABH1270/PodOptix/internal/auth"
	"github.com/RISHABH1270/PodOptix/pkg/models"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// CreateClusterRequest defines the expected JSON body for registering a cluster.
type CreateClusterRequest struct {
	Name           string `json:"name"            binding:"required"`
	PrometheusURL  string `json:"prometheus_url"  binding:"required"`
	PrometheusToken string `json:"prometheus_token" binding:"required"`
	LookbackWindow string `json:"lookback_window"`
}

// UpdateClusterRequest defines the expected JSON body for updating a cluster.
// All fields optional — only provided fields are updated.
type UpdateClusterRequest struct {
	Name          string `json:"name"`
	PrometheusURL string `json:"prometheus_url"`
	PrometheusToken string `json:"prometheus_token"`
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

	var cluster *models.Cluster
	cluster = &models.Cluster{
		ClusterID:      uuid.New().String(),
		Name:           req.Name,
		PrometheusURL:  req.PrometheusURL,
		PrometheusToken: encryptedToken,
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

// updateCluster updates an existing cluster's details.
func (s *Server) updateCluster(c *gin.Context) {
	var requestID string
	requestID = c.GetString("request_id")

	var id string
	id = c.Param("id")

	// fetch existing cluster
	cluster, err := s.store.GetCluster(c.Request.Context(), id)
	if err != nil {
		log.Printf("ERROR [%s] updateCluster not found id=%s: %v", requestID, id, err)
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

	// apply only the fields that were provided
	if req.Name != "" {
		cluster.Name = req.Name
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

	if err := s.store.UpdateCluster(c.Request.Context(), cluster); err != nil {
		log.Printf("ERROR [%s] updateCluster save id=%s: %v", requestID, id, err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":      "Failed to update cluster, please try again",
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
