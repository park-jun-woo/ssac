# pkg/crypto

AES-256-GCM 대칭 암·복호화와 TOTP 기반 OTP 생성/검증 유틸 패키지.

## 개요

민감 데이터 저장 시 사용할 AES-256-GCM 암호화와, 2단계 인증(2FA)용 TOTP(RFC 6238) 시크릿 발급·코드 검증을 제공한다. 암호화 키는 32바이트 hex 문자열(64자)로 전달받고, 암호문은 base64로 인코딩된 `nonce || ciphertext` 구조로 주고받는다. OTP는 `otpauth://` URL을 함께 반환하므로 QR 코드 렌더링에 바로 쓸 수 있다. SSaC 코드젠이 만든 핸들러에서 `@call crypto.Func({...})` 시퀀스로 호출된다.

## 공개 API

### Encrypt

평문을 AES-256-GCM으로 암호화한다. GCM nonce는 매 호출마다 `crypto/rand`로 새로 생성되어 암호문 앞에 프리픽스된 뒤 전체를 base64로 인코딩한다.

Request:

| 필드 | 타입 | 설명 |
|---|---|---|
| Plaintext | string | 평문 |
| Key | string | 32바이트 키 (hex 인코딩, 64자) |

Response:

| 필드 | 타입 | 설명 |
|---|---|---|
| Ciphertext | string | base64(`nonce \|\| sealed`) |

에러 조건: 키 hex 디코딩 실패, 키 길이 불일치(AES-256은 32바이트), nonce 생성 실패.

### Decrypt

AES-256-GCM 암호문을 복호화한다. 입력은 `Encrypt`가 만든 `base64(nonce || ciphertext)` 포맷을 기대한다.

Request:

| 필드 | 타입 | 설명 |
|---|---|---|
| Ciphertext | string | base64 인코딩된 암호문 |
| Key | string | 32바이트 키 (hex) |

Response:

| 필드 | 타입 | 설명 |
|---|---|---|
| Plaintext | string | 복호화된 평문 |

에러 조건: base64/hex 디코딩 실패, 암호문이 nonce 크기보다 짧음(`ciphertext too short`), GCM 인증 태그 불일치(위변조 감지).

### GenerateOTP

`Issuer`/`AccountName`을 받아 TOTP 시크릿과 프로비저닝 URL을 생성한다.

Request:

| 필드 | 타입 | 설명 |
|---|---|---|
| Issuer | string | 서비스 이름 (예: `"ACME"`) |
| AccountName | string | 사용자 식별자 (예: 이메일) |

Response:

| 필드 | 타입 | 설명 |
|---|---|---|
| Secret | string | base32 인코딩된 TOTP 시크릿 |
| URL | string | `otpauth://totp/...` URL (QR 코드용) |

에러 조건: `totp.Generate` 실패 시 해당 에러 반환.

### VerifyOTP

6자리 TOTP 코드가 시크릿과 일치하는지 검증한다.

Request:

| 필드 | 타입 | 설명 |
|---|---|---|
| Code | string | 사용자 입력 6자리 코드 |
| Secret | string | DB에 저장된 base32 시크릿 |

Response: (비어있음)

에러 조건: 불일치 시 `invalid OTP code`.

## 사용 예시

SSaC 시퀀스에서의 `@call`:

```go
// 2FA 설정: OTP 시크릿 발급 후 저장
// @call crypto.OTP otp = crypto.GenerateOTP({Issuer: "ACME", AccountName: currentUser.Email})
// @put User.SetOTPSecret({ID: currentUser.ID, Secret: otp.Secret})
// @response otp
```

```go
// 민감 필드 암호화 저장
// @call crypto.Cipher cipher = crypto.Encrypt({Plaintext: request.SSN, Key: config.EncKey})
// @post Record record = Record.Create({UserID: currentUser.ID, EncryptedSSN: cipher.Ciphertext})
// @response record
```

Go 직접 호출:

```go
key := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef" // 32바이트 hex

enc, err := crypto.Encrypt(crypto.EncryptRequest{Plaintext: "hello", Key: key})
if err != nil { /* ... */ }

dec, err := crypto.Decrypt(crypto.DecryptRequest{Ciphertext: enc.Ciphertext, Key: key})
// dec.Plaintext == "hello"

otp, _ := crypto.GenerateOTP(crypto.GenerateOTPRequest{
    Issuer: "ACME", AccountName: "user@example.com",
})
// otp.URL → QR 렌더

_, err = crypto.VerifyOTP(crypto.VerifyOTPRequest{Code: "123456", Secret: otp.Secret})
// err == nil → 일치, "invalid OTP code" → 불일치
```

## 외부 의존성

- `crypto/aes`, `crypto/cipher`, `crypto/rand` (표준 라이브러리) — AES-256-GCM
- `encoding/base64`, `encoding/hex` — 키/암호문 인코딩
- `github.com/pquerna/otp/totp` — TOTP 시크릿 생성 및 코드 검증
