# pkg/auth

비밀번호 해싱/검증과 리셋 토큰 발급을 담당하는 인증 유틸 패키지.

## 개요

로그인, 회원가입, 비밀번호 리셋 플로우에서 호출되는 저수준 유틸리티들을 모은다. `bcrypt`를 사용한 비밀번호 해싱/검증과 `crypto/rand` 기반 리셋 토큰 생성을 제공하며, 타이밍 공격으로 인한 계정 열거(enumeration)를 막기 위한 `DummyHash` 상수를 함께 노출한다. SSaC 코드젠이 만든 핸들러에서 `@call auth.Func({...})` 시퀀스로 호출된다.

## 공개 API

### HashPassword

평문 비밀번호를 bcrypt 해시로 변환한다 (cost = `bcrypt.DefaultCost`).

Request:

| 필드 | 타입 | 설명 |
|---|---|---|
| Password | string | 평문 비밀번호 |

Response:

| 필드 | 타입 | 설명 |
|---|---|---|
| HashedPassword | string | bcrypt 해시 문자열 |

에러 조건: bcrypt 해시 생성 실패 시 `bcrypt.GenerateFromPassword`의 에러를 그대로 반환.

### VerifyPassword

저장된 bcrypt 해시와 평문 비밀번호가 일치하는지 검증한다. 생성 코드에서 `@error 401` 시맨틱으로 처리된다.

Request:

| 필드 | 타입 | 설명 |
|---|---|---|
| PasswordHash | string | DB에 저장된 bcrypt 해시 |
| Password | string | 사용자 입력 평문 |

Response: (비어있음)

에러 조건: 불일치 시 `bcrypt.ErrMismatchedHashAndPassword`. 해시 포맷 오류 시 `bcrypt.ErrHashTooShort` 등.

### GenerateResetToken

비밀번호 리셋 플로우에 사용할 32바이트 랜덤 hex 토큰(64자)을 생성한다.

Request: (비어있음)

Response:

| 필드 | 타입 | 설명 |
|---|---|---|
| Token | string | 64자 hex 문자열 |

에러 조건: `crypto/rand.Read` 실패 시 에러 반환.

### DummyHash (상수)

타이밍 공격 방어용으로 미리 계산된 bcrypt 해시 상수.

로그인 흐름에서 이메일 조회가 미스(사용자 없음)했을 때도 공격자가 제출한 비밀번호를 `DummyHash`에 대해 `VerifyPassword`로 한 번 돌리는 용도. 두 분기(사용자 존재 / 부재) 모두 약 60ms의 bcrypt 비용을 지불하게 만들어, 응답 시간 차이로 이메일 존재 여부를 유추하는 공격을 차단한다. 해시의 평문은 도달 불가능한 센티넬이라 실제 사용자 입력과는 절대 일치하지 않는다.

주의: cost factor(`bcrypt.DefaultCost`)를 바꿀 경우, 두 분기의 타이밍 대칭이 깨지지 않도록 `DummyHash`도 함께 재생성해야 한다.

## 사용 예시

SSaC 시퀀스에서의 `@call`:

```go
// 로그인: 사용자 조회 실패 시에도 타이밍 대칭을 위해 더미 해시 검증
// @get User user = User.GetByEmail({Email: request.Email})
// @call auth.VerifyPassword({PasswordHash: user.PasswordHash, Password: request.Password})
// @response user
```

```go
// 비밀번호 리셋 토큰 발급
// @call string token = auth.GenerateResetToken({})
// @put PasswordReset.Create({UserID: user.ID, Token: token.Token})
// @response token
```

Go 직접 호출:

```go
hashed, err := auth.HashPassword(auth.HashPasswordRequest{Password: "s3cret"})
if err != nil { /* ... */ }

_, err = auth.VerifyPassword(auth.VerifyPasswordRequest{
    PasswordHash: hashed.HashedPassword,
    Password:     "s3cret",
})
// err == nil → 일치
// err == bcrypt.ErrMismatchedHashAndPassword → 불일치

// 사용자 미존재 시에도 타이밍 대칭 유지
_, _ = auth.VerifyPassword(auth.VerifyPasswordRequest{
    PasswordHash: auth.DummyHash,
    Password:     attackerInput,
})
```

## 외부 의존성

- `golang.org/x/crypto/bcrypt` — 비밀번호 해싱/검증
- `crypto/rand`, `encoding/hex` (표준 라이브러리) — 리셋 토큰
