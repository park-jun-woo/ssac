//go:build ignore

package service

import _ "github.com/geul-org/fullend/pkg/auth"

// @get User user = User.FindByEmail({Email: request.Email})
// @empty user "사용자를 찾을 수 없습니다"
// @call auth.VerifyPassword({PasswordHash: user.PasswordHash, Password: request.Password})
// @post Token token = Session.Create({UserID: user.ID})
// @response {
//   token: token
// }
func Login() {}
