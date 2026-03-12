# Phase028: ErrStatus를 @empty, @exists, @state, @auth에 확장

## 목표

trailing HTTP 상태 코드(`ErrStatus`)를 `@call` 외에 `@empty`, `@exists`, `@state`, `@auth`에도 지원한다.
기존 문법에 optional trailing 숫자를 추가하는 것이며, 없으면 기존 기본값 유지.

## 변경 파일

| 파일 | 변경 내용 |
|---|---|
| `parser/types.go` | `ErrStatus` 주석 수정 |
| `parser/parser.go` | `splitTargetMessage` 3-tuple 반환, `parseTwoQuoted` 3-tuple 반환, `parseGuard`/`parseState`/`parseAuth` ErrStatus 파싱 |
| `parser/parser_test.go` | 파서 테스트 4개 추가 |
| `generator/go_target.go` | `buildTemplateData`에서 empty/exists/state/auth ErrStatus 처리 |
| `generator/go_templates.go` | empty/exists/state/auth 템플릿에 `{{.ErrStatus}}` 적용 |
| `generator/generator_test.go` | 코드젠 테스트 추가 (ErrStatus가 생성 코드에 반영되는지) |

## 상세 변경

### 1. `parser/types.go` — 주석

```
ErrStatus    int    // 에러 HTTP 상태 코드 (0이면 타입별 기본값: @call→500, @empty→404, @exists→409, @state→409, @auth→403)
```

### 2. `parser/parser.go`

#### `splitTargetMessage` → 3-tuple 반환
`(target, msg)` → `(target, msg, remainder)`. `parseGuard`에서만 호출.

#### `parseTwoQuoted` → 3-tuple 반환
`(first, second)` → `(first, second, remainder)`. `parseState`에서만 호출.

#### `parseGuard` — @empty/@exists ErrStatus 파싱
remainder가 숫자이고 100-599 범위면 `seq.ErrStatus`에 저장.

#### `parseState` — remainder에서 ErrStatus 추출
`parseTwoQuoted` 결과의 3번째 반환값에서 ErrStatus 파싱.

#### `parseAuth` — remainder에서 ErrStatus 추출
`extractQuoted` 결과의 remainder에서 ErrStatus 파싱.

### 3. `generator/go_target.go` — `buildTemplateData`

현재 ErrStatus는 `@call`에서만 설정. empty/exists/state/auth도 동일 패턴으로 확장:

```go
// ErrStatus 처리 (empty, exists, state, auth)
switch seq.Type {
case parser.SeqEmpty:
    if seq.ErrStatus != 0 { d.ErrStatus = httpStatusConst(seq.ErrStatus) } else { d.ErrStatus = "http.StatusNotFound" }
case parser.SeqExists:
    if seq.ErrStatus != 0 { d.ErrStatus = httpStatusConst(seq.ErrStatus) } else { d.ErrStatus = "http.StatusConflict" }
case parser.SeqState:
    if seq.ErrStatus != 0 { d.ErrStatus = httpStatusConst(seq.ErrStatus) } else { d.ErrStatus = "http.StatusConflict" }
case parser.SeqAuth:
    if seq.ErrStatus != 0 { d.ErrStatus = httpStatusConst(seq.ErrStatus) } else { d.ErrStatus = "http.StatusForbidden" }
}
```

### 4. `generator/go_templates.go` — 하드코딩 → `{{.ErrStatus}}`

| 템플릿 | 현재 하드코딩 | 변경 |
|---|---|---|
| `empty` | `http.StatusNotFound` | `{{.ErrStatus}}` |
| `exists` | `http.StatusConflict` | `{{.ErrStatus}}` |
| `state` | `http.StatusConflict` | `{{.ErrStatus}}` |
| `auth` | `http.StatusForbidden` | `{{.ErrStatus}}` |

subscribe 템플릿(`sub_empty`, `sub_exists`, `sub_state`, `sub_auth`)은 HTTP 상태 코드를 사용하지 않으므로 변경 없음.

### 5. 테스트

#### `parser/parser_test.go` — 4개 추가
- `TestParseEmptyErrStatus`: `@empty target "msg" 402` → ErrStatus=402
- `TestParseExistsErrStatus`: `@exists target "msg" 422` → ErrStatus=422
- `TestParseStateErrStatus`: `@state ... "msg" 422` → ErrStatus=422
- `TestParseAuthErrStatus`: `@auth ... "msg" 401` → ErrStatus=401

#### `generator/generator_test.go` — 4개 추가
- `TestGenerateEmptyErrStatus`: 402 → `http.StatusPaymentRequired`
- `TestGenerateExistsErrStatus`: 422 → `http.StatusUnprocessableEntity`
- `TestGenerateStateErrStatus`: 422 → `http.StatusUnprocessableEntity`
- `TestGenerateAuthErrStatus`: 401 → `http.StatusUnauthorized`

기존 테스트(ErrStatus==0)는 기본값으로 동작하므로 호환성 유지.

## 의존성

없음. `strconv` import는 parser.go에 이미 존재.

## 검증

```bash
go test ./parser/... ./generator/... -count=1
```
