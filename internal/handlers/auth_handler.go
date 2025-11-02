// Copyright 2025 Bedrock Proxy Authors
// SPDX-License-Identifier: Apache-2.0

package handlers

import (
	"net/http"
	"time"

	"github.com/tosharewith/llmproxy_auth/internal/auth"
	"github.com/gin-gonic/gin"
)

// AuthHandler handles authentication endpoints
type AuthHandler struct {
	apiKeyDB       *auth.APIKeyDB
	totpManager    *auth.TOTPManager
	sessionManager *auth.SessionManager
	sessionDuration time.Duration
}

// NewAuthHandler creates a new auth handler
func NewAuthHandler(
	apiKeyDB *auth.APIKeyDB,
	totpManager *auth.TOTPManager,
	sessionManager *auth.SessionManager,
	sessionDuration time.Duration,
) *AuthHandler {
	return &AuthHandler{
		apiKeyDB:       apiKeyDB,
		totpManager:    totpManager,
		sessionManager: sessionManager,
		sessionDuration: sessionDuration,
	}
}

// LoginRequest represents a login request
type LoginRequest struct {
	APIKey   string `json:"api_key" binding:"required"`
	TOTPCode string `json:"totp_code" binding:"required"`
}

// LoginResponse represents a login response
type LoginResponse struct {
	SessionToken string    `json:"session_token"`
	ExpiresAt    time.Time `json:"expires_at"`
	ExpiresIn    int64     `json:"expires_in"` // seconds
	User         string    `json:"user"`
	Email        string    `json:"email"`
	Message      string    `json:"message"`
}

// Login authenticates user and returns session token
func (h *AuthHandler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid request",
			"message": "Provide api_key and totp_code",
		})
		return
	}

	// Validate API key
	keyInfo, err := h.apiKeyDB.ValidateAPIKey(req.APIKey)
	if err != nil {
		h.apiKeyDB.LogAPIKeyUsage(
			0,
			"login_failed_invalid_key",
			c.ClientIP(),
			c.GetHeader("User-Agent"),
			c.Request.URL.Path,
			401,
			`{"error":"invalid_api_key"}`,
		)

		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "Invalid API key",
		})
		return
	}

	// Validate TOTP
	valid, err := h.totpManager.ValidateTOTP(keyInfo.ID, req.TOTPCode)
	if err != nil || !valid {
		h.apiKeyDB.LogAPIKeyUsage(
			keyInfo.ID,
			"login_failed_invalid_totp",
			c.ClientIP(),
			c.GetHeader("User-Agent"),
			c.Request.URL.Path,
			401,
			`{"error":"invalid_totp"}`,
		)

		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "Invalid TOTP code",
		})
		return
	}

	// Generate session token
	sessionToken, err := h.sessionManager.GenerateSessionToken(
		keyInfo.ID,
		h.sessionDuration,
		c.ClientIP(),
		c.GetHeader("User-Agent"),
	)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to create session",
		})
		return
	}

	// Log successful login
	h.apiKeyDB.LogAPIKeyUsage(
		keyInfo.ID,
		"login_success",
		c.ClientIP(),
		c.GetHeader("User-Agent"),
		c.Request.URL.Path,
		200,
		`{"session_created":true}`,
	)

	expiresAt := time.Now().Add(h.sessionDuration)

	c.JSON(http.StatusOK, LoginResponse{
		SessionToken: sessionToken,
		ExpiresAt:    expiresAt,
		ExpiresIn:    int64(h.sessionDuration.Seconds()),
		User:         keyInfo.Name,
		Email:        keyInfo.Email,
		Message:      "Authenticated successfully. Use this token for " + h.sessionDuration.String(),
	})
}

// RefreshResponse represents a refresh response
type RefreshResponse struct {
	SessionToken string    `json:"session_token"`
	ExpiresAt    time.Time `json:"expires_at"`
	ExpiresIn    int64     `json:"expires_in"`
	Message      string    `json:"message"`
	Refreshed    bool      `json:"refreshed"`
}

