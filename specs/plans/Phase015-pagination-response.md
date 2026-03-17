# Phase 015: pagination 제네릭 타입 + @response 간단쓰기 + parseInputs 검증

## 목표

1. `parseInputs()`에서 콜론 없는 입력(`{query}`)을 파싱 에러로 처리
2. `Page[T]`, `Cursor[T]` 제네릭 타입 파싱/검증/코드젠 지원
3. `@response 변수명` 간단쓰기 지원 (struct 직접 반환)

## 구현 단계

### Step 1: parseInputs 검증

**변경 파일**: `parser/parser.go`, `parser/parser_test.go`

- `parseInputs()`에서 `colonIdx < 0`일 때 `continue` 대신 에러 반환
- 에러 메시지: `"{pair}"는 유효하지 않은 입력 형식입니다. "{Key: value}" 형식을 사용하세요.`
- `parseInputs()` 반환 타입을 `(map[string]string, error)`로 변경, 호출부에서 에러 전파
- 테스트: 콜론 없는 입력 시 에러 확인

### Step 2: Page[T] / Cursor[T] 타입 파싱

**변경 파일**: `parser/parser.go`, `parser/types.go`, `parser/parser_test.go`

- `parseResultType()`에서 `Page[Gig]`, `Cursor[Gig]` 형식 인식
- `Result` 구조체에 `Wrapper string` 필드 추가 (`"Page"`, `"Cursor"`, `""`)
- `Result.Type`은 내부 타입 저장 (`"Gig"`)
- 기존 `[]Type` 슬라이스 표기도 계속 지원
- 테스트: `Page[Gig]` → `Result{Type: "Gig", Wrapper: "Page"}`, `Cursor[Gig]` → `Result{Type: "Gig", Wrapper: "Cursor"}`

### Step 3: @response 간단쓰기

**변경 파일**: `parser/parser.go`, `parser/parser_test.go`

- `@response` 다음 토큰이 `{`이면 기존 필드 매핑 파싱
- `@response` 다음 토큰이 변수명이면 `seq.Target = 변수명` 설정 (Fields 비움)
- 테스트: `@response gigPage` → `Sequence{Type: SeqResponse, Target: "gigPage"}`

### Step 4: validator — x-pagination 교차검증

**변경 파일**: `validator/validator.go`, `validator/validator_test.go`

- `x-pagination: style: offset` → `Result.Wrapper == "Page"` 필수 (아니면 ERROR)
- `x-pagination: style: cursor` → `Result.Wrapper == "Cursor"` 필수 (아니면 ERROR)
- `x-pagination` 없음 → `Result.Wrapper`가 있으면 ERROR (Page[T]/Cursor[T] 사용 불가, `[]T` 사용해야 함)
- 외부 검증(심볼 테이블 있을 때)만 적용. 내부 검증 시에는 x-pagination 정보 없으므로 스킵
- 테스트: 불일치/일치 케이스

### Step 5: generator — Page[T]/Cursor[T] 코드젠 + @response 간단쓰기

**변경 파일**: `generator/go_target.go`, `generator/go_templates.go`, `generator/generator_test.go`

- `Result.Wrapper`가 `"Page"` 또는 `"Cursor"`이면:
  - model interface 반환 타입: `(*pagination.Page[Gig], error)` / `(*pagination.Cursor[Gig], error)`
  - import `"github.com/park-jun-woo/fullend/pkg/pagination"` 추가
  - 기존 3-tuple(`result, total, err`) 대신 단일 반환(`gigPage, err`) — `HasTotal = false`
  - `[]T` + QueryOpts → 기존 3-tuple 유지 (하위 호환)
- `@response 변수명` (간단쓰기):
  - `seq.Target`이 설정되어 있으면 `c.JSON(http.StatusOK, gigPage)` (gin.H 없이)
  - `seq.Fields`가 있으면 기존 풀어쓰기 동작 유지
- `deriveReturnType()`: `Result.Wrapper` 분기 추가 → `(*pagination.Page[T], error)` 등

## 변경 파일 목록

| 파일 | 변경 |
|---|---|
| `parser/parser.go` | parseInputs 에러, 제네릭 타입 파싱, @response 간단쓰기 |
| `parser/types.go` | Result.Wrapper 필드 |
| `parser/parser_test.go` | 3가지 기능 테스트 |
| `validator/validator.go` | x-pagination 교차검증 |
| `validator/validator_test.go` | 교차검증 테스트 |
| `generator/go_target.go` | Page/Cursor 반환 타입, response 간단쓰기 |
| `generator/go_templates.go` | response_direct 템플릿 |
| `generator/generator_test.go` | 코드젠 테스트 |

## 의존성

- `github.com/park-jun-woo/fullend/pkg/pagination` — 런타임 의존 (생성 코드에서 import)
- SSaC 자체에는 새 외부 의존성 없음

## 검증 방법

```bash
go test ./parser/... ./validator/... ./generator/... -count=1
```

## 상태: 대기
