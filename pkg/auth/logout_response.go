//ff:type feature=pkg-auth type=model topic=auth-refresh
//ff:what Logout 응답 모델 — 성공 여부 플래그
package auth

// LogoutResponse is the result of Logout. Success is always true when the
// function returns nil error; the field exists so SSaC `@response` can emit
// a non-empty JSON body (`{"success": true}`) without introducing a shared
// "empty" struct.
type LogoutResponse struct {
	Success bool
}