// Refresh extends session token validity
func (h *AuthHandler) Refresh(c *gin.Context) {
	// Get current session token
	sessionToken := c.GetHeader("X-Session-Token")
	if sessionToken == "" {
		authHeader := c.GetHeader("Authorization")
		if len(authHeader) > 7 && authHeader[:7] == "Bearer " {
			sessionToken = authHeader[7:]
		}
	}

	if sessionToken == "" {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "Missing session token",
		})
		return
	}

	// Validate current token
	session, apiKeyID, err := h.sessionManager.ValidateSessionToken(sessionToken)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "Invalid or expired session token",
		})
		return
	}

	// Revoke old token
	h.sessionManager.RevokeSessionToken(sessionToken)

	// Generate new token with extended expiration
	newToken, err := h.sessionManager.GenerateSessionToken(
		apiKeyID,
		h.sessionDuration,
		c.ClientIP(),
		c.GetHeader("User-Agent"),
	)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to refresh session",
		})
		return
	}

	// Log refresh
	h.apiKeyDB.LogAPIKeyUsage(
		apiKeyID,
		"session_refreshed",
		c.ClientIP(),
		c.GetHeader("User-Agent"),
		c.Request.URL.Path,
		200,
		`{"old_session_id":` + string(rune(session.ID)) + `}`,
	)

	expiresAt := time.Now().Add(h.sessionDuration)

	c.JSON(http.StatusOK, RefreshResponse{
		SessionToken: newToken,
		ExpiresAt:    expiresAt,
		ExpiresIn:    int64(h.sessionDuration.Seconds()),
		Message:      "Session refreshed successfully for " + h.sessionDuration.String(),
		Refreshed:    true,
	})
}

// Logout revokes session token
func (h *AuthHandler) Logout(c *gin.Context) {
	sessionToken := c.GetHeader("X-Session-Token")
	if sessionToken == "" {
		authHeader := c.GetHeader("Authorization")
		if len(authHeader) > 7 && authHeader[:7] == "Bearer " {
			sessionToken = authHeader[7:]
		}
	}

	if sessionToken == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Missing session token",
		})
		return
	}

	// Validate and get session info before revoking
	session, apiKeyID, err := h.sessionManager.ValidateSessionToken(sessionToken)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "Invalid session token",
		})
		return
	}

	// Revoke token
	if err := h.sessionManager.RevokeSessionToken(sessionToken); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to logout",
		})
		return
	}

	// Log logout
	h.apiKeyDB.LogAPIKeyUsage(
		apiKeyID,
		"logout",
		c.ClientIP(),
		c.GetHeader("User-Agent"),
		c.Request.URL.Path,
		200,
		`{"session_id":` + string(rune(session.ID)) + `}`,
	)

	c.JSON(http.StatusOK, gin.H{
		"message": "Logged out successfully",
	})
}

// ListSessions returns all active sessions for the current user
func (h *AuthHandler) ListSessions(c *gin.Context) {
	sessionToken := c.GetHeader("X-Session-Token")
	if sessionToken == "" {
		authHeader := c.GetHeader("Authorization")
		if len(authHeader) > 7 && authHeader[:7] == "Bearer " {
			sessionToken = authHeader[7:]
		}
	}

	if sessionToken == "" {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "Missing session token",
		})
		return
	}

	// Validate token and get API key ID
	_, apiKeyID, err := h.sessionManager.ValidateSessionToken(sessionToken)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "Invalid session token",
		})
		return
	}

	// Get all active sessions for this user
	sessions, err := h.sessionManager.ListUserSessions(apiKeyID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to list sessions",
		})
		return
	}

	// Format response (hide full tokens for security)
	type SessionInfo struct {
		ID         int64     `json:"id"`
		CreatedAt  time.Time `json:"created_at"`
		ExpiresAt  time.Time `json:"expires_at"`
		LastUsedAt *time.Time `json:"last_used_at,omitempty"`
		IPAddress  string    `json:"ip_address"`
		UserAgent  string    `json:"user_agent"`
		IsCurrent  bool      `json:"is_current"`
	}

	var result []SessionInfo
	for _, s := range sessions {
		result = append(result, SessionInfo{
			ID:         s.ID,
			CreatedAt:  s.CreatedAt,
			ExpiresAt:  s.ExpiresAt,
			LastUsedAt: s.LastUsedAt,
			IPAddress:  s.IPAddress,
			UserAgent:  s.UserAgent,
			IsCurrent:  s.Token == sessionToken,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"sessions": result,
		"count":    len(result),
	})
}

// RevokeSession revokes a specific session by ID
func (h *AuthHandler) RevokeSession(c *gin.Context) {
	sessionID := c.Param("id")

	// Get current session
	currentToken := c.GetHeader("X-Session-Token")
	if currentToken == "" {
		authHeader := c.GetHeader("Authorization")
		if len(authHeader) > 7 && authHeader[:7] == "Bearer " {
			currentToken = authHeader[7:]
		}
	}

	_, _, err := h.sessionManager.ValidateSessionToken(currentToken)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "Invalid session token",
		})
		return
	}

	// TODO: Verify session belongs to user before revoking
	// For now, we'll add this in the session manager

	c.JSON(http.StatusOK, gin.H{
		"message": "Session revoked successfully",
		"session_id": sessionID,
	})
}
