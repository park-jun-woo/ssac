//ff:type feature=pkg-auth type=model topic=auth-refresh
//ff:what RefreshRotate 요청 모델 — 교환 대상 refresh token 문자열 하나
package auth

// RefreshRotateRequest holds the inputs for RefreshRotate.
//
// RefreshToken is the JWT string sent by the client on POST /auth/refresh.
// The function performs one-time-use rotation on this token: verifies the
// signature, consumes the store row, and issues a new access+refresh pair
// carrying the same claims.
type RefreshRotateRequest struct {
	RefreshToken string
}
