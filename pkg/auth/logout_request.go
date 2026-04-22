//ff:type feature=pkg-auth type=model topic=auth-refresh
//ff:what Logout 요청 모델 — 폐기 대상 refresh token 문자열 하나
package auth

// LogoutRequest holds the inputs for Logout. RefreshToken is the plaintext
// JWT the caller wants to revoke. An empty or unknown token is treated as
// a no-op (idempotent); callers receive a nil error either way.
type LogoutRequest struct {
	RefreshToken string
}
