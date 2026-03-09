✅ 완료

# Phase 013: @component 폐지 — @func만 허용

## 목표

`@sequence call`에서 `@component`를 완전 제거하고 `@func`만 허용한다.
`@component` 사용 시 validator가 에러를 반환하도록 변경.

## 변경 사항

### 1. parser/types.go
- `Sequence.Component` 필드 삭제

### 2. parser/parser.go
- `case "component":` 분기 삭제 (파싱 시 무시)

### 3. validator/validator.go
- `SeqCall` 검증: `@component` 관련 로직 제거
  - `seq.Component == "" && seq.Func == ""` → `seq.Func == ""` (단순화)
  - `seq.Component != "" && seq.Func != ""` → 삭제
- `validateCallTarget`: `seq.Component` 관련 심볼 검증 삭제

### 4. validator/symbol.go
- `SymbolTable.Components` 맵 삭제
- `loadModelDir()`에서 interface → Components 등록 로직 삭제

### 5. generator/go_target.go
- `templateData`에서 `Component`, `ComponentMethod` 필드 삭제
- `buildTemplateData`에서 `d.Component`, `d.ComponentMethod` 할당 삭제
- `templateName`에서 `seq.Component != ""` → `"call_component"` 분기 삭제
- `defaultMessage`에서 `seq.Component` 관련 분기 삭제

### 6. generator/go_templates.go
- `{{define "call_component"}}` 템플릿 삭제

### 7. testdata/backend-service/delete_project.go
- `@sequence call` + `@component notification` 시퀀스 삭제

### 8. specs/dummy-study/service/
- `cancel_reservation.go`: `@component notification` 시퀀스 삭제
- `create_reservation.go`: `@component notification` 시퀀스 삭제

### 9. specs/dummy-study/model/notification.go
- 파일 삭제

### 10. 테스트 업데이트
- `parser/parser_test.go`: component 관련 assertion 제거/수정
- `validator/validator_test.go`: component 관련 테스트 케이스 삭제/수정

## 변경 파일 목록

| 파일 | 변경 |
|---|---|
| `parser/types.go` | `Component` 필드 삭제 |
| `parser/parser.go` | `case "component":` 삭제 |
| `validator/validator.go` | call 검증 단순화, `validateCallTarget` component 로직 삭제 |
| `validator/symbol.go` | `Components` 맵 + interface 등록 삭제 |
| `generator/go_target.go` | `Component`/`ComponentMethod` 필드·할당·분기 삭제 |
| `generator/go_templates.go` | `call_component` 템플릿 삭제 |
| `testdata/backend-service/delete_project.go` | `@component` 시퀀스 삭제 |
| `specs/dummy-study/service/cancel_reservation.go` | `@component` 시퀀스 삭제 |
| `specs/dummy-study/service/create_reservation.go` | `@component` 시퀀스 삭제 |
| `specs/dummy-study/model/notification.go` | 파일 삭제 |
| `parser/parser_test.go` | component assertion 제거 |
| `validator/validator_test.go` | component 테스트 케이스 제거/수정 |

## 의존성

- 없음

## 검증

```bash
go test ./parser/... ./validator/... ./generator/... -count=1
```
