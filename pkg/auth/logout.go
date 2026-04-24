//ff:func feature=pkg-auth type=util control=sequence topic=auth-refresh
//ff:what refresh token 폐기 (idempotent) — Logout 정규 경로
package auth

import (
	"context"
	"errors"
)

// @func logout
// @error 500
// @description refresh token 폐기 (idempotent) — 이미 revoked/만료/미존재 모두 nil error

// Logout revokes the supplied refresh token via RefreshStore.Revoke. It is
// idempotent: an already-revoked, expired, or unknown token returns a nil
// error. Intended as the `@call auth.Logout` target in SSaC for the
// canonical POST /auth/logout endpoint.
func Logout(ctx context.Context, store RefreshStore, refreshToken string) (LogoutResponse, error) {
	if store == nil {
		store = currentStore()
	}
	if store == nil {
		return LogoutResponse{}, errors.New("auth: refresh store not configured")
	}
	if refreshToken == "" {
		// Silent no-op. Logging out without a token is semantically a
		// successful logout — the client simply has no server-side session
		// to drop.
		return LogoutResponse{Success: true}, nil
	}
	if err := store.Revoke(ctx, refreshToken); err != nil {
		return LogoutResponse{}, err
	}
	return LogoutResponse{Success: true}, nil
}
