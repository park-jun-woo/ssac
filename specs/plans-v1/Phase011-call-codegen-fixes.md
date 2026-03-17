✅ 완료

# Phase 011: @sequence call 코드 생성 5건 수정

## 목표

fullend에서 보고된 `@sequence call` 코드젠 컴파일 에러 5건 수정.

## 변경 사항

### 1. import 경로: spec 파일의 import 선언을 사용

`@func auth.x`에서 `auth`를 import 경로로 자동 추가하지 않고, **spec 파일의 Go import 블록을 파싱하여 그대로 전달**한다. `@func`의 패키지명은 import된 패키지의 alias로 간주.

```go
// spec 파일
package service

import (
    "net/http"
    "github.com/park-jun-woo/fullend/pkg/auth"
)

// @sequence call
// @func auth.verifyPassword
// ...
func Login(w http.ResponseWriter, r *http.Request) {}
```

→ 생성 코드에 `import "github.com/park-jun-woo/fullend/pkg/auth"` 포함.

**구현:**
- `parser/types.go`: `ServiceFunc`에 `Imports []string` 필드 추가
- `parser/parser.go`: `ParseFile`에서 Go AST의 import 선언을 수집하여 `ServiceFunc.Imports`에 저장. `"net/http"`는 codegen이 이미 자동 추가하므로 제외.
- `generator/go_target.go`: `collectImports`에서 `@func` 패키지 자동 추가 제거, 대신 `sf.Imports`의 import를 출력에 포함
- `GenerateFunc` 시그니처에서 `sf.Imports` 접근 가능 (이미 `sf parser.ServiceFunc` 전달)

### 2. struct 이름: `*Input` → `*Request`

**변경 파일:**
- `generator/go_templates.go`: `call_func` 템플릿에서 `Input` → `Request`

### 3. 결과 없는 call의 반환값 처리

func는 항상 `(Response, error)` 반환. `@result` 없어도 `_, err` 패턴 사용.

**변경 파일:**
- `generator/go_templates.go`: `call_func` 템플릿에서 `@result` 없을 때 `_, err` 패턴

### 4. 결과 추출 필드명: `@result` 확장 문법

`@result var Type.Field` — Response struct에서 추출할 필드명을 명시.

```go
// @result token Token.AccessToken
// → token := out.AccessToken
```

**변경 파일:**
- `parser/types.go`: `Result` struct에 `Field string` 필드 추가
- `parser/parser.go`: `parseResult` 확장 — `Type`에 `.`이 있으면 분리하여 `Result.Field` 설정
- `generator/go_target.go`: `buildTemplateData`에서 `d.ResultField`를 `seq.Result.Field`에서 가져오도록 변경. Field 비어있으면 기존 동작(ucFirst(var))

### 5. Input struct 필드명: `->` 매핑 지원

call @func의 `@param`에서 `->` 를 사용하여 Request struct 필드명을 명시.

```go
// @param user.ID -> UserID
// → {UserID: user.ID}
```

기존 `Param.Column` 필드를 재활용.

**변경 파일:**
- `generator/go_target.go`: `buildInputFields`에서 `p.Column`이 있으면 `fieldName = p.Column`으로 사용

## 변경 파일 목록

| 파일 | 변경 |
|---|---|
| `parser/types.go` | `ServiceFunc`에 `Imports []string` 추가, `Result`에 `Field string` 추가 |
| `parser/parser.go` | import 파싱 → `sf.Imports`, `parseResult` — `Type.Field` 분리 |
| `generator/go_target.go` | collectImports에서 @func 자동 import 제거 + sf.Imports 병합, buildTemplateData ResultField, buildInputFields Column 매핑 |
| `generator/go_templates.go` | `call_func` 템플릿: Input→Request, _, err 패턴 |
| `parser/parser_test.go` | `parseResult` 확장 테스트, import 파싱 테스트 |
| `generator/generator_test.go` | call_func 코드젠 검증 업데이트 |

## 의존성

- 없음

## 검증

```bash
go test ./parser/... ./validator/... ./generator/... -count=1
```

## 테스트 계획

### parser
- `TestParseResult`: `"token Token.AccessToken"` → Var="token", Type="Token", Field="AccessToken". 기존 `"project Project"` → Field="" (하위 호환)
- `TestParseFileImports`: spec 파일에 import가 있을 때 `sf.Imports`에 포함되는지 검증 (`"net/http"` 제외)

### generator
- `TestGenerateDeleteProject`: `cleanup.ProjectResources` 호출이 `cleanup.ProjectResourcesRequest{...}` 사용 확인
- `TestGenerateCustomMessages`: call+func guard형이 `_, err` 패턴 확인
- 신규 `TestGenerateCallFuncImports`: sf.Imports에 포함된 import가 생성 코드에 나타나는지 검증
- 신규 또는 기존 확장: ResultField `->` 매핑, InputFields `->` 매핑 검증
