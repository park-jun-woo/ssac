//ff:func feature=pkg-auth type=util control=sequence topic=auth-refresh
//ff:what CreateRefresh — package-level wrapper around currentStore().Create

package auth

import (
	"context"
	"errors"
	"time"
)

// CreateRefresh persists a new refresh-token row on the package-level
// RefreshStore installed via Init. The helper mirrors the nil-fallback
// pattern used by RefreshRotate / Logout so SSaC handlers that issue a
// fresh refresh token (e.g. Login → auth.RefreshToken → auth.CreateRefresh)
// never need to reach through a Server struct field.
//
// Returns an error when Init has not run (defaultStore == nil). Callers
// that explicitly hold a RefreshStore instance should invoke its Create
// method directly instead of routing through this helper.
func CreateRefresh(ctx context.Context, token string, claims any, expiresAt time.Time) error {
	store := currentStore()
	if store == nil {
		return errors.New("auth: refresh store not configured (call auth.Init first)")
	}
	return store.Create(ctx, token, claims, expiresAt)
}
