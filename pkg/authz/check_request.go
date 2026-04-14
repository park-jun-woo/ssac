//ff:type feature=pkg-authz type=model
//ff:what 인가 검사 요청 구조체
package authz

// CheckRequest holds the inputs for an authorization check.
type CheckRequest struct {
	Action     string
	Resource   string
	UserID     int64
	Role       string
	ResourceID int64
}
