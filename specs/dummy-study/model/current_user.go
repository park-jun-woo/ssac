package model

// CurrentUser는 인증 미들웨어가 주입하는 현재 사용자 정보다.
type CurrentUser struct {
	ID   int64
	Role string
}
