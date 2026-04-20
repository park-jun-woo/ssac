# pkg/errors

서비스 계층 표준 에러 타입. HTTP 상태코드와 기계 판독 가능한 코드를 함께 운반한다.

## 개요

SSaC가 생성한 gin 핸들러는 `@call`, `@state`, `@auth` 등의 guard 지점에서 반환된 에러를 `errors.As`로 `*ServiceError`에 캐스팅해 내부의 `Status`를 꺼내 `c.JSON(status, ...)`로 응답한다. 그래서 서비스 레이어(특히 `@call`로 호출되는 외부 패키지 함수)는 이 타입을 반환해야 HTTP 상태코드가 정확히 전달된다.

`Cause` 필드는 서버 로그 전용이며 클라이언트에 직접 노출하지 않는다. 핸들러는 `slog.Error(..., "err", se.Cause)`로 내부 원인을 남기고, `Status/Code/Message/Details`만 응답 바디로 내보낸다.

## 공개 API

### 타입

- `ServiceError struct{ Status int; Code string; Message string; Details map[string]any; Cause error }` — 서비스 에러 canonical 타입. OpenAPI `ErrorResponse`(error/code/details)로 매핑된다.
- `(e *ServiceError) Error() string` — `Message`를 그대로 반환. 불투명하게 다루는 호출자도 문자열을 얻는다.
- `(e *ServiceError) Unwrap() error` — `Cause`를 노출해 `errors.Is` / `errors.As`가 체인을 추적할 수 있게 한다.

### 생성자

- `New(status int, code, message string) *ServiceError` — 원인 없이 새 에러 생성. (예: 검증 실패, 잔액 부족 등 의도된 비즈니스 에러)
- `Wrap(status int, code, message string, cause error) *ServiceError` — 하위 에러(`sql.ErrNoRows`, `bcrypt` mismatch 등)를 감싸 서비스 에러로 변환. `cause`는 서버 로그용으로 보존된다.
- `WithDetails(err *ServiceError, details map[string]any) *ServiceError` — 필드별 검증 오류 등 구조화된 부가정보 부착. `nil` 수신자는 안전하게 `nil` 반환.

## 사용 예시

### Go 직접 호출

```go
import "github.com/park-jun-woo/ssac/pkg/errors"

// 의도된 비즈니스 에러
if user.Credits < cost {
    return errors.New(402, "credit_insufficient", "크레딧이 부족합니다")
}

// 하위 에러 래핑 (Cause는 로그용으로만 보존)
row, err := q.GetUser(ctx, id)
if err != nil {
    return errors.Wrap(500, "internal_error", "사용자 조회 실패", err)
}

// 필드 검증 details 부착
return errors.WithDetails(
    errors.New(400, "validation_failed", "입력값이 올바르지 않습니다"),
    map[string]any{"email": []string{"required"}},
)
```

### SSaC 맥락 — 생성 코드에서의 사용

`@call` guard는 반환 에러를 이 타입으로 unwrap해 상태코드를 꺼낸다:

```go
// SSaC:  @call Session session = auth.Login({Email: request.Email, Password: request.Password})
result, err := auth.Login(c.Request.Context(), auth.LoginRequest{...})
if err != nil {
    var se *errors.ServiceError
    if stderrors.As(err, &se) {
        slog.Error("login failed", "err", se.Cause)
        c.JSON(se.Status, gin.H{"error": se.Message, "code": se.Code, "details": se.Details})
        return
    }
    c.JSON(500, gin.H{"error": "internal_error"})
    return
}
```

`@call`로 호출되는 외부 패키지 함수(`pkg.Func`)가 이 패키지의 `ServiceError`를 반환하도록 구현하면, 생성된 핸들러와 HTTP 상태가 자연스럽게 맞춰진다.
