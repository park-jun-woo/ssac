//ff:func feature=pkg-auth type=handler control=sequence topic=auth-refresh
//ff:what POST /auth/refresh 핸들러 — refresh token rotation (one-time-use, HS256)
package auth

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
)

// RefreshHandler returns a gin.HandlerFunc for POST /auth/refresh.
//
// Mount it in main.go after Configure + RefreshStore are built:
//
//	r.POST("/auth/refresh", auth.RefreshHandler(&auth.RefreshStore{DB: conn}))
//
// Request body:  {"refresh_token": "..."}
// Response 200:  {"access_token": "...", "refresh_token": "..."}
// Response 401:  invalid / expired / revoked input.
//
// Claims are treated as opaque JSON: the handler never decodes them. The
// previous refresh row's `claims` blob is passed verbatim to IssueToken /
// RefreshToken so the user's session retains role/org/email etc.
func RefreshHandler(store *RefreshStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		var body struct {
			RefreshToken string `json:"refresh_token"`
		}
		if err := c.ShouldBindJSON(&body); err != nil || body.RefreshToken == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		// 1) Cryptographic verification — ensures the token came from us and is
		// not expired. Must pass before we touch the DB.
		if _, err := VerifyToken(VerifyTokenRequest{Token: body.RefreshToken}); err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		// 2) Atomic consume (one-time-use rotation). Reuse attempts return the
		// revoked row's claims so RevokeAll can scope the lockout.
		claimsRaw, err := store.Consume(c.Request.Context(), body.RefreshToken)
		if errors.Is(err, ErrRefreshTokenReused) {
			if store.DetectReuseLogoutAll && len(claimsRaw) > 0 {
				var matcher ClaimMatcher
				if decodeErr := json.Unmarshal(claimsRaw, &matcher); decodeErr == nil && len(matcher) > 0 {
					_ = store.RevokeAll(c.Request.Context(), matcher)
				}
			}
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		// 3) Issue a new access+refresh pair carrying the same claim set.
		access, err := IssueToken(IssueTokenRequest{Claims: claimsRaw})
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "issue failed"})
			return
		}
		newRefresh, err := RefreshToken(RefreshTokenRequest{Claims: claimsRaw})
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "refresh failed"})
			return
		}

		// 4) Persist the new refresh row with the same claim blob.
		if err := store.Create(c.Request.Context(), newRefresh.RefreshToken, claimsRaw, newRefresh.ExpiresAt); err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "store failed"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"access_token":  access.AccessToken,
			"refresh_token": newRefresh.RefreshToken,
		})
	}
}
