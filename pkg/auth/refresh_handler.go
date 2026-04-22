//ff:func feature=pkg-auth type=handler control=sequence topic=auth-refresh
//ff:what POST /auth/refresh 핸들러 — RefreshRotate 래퍼 (deprecated, SSaC 정규 경로 권장)
package auth

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
)

// RefreshHandler returns a gin.HandlerFunc for POST /auth/refresh.
//
// Deprecated: RefreshHandler was the Phase002 "mount /auth/refresh outside
// OpenAPI" stopgap. Phase009 moved the canonical path onto openapi + SSaC:
// declare POST /auth/refresh in openapi.yaml and author a 3-line SSaC file
// that `@call auth.RefreshRotate(...)`. This wrapper is kept for backward
// compatibility only and will be removed in a future major release.
//
// Request body:  {"refresh_token": "..."}
// Response 200:  {"access_token": "...", "refresh_token": "..."}
// Response 401:  invalid / expired / revoked input.
// Response 500:  internal issue / refresh / store persistence failure.
//
// Internally the handler now delegates the full rotation flow to
// RefreshRotate so the two entry points share one implementation.
func RefreshHandler(store *RefreshStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		var body struct {
			RefreshToken string `json:"refresh_token"`
		}
		if err := c.ShouldBindJSON(&body); err != nil || body.RefreshToken == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		rotated, err := RefreshRotate(c.Request.Context(), store, body.RefreshToken)
		if err != nil {
			// Token-domain failures (invalid signature, revoked row, reuse
			// detection, missing token) are 401. Persistence / issuance
			// failures (IssueToken/RefreshToken/Create) surface as 500.
			if isRefreshTokenDomainError(err) {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
				return
			}
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "refresh failed"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"access_token":  rotated.AccessToken,
			"refresh_token": rotated.RefreshToken,
		})
	}
}

// isRefreshTokenDomainError returns true when err represents a caller-facing
// 401 condition (reuse, not-found, or VerifyToken failure). RefreshRotate
// wraps VerifyToken errors with "auth: verify refresh token:" so the legacy
// 401 response mirrors the pre-refactor handler's behavior.
func isRefreshTokenDomainError(err error) bool {
	if errors.Is(err, ErrRefreshTokenReused) {
		return true
	}
	if errors.Is(err, ErrRefreshTokenNotFound) {
		return true
	}
	// VerifyToken-originating errors carry the "verify refresh token" wrap
	// prefix. Using errors.As against the jwt library's error types would
	// couple this package to jwt/v5 internals unnecessarily.
	if err != nil && containsVerifyPrefix(err.Error()) {
		return true
	}
	return false
}

func containsVerifyPrefix(s string) bool {
	const prefix = "auth: verify refresh token:"
	if len(s) < len(prefix) {
		return false
	}
	return s[:len(prefix)] == prefix
}
