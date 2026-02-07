package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	apperrors "kuberan/internal/errors"
	"kuberan/internal/middleware"
	"kuberan/internal/services"
)

// AuthHandler handles authentication-related requests
type AuthHandler struct {
	userService *services.UserService
}

// NewAuthHandler creates a new AuthHandler
func NewAuthHandler(userService *services.UserService) *AuthHandler {
	return &AuthHandler{userService: userService}
}

// RegisterRequest represents the registration request payload
type RegisterRequest struct {
	Email     string `json:"email" binding:"required,email,max=255"`
	Password  string `json:"password" binding:"required,min=8,max=128"`
	FirstName string `json:"first_name" binding:"max=100"`
	LastName  string `json:"last_name" binding:"max=100"`
}

// LoginRequest represents the login request payload
type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

// UserResponse represents the user data in the response
type UserResponse struct {
	ID        uint   `json:"id"`
	Email     string `json:"email"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
}

// AuthResponse represents the authentication response with token
type AuthResponse struct {
	Token string       `json:"token"`
	User  UserResponse `json:"user"`
}

// Register handles user registration
// @Summary     Register a new user
// @Description Register a new user with email and password
// @Tags        auth
// @Accept      json
// @Produce     json
// @Param       request body RegisterRequest true "User registration data"
// @Success     201 {object} AuthResponse "User registered and token generated"
// @Failure     400 {object} ErrorResponse "Invalid input"
// @Failure     500 {object} ErrorResponse "Server error"
// @Router      /auth/register [post]
func (h *AuthHandler) Register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondWithError(c, apperrors.WithMessage(apperrors.ErrInvalidInput, err.Error()))
		return
	}

	user, err := h.userService.CreateUser(req.Email, req.Password, req.FirstName, req.LastName)
	if err != nil {
		respondWithError(c, err)
		return
	}

	token, err := middleware.GenerateToken(user)
	if err != nil {
		respondWithError(c, apperrors.Wrap(apperrors.ErrInternalServer, err))
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"token": token,
		"user": gin.H{
			"id":         user.ID,
			"email":      user.Email,
			"first_name": user.FirstName,
			"last_name":  user.LastName,
		},
	})
}

// Login handles user login
// @Summary     Login user
// @Description Authenticate a user and get a token
// @Tags        auth
// @Accept      json
// @Produce     json
// @Param       request body LoginRequest true "User login credentials"
// @Success     200 {object} AuthResponse "User authenticated and token generated"
// @Failure     400 {object} ErrorResponse "Invalid input"
// @Failure     401 {object} ErrorResponse "Invalid credentials"
// @Failure     500 {object} ErrorResponse "Server error"
// @Router      /auth/login [post]
func (h *AuthHandler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondWithError(c, apperrors.WithMessage(apperrors.ErrInvalidInput, err.Error()))
		return
	}

	user, err := h.userService.GetUserByEmail(req.Email)
	if err != nil {
		respondWithError(c, apperrors.ErrInvalidCredentials)
		return
	}

	if !h.userService.VerifyPassword(user, req.Password) {
		respondWithError(c, apperrors.ErrInvalidCredentials)
		return
	}

	token, err := middleware.GenerateToken(user)
	if err != nil {
		respondWithError(c, apperrors.Wrap(apperrors.ErrInternalServer, err))
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"token": token,
		"user": gin.H{
			"id":         user.ID,
			"email":      user.Email,
			"first_name": user.FirstName,
			"last_name":  user.LastName,
		},
	})
}

// GetProfile returns the user's profile
// @Summary     Get user profile
// @Description Get the authenticated user's profile information
// @Tags        user
// @Accept      json
// @Produce     json
// @Security    BearerAuth
// @Success     200 {object} UserResponse "User profile"
// @Failure     401 {object} ErrorResponse "Unauthorized"
// @Failure     500 {object} ErrorResponse "Server error"
// @Router      /profile [get]
func (h *AuthHandler) GetProfile(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		respondWithError(c, apperrors.ErrUnauthorized)
		return
	}

	user, err := h.userService.GetUserByID(userID.(uint))
	if err != nil {
		respondWithError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"user": gin.H{
			"id":         user.ID,
			"email":      user.Email,
			"first_name": user.FirstName,
			"last_name":  user.LastName,
		},
	})
}

// ErrorDetail represents the inner error object in an error response.
type ErrorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// ErrorResponse represents an error response.
type ErrorResponse struct {
	Error ErrorDetail `json:"error"`
}
