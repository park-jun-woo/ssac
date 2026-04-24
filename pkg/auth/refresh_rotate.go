//ff:func feature=pkg-auth type=util control=sequence topic=auth-refresh
//ff:what refresh token 1회용 rotation 실행 — Verify → Consume → Issue+Refresh+Create
package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
)

// @func refreshRotate
// @error 401
// @description refresh token 1회용 회전 (Verify → Consume → Issue+Refresh+Create)

// RefreshRotate performs the full one-time-use refresh rotation in a single
// atomic sequence so SSaC @call authors can express the flow in one line:
//
//	@call auth.RefreshRotateResponse rotated = auth.RefreshRotate({RefreshToken: request.refresh_token})
//
// Steps:
//  1. VerifyToken — cryptographic signature + expiry check on the input JWT.
//  2. store.Consume — atomic "revoke + return claims". Reuse surfaces as
//     ErrRefreshTokenReused; when `detectReuseLogoutAll` is true, every active
//     token sharing the same claim set is revoked before the error propagates.
//  3. IssueToken + RefreshToken — new access+refresh pair carrying the
//     claims unchanged.
//  4. store.Create — persist the new refresh row under the same claims.
//
// The function never decodes claims; they are passed verbatim through
// json.RawMessage so this helper stays claim-schema-agnostic.
//
// detectReuseLogoutAll is passed explicitly rather than living on the store
// implementation because the reuse-policy is orthogonal to persistence.
func RefreshRotate(ctx context.Context, store RefreshStore, refreshToken string, detectReuseLogoutAll bool) (RefreshRotateResponse, error) {
	if store == nil {
		store = currentStore()
	}
	if store == nil {
		return RefreshRotateResponse{}, errors.New("auth: refresh store not configured")
	}
	if refreshToken == "" {
		return RefreshRotateResponse{}, errors.New("auth: empty refresh token")
	}

	// 1) Cryptographic verification before touching the store.
	if _, err := VerifyToken(VerifyTokenRequest{Token: refreshToken}); err != nil {
		return RefreshRotateResponse{}, fmt.Errorf("auth: verify refresh token: %w", err)
	}

	// 2) Atomic consume (one-time-use). Reuse attempts return the revoked
	// row's claims so family-lockout can be scoped.
	claimsRaw, err := store.Consume(ctx, refreshToken)
	if errors.Is(err, ErrRefreshTokenReused) {
		if detectReuseLogoutAll && len(claimsRaw) > 0 {
			var matcher ClaimMatcher
			if decodeErr := json.Unmarshal(claimsRaw, &matcher); decodeErr == nil && len(matcher) > 0 {
				_ = store.RevokeAll(ctx, matcher)
			}
		}
		return RefreshRotateResponse{}, err
	}
	if err != nil {
		return RefreshRotateResponse{}, err
	}

	// 3) Issue a new access+refresh pair carrying the same claim set.
	access, err := IssueToken(IssueTokenRequest{Claims: claimsRaw})
	if err != nil {
		return RefreshRotateResponse{}, fmt.Errorf("auth: issue access token: %w", err)
	}
	newRefresh, err := RefreshToken(RefreshTokenRequest{Claims: claimsRaw})
	if err != nil {
		return RefreshRotateResponse{}, fmt.Errorf("auth: issue refresh token: %w", err)
	}

	// 4) Persist the new refresh row with the same claim blob.
	if err := store.Create(ctx, newRefresh.RefreshToken, claimsRaw, newRefresh.ExpiresAt); err != nil {
		return RefreshRotateResponse{}, fmt.Errorf("auth: persist new refresh token: %w", err)
	}

	return RefreshRotateResponse{
		AccessToken:  access.AccessToken,
		RefreshToken: newRefresh.RefreshToken,
		ExpiresAt:    newRefresh.ExpiresAt,
	}, nil
}
