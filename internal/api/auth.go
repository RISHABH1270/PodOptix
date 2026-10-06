package api

import (
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/RISHABH1270/PodOptix/internal/auth"
	"github.com/RISHABH1270/PodOptix/internal/store"
	"github.com/RISHABH1270/PodOptix/pkg/models"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// RegisterRequest defines the expected JSON body for registration.
// Password: min 8 (so bcrypt isn't trivially brute-forced); max 128 so a huge
// payload can't DoS the hashing path (bcrypt truncates at 72 bytes but the
// allocation + encode still runs on whatever we accept).
type RegisterRequest struct {
	Email    string `json:"email"    binding:"required,email"`
	Password string `json:"password" binding:"required,min=8,max=128"`
}

// LoginRequest defines the expected JSON body for login.
// Same length caps on password — stops the same cheap DoS on CheckPassword.
// Email format validation here keeps 400s crisp before we even hit the store.
type LoginRequest struct {
	Email    string `json:"email"    binding:"required,email"`
	Password string `json:"password" binding:"required,min=8,max=128"`
}

// register creates a new user account.
func (s *Server) register(c *gin.Context) {
	requestID := c.GetString("request_id")

	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid request — email must be a valid email address, password must be 8–128 characters.",
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
		if errors.Is(err, store.ErrEmailAlreadyRegistered) {
			c.JSON(http.StatusConflict, gin.H{
				"error":      "An account with this email already exists",
				"request_id": requestID,
			})
			return
		}
		log.Printf("ERROR [%s] register create user: %v", requestID, err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":      "Failed to create account, please try again",
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

	log.Printf("INFO  user registered email=%s user_id=%s req=%s", user.Email, user.UserID, requestID)
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
			"error":      "Invalid request — email must be a valid email address, password must be 8–128 characters.",
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

	log.Printf("INFO  user login email=%s req=%s", user.Email, requestID)
	c.JSON(http.StatusOK, gin.H{
		"token":   token,
		"user_id": user.UserID,
		"email":   user.Email,
	})
}
