//ff:type feature=pkg-auth type=model topic=auth-refresh
//ff:what RefreshRotate 응답 모델 — 새로 발급된 access+refresh 쌍
package auth

import "time"

// RefreshRotateResponse is the result of RefreshRotate.
//
// AccessToken / RefreshToken are the newly minted HS256 JWT strings carrying
// the same claim set as the consumed input. ExpiresAt mirrors the refresh
// token's `exp` claim so callers can surface the refresh expiry alongside
// the access expiry if needed.
type RefreshRotateResponse struct {
	AccessToken  string
	RefreshToken string
	ExpiresAt    time.Time
}
