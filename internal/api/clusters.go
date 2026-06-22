package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// listClusters returns all registered clusters.
func (s *Server) listClusters(c *gin.Context) {
	// TODO: fetch from database
	c.JSON(http.StatusOK, gin.H{
		"message": "list clusters — coming soon",
	})
}

// createCluster registers a new cluster.
func (s *Server) createCluster(c *gin.Context) {
	// TODO: validate input, save to database
	c.JSON(http.StatusCreated, gin.H{
		"message": "create cluster — coming soon",
	})
}

// getCluster returns a single cluster by ID.
func (s *Server) getCluster(c *gin.Context) {
	var id string
	id = c.Param("id")
	// TODO: fetch from database
	c.JSON(http.StatusOK, gin.H{
		"message": "get cluster — coming soon",
		"id":      id,
	})
}

// deleteCluster removes a cluster by ID.
func (s *Server) deleteCluster(c *gin.Context) {
	var id string
	id = c.Param("id")
	// TODO: delete from database
	c.JSON(http.StatusOK, gin.H{
		"message": "delete cluster — coming soon",
		"id":      id,
	})
}

