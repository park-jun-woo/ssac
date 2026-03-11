# Phase 026: @call 에러 HTTP 상태 코드 커스텀 지정 + 기본값 500

## 목표

`@call` 에러 시 HTTP 상태 코드를 커스텀 지정할 수 있게 하고, 기본값을 500으로 통일한다.

현재 `call_no_result`는 401, `call_with_result`는 500 — 불일치.
`mail.SendTemplateEmail` 같은 인증 무관 함수도 401이 되는 문제.

## DSL 문법 확장

```ssac
// 기본 (500)
// @call mail.SendTemplateEmail({To: user.Email, Subject: "Welcome"})

// 명시적 지정 (401)
// @call auth.VerifyPassword({Email: email, Password: password}) 401

// result 있음, 기본 500
// @call billing.ChargeResponse charge = billing.Charge({Amount: order.Total})

// result 있음, 402 지정
// @call billing.ChargeResponse charge = billing.Charge({Amount: order.Total}) 402
```

`({...})` 뒤에 3자리 HTTP 상태 코드 (선택).

## 변경 파일 목록

### 1. parser/types.go — Sequence에 ErrStatus 추가

```go
ErrStatus int // @call 에러 HTTP 상태 코드 (0이면 기본값 500)
```

`Message` 필드 옆에 추가.

### 2. parser/parser.go — parseCall에서 trailing 숫자 파싱

`parseCallExprInputs`가 `({...})` 까지 소비한 후 반환. 현재 `TrimSuffix(inner, ")")`로 닫는 괄호를 제거하는데, 그 뒤에 trailing text가 있을 수 있음.

접근법: `parseCallExprInputs`에서 remainder를 반환하도록 확장하거나, `parseCall`에서 원본 rest에서 마지막 `)` 뒤를 직접 파싱.

**선택: parseCallExprInputs 반환값 확장** — `(model, inputs, remainder, error)`.

```go
func parseCallExprInputs(expr string) (string, map[string]string, string, error) {
    // ... 기존 파싱 ...
    // 닫는 괄호 뒤의 remainder 반환
}
```

`parseCall`에서:
```go
model, inputs, remainder, err := parseCallExprInputs(rhs)
// remainder 파싱 → 3자리 숫자면 seq.ErrStatus
```

### 3. generator/go_target.go — templateData에 ErrStatus 추가

```go
type templateData struct {
    // ...
    ErrStatus string // "http.StatusInternalServerError", "http.StatusUnauthorized" 등
}
```

`buildTemplateData`에서 @call 분기:
```go
if seq.Type == parser.SeqCall {
    if seq.ErrStatus != 0 {
        d.ErrStatus = httpStatusConst(seq.ErrStatus)
    } else {
        d.ErrStatus = "http.StatusInternalServerError"
    }
}
```

`httpStatusConst` 헬퍼 추가 (switch 400→StatusBadRequest, 401→StatusUnauthorized, ...).

### 4. generator/go_templates.go — 템플릿에서 ErrStatus 사용

`call_no_result`:
```
- c.JSON(http.StatusUnauthorized, ...)
+ c.JSON({{.ErrStatus}}, ...)
```

`call_with_result`:
```
- c.JSON(http.StatusInternalServerError, ...)
+ c.JSON({{.ErrStatus}}, ...)
```

`sub_call_no_result`, `sub_call_with_result`는 `return fmt.Errorf(...)` — HTTP 상태 코드 없으므로 변경 불필요.

## 의존성

- Phase025 완료 상태

## 검증 방법

### 테스트 추가 (2개)

| 테스트 | 패키지 | 검증 내용 |
|---|---|---|
| `TestParseCallErrStatus` | parser | `@call auth.Verify({...}) 401` → `ErrStatus: 401` |
| `TestGenerateCallErrStatus` | generator | `ErrStatus: 401` → `http.StatusUnauthorized` |

### 기존 테스트 수정

| 테스트 | 변경 |
|---|---|
| `TestGenerateCallWithoutResult` | `StatusUnauthorized` → `StatusInternalServerError` |
| `TestGenerateAuthCallStyle` | `StatusUnauthorized` → `StatusInternalServerError` (있다면) |

### 기존 테스트 확인

```bash
go test ./parser/... ./validator/... ./generator/... -count=1
```

160 + 2 = 162 테스트 목표.

## 상태: ✅ 완료
