# pkg/mail

SMTP 기반 이메일 발송 유틸리티 패키지.

## 개요

SSaC `@call mail.SendEmail({...})` / `@call mail.SendTemplateEmail({...})` 시퀀스로 호출되는 이메일 발송 패키지. Go 표준 `net/smtp`를 사용해 plain text 또는 HTML 템플릿 이메일을 전송한다. `SendEmail`은 SMTP 설정을 Request 필드로 직접 받고, `SendTemplateEmail`은 `SMTP_HOST`, `SMTP_PORT`, `SMTP_USERNAME`, `SMTP_PASSWORD`, `SMTP_FROM` 환경변수에서 설정을 읽는다. 모든 함수는 `ctx`를 첫 인자로 받지만, 현 구현은 `net/smtp`가 ctx 미지원이라 파라미터만 보유한다(향후 ctx-aware dialer 마이그레이션 대비).

## 공개 API

### `SendEmail(ctx context.Context, req SendEmailRequest) (SendEmailResponse, error)`

plain text 이메일을 발송한다.

#### SendEmailRequest

| 필드 | 타입 | 설명 |
|---|---|---|
| Host | string | SMTP 호스트 (예: `smtp.gmail.com`) |
| Port | int | SMTP 포트 (예: 587) |
| Username | string | SMTP 인증 사용자명 |
| Password | string | SMTP 인증 비밀번호 |
| From | string | 발신자 주소 |
| To | string | 수신자 주소 |
| Subject | string | 제목 |
| Body | string | 본문 (plain text) |

#### SendEmailResponse

비어 있음. 발송 성공 여부는 `error`로 판정.

### `SendTemplateEmail(ctx context.Context, req SendTemplateEmailRequest) (SendTemplateEmailResponse, error)`

Go `html/template`로 렌더링한 HTML 이메일을 발송한다. SMTP 설정은 환경변수에서 로드한다(`SMTP_HOST`, `SMTP_PORT`, `SMTP_USERNAME`, `SMTP_PASSWORD`, `SMTP_FROM`). `TemplateName` 문자열을 템플릿 소스로 파싱 후 nil 데이터로 실행한다.

#### SendTemplateEmailRequest

| 필드 | 타입 | 설명 |
|---|---|---|
| To | string | 수신자 주소 |
| Subject | string | 제목 |
| TemplateName | string | 템플릿 소스 문자열 |

#### SendTemplateEmailResponse

비어 있음.

## 사용 예시

### SSaC 시퀀스

```go
// @call mail.SendEmail({
//   Host: "smtp.gmail.com",
//   Port: 587,
//   Username: config.SmtpUser,  // 실제로는 os.Getenv 내부 사용
//   From: "noreply@example.com",
//   To: user.Email,
//   Subject: "환영합니다",
//   Body: "가입을 환영합니다."
// })
```

`@call`은 result가 없으므로 `_, err` guard 형으로 생성되어 500 응답으로 떨어진다.

### Go 직접 호출

```go
import (
    "context"
    "github.com/park-jun-woo/ssac/pkg/mail"
)

_, err := mail.SendEmail(ctx, mail.SendEmailRequest{
    Host:     "smtp.gmail.com",
    Port:     587,
    Username: "user@example.com",
    Password: os.Getenv("SMTP_PASSWORD"),
    From:     "noreply@example.com",
    To:       "recipient@example.com",
    Subject:  "Hello",
    Body:     "plain text body",
})

_, err = mail.SendTemplateEmail(ctx, mail.SendTemplateEmailRequest{
    To:           "recipient@example.com",
    Subject:      "HTML Mail",
    TemplateName: "<h1>Hello {{.Name}}</h1>",
})
```
