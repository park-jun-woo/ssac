# pkg/redact

`slog` 구조화 로그에서 민감 필드를 자동 마스킹하는 `ReplaceAttr` 훅.

## 개요

Go 표준 `log/slog` 핸들러는 `HandlerOptions.ReplaceAttr`에 콜백을 꽂아 속성(attr)을 변형할 수 있다. 이 패키지는 "키 이름이 민감 목록에 들어 있으면 값을 `[REDACTED]`로 치환"하는 콜백을 제공한다.

`DefaultKeys`는 `password`, `token`, `authorization`, `credit_card` 등 흔한 민감 키를 담고 있고, SSaC 코드젠은 여기에 DDL의 `-- @sensitive` 주석이 붙은 컬럼명을 추가한 뒤 애플리케이션 초기화 시점에 logger에 주입한다. 이 훅은 `slog.Info("reset", "password", v)` 같은 애드혹 호출에 대한 방어선이며, 구조체 값에는 별도로 `slog.LogValuer`(생성된 테이블별 `LogValue` 메서드)와 함께 쓰도록 설계되었다.

## 공개 API

### 상수

- `Redacted = "[REDACTED]"` — 마스킹 후 기록되는 문자열 값.

### 변수

- `DefaultKeys map[string]bool` — 기본 민감 키 집합 (소문자 기준). 현재 포함: `password`, `password_hash`, `passwordhash`, `secret`, `token`, `access_token`, `refresh_token`, `api_key`, `apikey`, `ssn`, `credit_card`, `cvv`, `authorization`.

### 함수

- `ReplaceAttr(sensitiveKeys map[string]bool) func([]string, slog.Attr) slog.Attr` — `slog.HandlerOptions.ReplaceAttr`에 그대로 꽂을 수 있는 콜백을 반환. 들어온 `attr.Key`를 소문자로 내려 맵을 조회하고, 일치하면 값을 `Redacted`로 치환, 아니면 그대로 통과. 대소문자 혼용 키(`Authorization` 등)도 마스킹된다. 맵을 읽기만 하므로 동시성 안전이지만, 호출자는 시작 시점에 맵을 만들고 이후 변형하지 않아야 한다.

## 사용 예시

### Go 직접 호출 — 기본 키로 logger 구성

```go
import (
    "log/slog"
    "os"
    "github.com/park-jun-woo/ssac/pkg/redact"
)

func initLogger() *slog.Logger {
    h := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
        ReplaceAttr: redact.ReplaceAttr(redact.DefaultKeys),
    })
    return slog.New(h)
}

// slog.Info("login", "email", "a@b.com", "password", "hunter2")
// → {"msg":"login","email":"a@b.com","password":"[REDACTED]"}
```

### 커스텀 키 추가 — DDL `@sensitive` 또는 도메인 특화 필드

```go
keys := map[string]bool{}
for k, v := range redact.DefaultKeys {
    keys[k] = v
}
keys["national_id"] = true   // 도메인 특화
keys["otp_code"] = true

h := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
    ReplaceAttr: redact.ReplaceAttr(keys),
})
slog.SetDefault(slog.New(h))
```

### SSaC 맥락

SSaC의 코드젠은 애플리케이션 부트스트랩 코드를 만들 때 `DefaultKeys`를 복제하고 DDL의 `-- @sensitive` 컬럼명을 덧붙여 `ReplaceAttr`에 전달한다. 따라서 스펙 작성자는 DDL에서 민감 컬럼만 표시하면 되고, 런타임 마스킹은 자동으로 연결된다. `@call`·`@publish` 등에서 주고받는 페이로드도 `slog.LogValuer`가 붙은 모델 타입이면 구조체 단위로 안전하게 마스킹된다.
