// Copyright 2025 Bedrock Proxy Authors
// SPDX-License-Identifier: Apache-2.0

package middleware

import (
	"net/http"
	"strings"

	"github.com/tosharewith/llmproxy_auth/internal/auth"
	"github.com/gin-gonic/gin"
)

// SessionTokenAuth validates session tokens (no TOTP needed after initial auth)
func SessionTokenAuth(sessionManager *auth.SessionManager, apiKeyDB *auth.APIKeyDB) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Extract session token from header or Authorization Bearer
		sessionToken := c.GetHeader("X-Session-Token")
		if sessionToken == "" {
			authHeader := c.GetHeader("Authorization")
			if strings.HasPrefix(authHeader, "Bearer ") {
				sessionToken = strings.TrimPrefix(authHeader, "Bearer ")
			}
		}

		if sessionToken == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error":   "Missing session token",
				"message": "Provide session token via X-Session-Token header or Authorization: Bearer <token>",
				"hint":    "Get a session token by calling /auth/login with API key + TOTP",
			})
			c.Abort()
			return
		}

		// Validate session token
		session, apiKeyID, err := sessionManager.ValidateSessionToken(sessionToken)
		if err != nil {
			// Log failed attempt
			apiKeyDB.LogAPIKeyUsage(
				0,
				"session_auth_failed",
				c.ClientIP(),
				c.GetHeader("User-Agent"),
				c.Request.URL.Path,
				401,
				`{"error":"invalid_session_token"}`,
			)

			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Invalid or expired session token",
			})
			c.Abort()
			return
		}

		// Get API key info
		keyInfo, err := apiKeyDB.GetAPIKeyByID(apiKeyID)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Session token valid but API key not found",
			})
			c.Abort()
			return
		}

		// Set user context
		c.Set("user", keyInfo.Name)
		c.Set("user_email", keyInfo.Email)
		c.Set("api_key_id", apiKeyID)
		c.Set("session_id", session.ID)
		c.Set("auth_method", "session_token")

		// Log successful authentication
		apiKeyDB.LogAPIKeyUsage(
			apiKeyID,
			"session_auth_success",
			c.ClientIP(),
			c.GetHeader("User-Agent"),
			c.Request.URL.Path,
			200,
			`{"session_id":` + intToString(int(session.ID)) + `}`,
		)

		c.Next()
	}
}

// HybridAuth accepts either session token OR API key + TOTP
func HybridAuth(
	sessionManager *auth.SessionManager,
	apiKeyDB *auth.APIKeyDB,
	totpManager *auth.TOTPManager,
	require2FA bool,
) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Try session token first
		sessionToken := c.GetHeader("X-Session-Token")
		if sessionToken == "" {
			authHeader := c.GetHeader("Authorization")
			if strings.HasPrefix(authHeader, "Bearer ") {
				sessionToken = strings.TrimPrefix(authHeader, "Bearer ")
			}
		}

		// If has session token, validate it
		if sessionToken != "" {
			session, apiKeyID, err := sessionManager.ValidateSessionToken(sessionToken)
			if err == nil {
				// Valid session token - authenticated!
				keyInfo, _ := apiKeyDB.GetAPIKeyByID(apiKeyID)
				c.Set("user", keyInfo.Name)
				c.Set("user_email", keyInfo.Email)
				c.Set("api_key_id", apiKeyID)
				c.Set("session_id", session.ID)
				c.Set("auth_method", "session_token")
				c.Next()
				return
			}
			// Session token invalid, fall through to API key + TOTP
		}

		// No valid session token, require API key + TOTP
		apiKey := c.GetHeader("X-API-Key")
		if apiKey == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error":   "Authentication required",
				"message": "Provide either session token or API key + TOTP",
				"hint":    "Get session token: POST /auth/login with API key + TOTP",
			})
			c.Abort()
			return
		}

		// Validate API key
		keyInfo, err := apiKeyDB.ValidateAPIKey(apiKey)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Invalid API key",
			})
			c.Abort()
			return
		}

		// Check if 2FA is required
		twoFAEnabled, _ := totpManager.IsTOTPEnabled(keyInfo.ID)
		if twoFAEnabled || require2FA {
			totpCode := c.GetHeader("X-TOTP-Code")
			if totpCode == "" {
				c.JSON(http.StatusUnauthorized, gin.H{
					"error":   "2FA required",
					"message": "Provide TOTP code via X-TOTP-Code header, or use session token",
					"hint":    "Get session token: POST /auth/login with API key + TOTP",
				})
				c.Abort()
				return
			}

			valid, err := totpManager.ValidateTOTP(keyInfo.ID, totpCode)
			if err != nil || !valid {
				c.JSON(http.StatusUnauthorized, gin.H{
					"error": "Invalid TOTP code",
				})
				c.Abort()
				return
			}
		}

		// Authenticated with API key + TOTP
		c.Set("user", keyInfo.Name)
		c.Set("user_email", keyInfo.Email)
		c.Set("api_key_id", keyInfo.ID)
		c.Set("auth_method", "api_key_totp")

		c.Next()
	}
}
