# Phase 010: password 폐지 + @func 패키지 분리 코드젠 ✅ 완료

## 목표

1. `@sequence password` 폐지 — bcrypt 하드코딩 제거, `@sequence call`로 통합. 시퀀스 타입 11→10.
2. `@func` 패키지 분리 코드젠 — `@func auth.hashPassword` → `h.Auth.HashPassword(auth.HashPasswordInput{...})` 호출 코드 생성.

## 변경 파일

### parser/types.go
- `SeqPassword` 상수 제거
- `ValidSequenceTypes`에서 `SeqPassword` 항목 제거 (11→10)
- `Sequence` struct에 `Package string` 필드 추가
- `Type` 주석에서 password 제거

### parser/parser.go
- `@func` 파싱: `.`으로 split → `seq.Package = parts[0]`, `seq.Func = parts[1]`
- password 관련 분기 없음 (기존에도 parser는 password 전용 분기 없음)

### validator/validator.go
- `case parser.SeqPassword:` 분기 제거 (validateRequiredFields)
- `case parser.SeqCall:` — `@func`일 때 `seq.Package`가 비어있으면 ERROR (`.` 필수)

### generator/go_templates.go
- `{{define "password"}}` 템플릿 제거
- `{{define "call_func"}}` 템플릿 수정:
  ```
  h.{{.PkgField}}.{{.FuncMethod}}({{.PkgName}}.{{.FuncMethod}}Input{ {{.InputFields}} })
  ```
- `@result` 없는 call_func: guard형 (err → StatusUnauthorized 또는 커스텀 status)

### generator/go_target.go
- `templateData` 변경:
  - `Hash`, `Plain` 필드 제거
  - `PkgName`, `PkgField`, `FuncMethod`, `InputFields` 필드 추가
- `buildTemplateData`:
  - `SeqPassword` 분기 제거
  - `SeqCall` + `seq.Package != ""` 분기 추가 → PkgName/PkgField/FuncMethod/InputFields 구성
- `collectImports`:
  - `SeqPassword` → bcrypt import 제거
  - `SeqCall` + `seq.Package != ""` → `"<module>/internal/<package>"` import 추가 (module path: go.mod에서 읽거나 상수)
- `defaultMessage`: `SeqPassword` 분기 제거
- `err 선언 추적`: `SeqPassword` 분기 제거

### generator/go_templates.go — call_func 템플릿 변경
- result 있음: `out, err := h.Pkg.Func(pkg.FuncInput{...})` + `var := out.Var`
- result 없음: `_, err = h.Pkg.Func(pkg.FuncInput{...})` + err 시 커스텀 status

### specs/dummy-study/service/login.go
- `@sequence password` → `@sequence call` + `@func auth.verifyPassword`
- `@message` 추가

### 테스트 변경

#### parser/parser_test.go
- `TestParsePasswordAndPut` → `@sequence password` → `@sequence call` + `@func auth.verifyPassword`로 변환
- `TestParseSequenceType`: `{"password", "password"}` 항목 제거
- 신규: `TestParseFuncPackage` — `@func auth.hashPassword` → Package="auth", Func="hashPassword"

#### validator/validator_test.go
- `TestValidatePasswordMissingParams` → 제거 또는 call+func 검증으로 변환
- 신규: `TestValidateCallFuncMissingPackage` — `@func hashPassword` (`.`없음) → ERROR

#### generator/generator_test.go
- `TestGenerateCustomMessages`: password 테스트 케이스 → call+func 패턴으로 변환
- `TestGenerateDeleteProject`: `cleanupProjectResources` 관련 검증 수정 (패키지 분리 적용 여부 확인)
- 신규: `TestGenerateCallFunc` — `@func auth.hashPassword` → `h.Auth.HashPassword(auth.HashPasswordInput{...})` 검증

#### testdata/backend-service/delete_project.go
- `@func cleanupProjectResources` → `@func cleanup.projectResources` (패키지 분리)

### 문서 업데이트 (CLAUDE.md, README.md, manual-for-ai.md, manual-for-human.md)
- 시퀀스 타입 11→10
- password 타입 제거
- @func 문법 변경: `<package>.<funcName>` 필수

## 의존성
- 없음

## 주요 설계 결정

### Input/Output struct 필드 매핑
- `@param Password request` → `Password: password`
- `@param user.PasswordHash` → `PasswordHash: user.PasswordHash`
- 필드명은 @param의 Name (dot notation이면 `.` 뒤 부분)

### Handler struct 미생성
- 수정지시서에서 Handler struct 언급하지만, 현재 SSaC 코드젠은 Handler struct를 생성하지 않음 (standalone func 패턴)
- 따라서 `h.Auth.HashPassword(...)` 대신 `auth.HashPassword(...)` 직접 호출로 구현
- import만 `"github.com/park-jun-woo/ssac/internal/auth"` 추가 (또는 module path 기반)

### module path
- `go.mod`의 module path (`github.com/park-jun-woo/ssac`) 활용은 코드젠 시점에서 복잡
- `@func auth.verifyPassword` → import `"<configurable>/auth"` 또는 상대경로
- 현재 코드젠은 import path를 하드코딩하지 않으므로, `@func` 패키지의 import path 결정 방식 확인 필요
- **결정**: 생성 코드의 import에 패키지명만 추가. 실제 import path는 프로젝트 구조에 따라 fullend가 관리.

### @func 없이 @component만 있는 call은 변경 없음
- `@component`는 기존 패턴 유지

## 검증

```bash
go test ./parser/... ./validator/... ./generator/... -count=1
```

## 리스크

- `testdata/backend-service/delete_project.go`의 `@func cleanupProjectResources`가 `.` 없이 사용 중
  → `@func cleanup.projectResources` 등으로 변경 필요
  → 이에 의존하는 `TestGenerateDeleteProject` 수정 필요
