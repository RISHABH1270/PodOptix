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

// RegisterRequest defines the expected JSON body for registration.
type RegisterRequest struct {
	Email    string `json:"email"    binding:"required"`
	Password string `json:"password" binding:"required"`
}

// LoginRequest defines the expected JSON body for login.
type LoginRequest struct {
	Email    string `json:"email"    binding:"required"`
	Password string `json:"password" binding:"required"`
}

// register creates a new user account.
func (s *Server) register(c *gin.Context) {
	requestID := c.GetString("request_id")

	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Email and password are required",
			"request_id": requestID,
		})
		return
	}

	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		log.Printf("ERROR [%s] register hash password: %v", requestID, err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":      "Failed to create account, please try again",
			"request_id": requestID,
		})
		return
	}

	user := &models.User{
		UserID:       uuid.New().String(),
		Email:        req.Email,
		PasswordHash: hash,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	if err = s.store.CreateUser(c.Request.Context(), user); err != nil {
		c.JSON(http.StatusConflict, gin.H{
			"error":      "An account with this email already exists",
			"request_id": requestID,
		})
		return
	}

	token, err := auth.GenerateToken(user.UserID, user.Email, s.jwtSecret)
	if err != nil {
		log.Printf("ERROR [%s] register generate token: %v", requestID, err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":      "Account created. Please log in.",
			"request_id": requestID,
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"token":   token,
		"user_id": user.UserID,
		"email":   user.Email,
	})
}

// login authenticates a user and returns a JWT token.
func (s *Server) login(c *gin.Context) {
	requestID := c.GetString("request_id")

	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Email and password are required",
			"request_id": requestID,
		})
		return
	}

	user, err := s.store.GetUserByEmail(c.Request.Context(), req.Email)
	if err != nil {
		// same error for wrong email OR wrong password — prevents user enumeration
		c.JSON(http.StatusUnauthorized, gin.H{
			"error":      "Invalid email or password",
			"request_id": requestID,
		})
		return
	}

	if err = auth.CheckPassword(req.Password, user.PasswordHash); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error":      "Invalid email or password",
			"request_id": requestID,
		})
		return
	}

	token, err := auth.GenerateToken(user.UserID, user.Email, s.jwtSecret)
	if err != nil {
		log.Printf("ERROR [%s] login generate token: %v", requestID, err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":      "Login failed, please try again",
			"request_id": requestID,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"token":   token,
		"user_id": user.UserID,
		"email":   user.Email,
	})
}
