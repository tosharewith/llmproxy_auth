// Copyright 2025 Bedrock Proxy Authors
// SPDX-License-Identifier: Apache-2.0

package middleware

import (
	"net/http"
	"strings"

	"github.com/tosharewith/llmproxy_auth/internal/auth"
	"github.com/gin-gonic/gin"
)

// EnhancedAPIKeyAuth validates API keys from database with optional 2FA
func EnhancedAPIKeyAuth(apiKeyDB *auth.APIKeyDB, totpManager *auth.TOTPManager, require2FA bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Extract API key from header
		apiKey := c.GetHeader("X-API-Key")
		if apiKey == "" {
			authHeader := c.GetHeader("Authorization")
			if strings.HasPrefix(authHeader, "Bearer ") {
				apiKey = strings.TrimPrefix(authHeader, "Bearer ")
			}
		}

		if apiKey == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Missing API key",
				"message": "Provide API key via X-API-Key header or Authorization: Bearer <key>",
			})
			c.Abort()
			return
		}

		// Validate API key against database
		keyInfo, err := apiKeyDB.ValidateAPIKey(apiKey)
		if err != nil {
			// Log failed attempt
			apiKeyDB.LogAPIKeyUsage(
				0, // unknown key ID
				"auth_failed",
				c.ClientIP(),
				c.GetHeader("User-Agent"),
				c.Request.URL.Path,
				401,
				`{"error":"invalid_api_key"}`,
			)

			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Invalid API key",
			})
			c.Abort()
			return
		}

		// Check if 2FA is enabled for this key
		twoFAEnabled, _ := totpManager.IsTOTPEnabled(keyInfo.ID)

		if twoFAEnabled || require2FA {
			// Extract TOTP code from header
			totpCode := c.GetHeader("X-TOTP-Code")
			if totpCode == "" {
				c.JSON(http.StatusUnauthorized, gin.H{
					"error": "2FA required",
					"message": "Provide TOTP code via X-TOTP-Code header",
				})
				c.Abort()
				return
			}

			// Validate TOTP code
			valid, err := totpManager.ValidateTOTP(keyInfo.ID, totpCode)
			if err != nil || !valid {
				// Log failed 2FA attempt
				apiKeyDB.LogAPIKeyUsage(
					keyInfo.ID,
					"2fa_failed",
					c.ClientIP(),
					c.GetHeader("User-Agent"),
					c.Request.URL.Path,
					401,
					`{"error":"invalid_totp"}`,
				)

				c.JSON(http.StatusUnauthorized, gin.H{
					"error": "Invalid TOTP code",
				})
				c.Abort()
				return
			}
		}

		// Set user context
		c.Set("user", keyInfo.Name)
		c.Set("user_email", keyInfo.Email)
		c.Set("api_key_id", keyInfo.ID)
		c.Set("auth_method", "api_key_db")
		c.Set("2fa_enabled", twoFAEnabled)

		// Log successful authentication
		apiKeyDB.LogAPIKeyUsage(
			keyInfo.ID,
			"auth_success",
			c.ClientIP(),
			c.GetHeader("User-Agent"),
			c.Request.URL.Path,
			200,
			`{"2fa_used":` + boolToString(twoFAEnabled) + `}`,
		)

		c.Next()

		// Log request completion (after processing)
		statusCode := c.Writer.Status()
		apiKeyDB.LogAPIKeyUsage(
			keyInfo.ID,
			"request_completed",
			c.ClientIP(),
			c.GetHeader("User-Agent"),
			c.Request.URL.Path,
			statusCode,
			`{"status":` + intToString(statusCode) + `}`,
		)
	}
}

// AuditLogger logs all requests for compliance
func AuditLogger(apiKeyDB *auth.APIKeyDB) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get user context
		keyID, exists := c.Get("api_key_id")
		if !exists {
			keyID = int64(0)
		}

		user, _ := c.Get("user")
		email, _ := c.Get("user_email")

		// Process request
		c.Next()

		// Log audit trail
		apiKeyDB.LogAPIKeyUsage(
			keyID.(int64),
			"audit",
			c.ClientIP(),
			c.GetHeader("User-Agent"),
			c.Request.URL.Path,
			c.Writer.Status(),
			`{"user":"`+toString(user)+`","email":"`+toString(email)+`","method":"`+c.Request.Method+`"}`,
		)
	}
}

// Helper functions
func boolToString(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

func intToString(i int) string {
	return string(rune(i))
}

func toString(v interface{}) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}
