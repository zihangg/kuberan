package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	apperrors "kuberan/internal/errors"
	"kuberan/internal/middleware"
	"kuberan/internal/models"
	"kuberan/internal/services"
)

// AuthHandler handles authentication-related requests.
type AuthHandler struct {
	userService  services.UserServicer
	auditService services.AuditServicer
}

// NewAuthHandler creates a new AuthHandler.
func NewAuthHandler(userService services.UserServicer, auditService services.AuditServicer) *AuthHandler {
	return &AuthHandler{userService: userService, auditService: auditService}
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

// RefreshRequest represents the token refresh request payload.
type RefreshRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

// UserResponse represents the user data in the response
type UserResponse struct {
	ID        uint   `json:"id"`
	Email     string `json:"email"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
}

// AuthResponse represents the authentication response with tokens.
type AuthResponse struct {
	AccessToken  string       `json:"access_token"`
	RefreshToken string       `json:"refresh_token"`
	User         UserResponse `json:"user"`
}

// Register handles user registration
// @Summary     Register a new user
// @Description Register a new user with email and password
// @Tags        auth
// @Accept      json
// @Produce     json
// @Param       request body RegisterRequest true "User registration data"
// @Success     201 {object} AuthResponse "User registered and tokens generated"
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

	accessToken, refreshToken, err := h.startSession(c, user)
	if err != nil {
		respondWithError(c, err)
		return
	}

	h.auditService.Log(user.ID, "REGISTER", "user", user.ID, c.ClientIP(), nil)

	c.JSON(http.StatusCreated, gin.H{
		"access_token":  accessToken,
		"refresh_token": refreshToken,
		"user":          userPayload(user),
	})
}

// Login handles user login
// @Summary     Login user
// @Description Authenticate a user and get access and refresh tokens
// @Tags        auth
// @Accept      json
// @Produce     json
// @Param       request body LoginRequest true "User login credentials"
// @Success     200 {object} AuthResponse "User authenticated and tokens generated"
// @Failure     400 {object} ErrorResponse "Invalid input"
// @Failure     401 {object} ErrorResponse "Invalid credentials"
// @Failure     423 {object} ErrorResponse "Account locked"
// @Failure     500 {object} ErrorResponse "Server error"
// @Router      /auth/login [post]
func (h *AuthHandler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondWithError(c, apperrors.WithMessage(apperrors.ErrInvalidInput, err.Error()))
		return
	}

	user, err := h.userService.AttemptLogin(req.Email, req.Password)
	if err != nil {
		h.auditService.Log("", "LOGIN_FAILED", "user", "", c.ClientIP(),
			map[string]interface{}{"email": req.Email})
		respondWithError(c, err)
		return
	}

	accessToken, refreshToken, err := h.startSession(c, user)
	if err != nil {
		respondWithError(c, err)
		return
	}

	h.auditService.Log(user.ID, "LOGIN", "user", user.ID, c.ClientIP(), nil)

	c.JSON(http.StatusOK, gin.H{
		"access_token":  accessToken,
		"refresh_token": refreshToken,
		"user":          userPayload(user),
	})
}

// RefreshToken exchanges a valid refresh token for a new token pair.
// @Summary     Refresh access token
// @Description Exchange a valid refresh token for new access and refresh tokens
// @Tags        auth
// @Accept      json
// @Produce     json
// @Param       request body RefreshRequest true "Refresh token"
// @Success     200 {object} AuthResponse "New tokens generated"
// @Failure     400 {object} ErrorResponse "Invalid input"
// @Failure     401 {object} ErrorResponse "Invalid or expired refresh token"
// @Failure     500 {object} ErrorResponse "Server error"
// @Router      /auth/refresh [post]
func (h *AuthHandler) RefreshToken(c *gin.Context) {
	var req RefreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondWithError(c, apperrors.WithMessage(apperrors.ErrInvalidInput, err.Error()))
		return
	}

	// Validate the refresh token JWT
	claims, err := middleware.ValidateRefreshToken(req.RefreshToken)
	if err != nil {
		respondWithError(c, apperrors.ErrUnauthorized)
		return
	}

	user, err := h.userService.GetUserByID(claims.UserID)
	if err != nil {
		respondWithError(c, err)
		return
	}
	if !user.IsActive {
		respondWithError(c, apperrors.ErrUnauthorized)
		return
	}

	accessToken, refreshToken, err := issueTokenPair(user)
	if err != nil {
		respondWithError(c, err)
		return
	}

	// Rotate within this device's session only; other devices are unaffected.
	if err := h.userService.RotateRefreshSession(user.ID, middleware.HashToken(req.RefreshToken),
		middleware.HashToken(refreshToken), time.Now().Add(middleware.RefreshTokenExpiry)); err != nil {
		respondWithError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"access_token":  accessToken,
		"refresh_token": refreshToken,
		"user":          userPayload(user),
	})
}

// Logout ends the device session that owns the given refresh token. Other
// devices stay signed in. It always succeeds so a stale token can't block logout.
// @Summary     Logout
// @Description Revoke the refresh session for this device
// @Tags        auth
// @Accept      json
// @Param       request body RefreshRequest true "Refresh token"
// @Success     204 "Logged out"
// @Failure     400 {object} ErrorResponse "Invalid input"
// @Router      /auth/logout [post]
func (h *AuthHandler) Logout(c *gin.Context) {
	var req RefreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondWithError(c, apperrors.WithMessage(apperrors.ErrInvalidInput, err.Error()))
		return
	}

	if claims, err := middleware.ValidateRefreshToken(req.RefreshToken); err == nil {
		if err := h.userService.RevokeRefreshSession(claims.UserID, middleware.HashToken(req.RefreshToken)); err != nil {
			respondWithError(c, err)
			return
		}
		h.auditService.Log(claims.UserID, "LOGOUT", "user", claims.UserID, c.ClientIP(), nil)
	}

	c.Status(http.StatusNoContent)
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
	userID, err := getUserID(c)
	if err != nil {
		respondWithError(c, err)
		return
	}

	user, err := h.userService.GetUserByID(userID)
	if err != nil {
		respondWithError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"user": userPayload(user),
	})
}

// userPayload is the user object embedded in auth and profile responses.
func userPayload(user *models.User) gin.H {
	return gin.H{
		"id":                 user.ID,
		"email":              user.Email,
		"first_name":         user.FirstName,
		"last_name":          user.LastName,
		"hide_balances":      user.HideBalances,
		"default_account_id": user.DefaultAccountID,
	}
}

// UpdateProfileSettingsRequest represents the payload for updating profile settings.
// Every field is optional, but at least one must be present.
type UpdateProfileSettingsRequest struct {
	HideBalances *bool `json:"hide_balances"`
	// DefaultAccountID sets the account preselected for new transactions;
	// null or "" clears it. Must be a UUID otherwise.
	DefaultAccountID optionalString `json:"default_account_id" swaggertype:"string"`
}

// optionalString distinguishes an absent JSON field (Set=false) from an
// explicit null (Set=true, Value=nil).
type optionalString struct {
	Set   bool
	Value *string
}

// UnmarshalJSON records that the field was present and decodes its value.
func (o *optionalString) UnmarshalJSON(data []byte) error {
	o.Set = true
	return json.Unmarshal(data, &o.Value)
}

// UpdateProfileSettings updates the authenticated user's profile settings.
// @Summary     Update profile settings
// @Description Update the authenticated user's profile settings (dashboard balance visibility, default transaction account)
// @Tags        user
// @Accept      json
// @Produce     json
// @Security    BearerAuth
// @Param       request body UpdateProfileSettingsRequest true "Profile settings to update"
// @Success     200 {object} UserResponse "Updated user profile"
// @Failure     400 {object} ErrorResponse "Invalid input"
// @Failure     401 {object} ErrorResponse "Unauthorized"
// @Failure     500 {object} ErrorResponse "Server error"
// @Router      /profile [patch]
func (h *AuthHandler) UpdateProfileSettings(c *gin.Context) {
	userID, err := getUserID(c)
	if err != nil {
		respondWithError(c, err)
		return
	}

	var req UpdateProfileSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondWithError(c, apperrors.WithMessage(apperrors.ErrInvalidInput, err.Error()))
		return
	}

	if req.HideBalances == nil && !req.DefaultAccountID.Set {
		respondWithError(c, apperrors.WithMessage(apperrors.ErrInvalidInput,
			"at least one of hide_balances or default_account_id is required"))
		return
	}

	update := services.ProfileSettingsUpdate{
		HideBalances:      req.HideBalances,
		SetDefaultAccount: req.DefaultAccountID.Set,
	}
	if req.DefaultAccountID.Set {
		update.DefaultAccountID, err = optionalUUID(req.DefaultAccountID.Value, "default_account_id")
		if err != nil {
			respondWithError(c, err)
			return
		}
	}

	if err := h.userService.UpdateProfileSettings(userID, update); err != nil {
		respondWithError(c, err)
		return
	}

	user, err := h.userService.GetUserByID(userID)
	if err != nil {
		respondWithError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"user": userPayload(user),
	})
}

// startSession issues a token pair for a fresh login and records a new
// per-device refresh session for it.
func (h *AuthHandler) startSession(c *gin.Context, user *models.User) (accessToken, refreshToken string, err error) {
	accessToken, refreshToken, err = issueTokenPair(user)
	if err != nil {
		return "", "", err
	}

	if err := h.userService.CreateRefreshSession(user.ID, middleware.HashToken(refreshToken),
		c.Request.UserAgent(), c.ClientIP(), time.Now().Add(middleware.RefreshTokenExpiry)); err != nil {
		return "", "", err
	}

	return accessToken, refreshToken, nil
}

// issueTokenPair signs a new access/refresh token pair without persisting it.
func issueTokenPair(user *models.User) (accessToken, refreshToken string, err error) {
	accessToken, err = middleware.GenerateAccessToken(user)
	if err != nil {
		return "", "", apperrors.Wrap(apperrors.ErrInternalServer, err)
	}

	refreshToken, err = middleware.GenerateRefreshToken(user)
	if err != nil {
		return "", "", apperrors.Wrap(apperrors.ErrInternalServer, err)
	}

	return accessToken, refreshToken, nil
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
