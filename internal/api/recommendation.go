package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// listRecommendations returns all recommendations for a cluster.
func (s *Server) listRecommendations(c *gin.Context) {
	var id string
	id = c.Param("id")
	// TODO: fetch from database
	c.JSON(http.StatusOK, gin.H{
		"message":    "list recommendations — coming soon",
		"cluster_id": id,
	})
}
