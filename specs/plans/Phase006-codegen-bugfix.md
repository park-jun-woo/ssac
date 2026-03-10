# Phase 006: 코드젠 버그 수정 + 예약 소스 공식화 ✅ 완료

## 목표

1. 생성 코드의 컴파일 에러 2건 수정
2. 예약 소스(reserved sources) 공식화

## 버그 1: 같은 변수 `:=` 중복 선언

### 현상

같은 result 변수가 두 번 이상 선언될 때, 두 번째부터도 `:=`를 사용하여 Go 컴파일 에러 발생.

```go
// 첫 번째 @get (OK)
reservation, err := reservationModel.FindByID(reservationID)

// 두 번째 @get (BUG: := → = 이어야 함)
reservation, err := reservationModel.FindByID(reservationID)  // compile error
```

### 원인

`go_target.go:buildTemplateData()` line 250-254에서 `get/post`는 무조건 `FirstErr = true`로 설정.
`get` 템플릿은 `{{.Result.Var}}, {{if .HasTotal}}total, {{end}}err := ...`로 항상 `:=` 사용.

### 수정 방안

1. result 변수 재선언 추적: `declaredVars map[string]bool`을 시퀀스 루프에서 관리
2. 이미 선언된 result 변수가 나오면 `=` 사용
3. `templateData`에 `ReAssign bool` 필드 추가
4. `get`/`post` 템플릿에서 `ReAssign`이면 `=`, 아니면 `:=` 사용

변경 파일:
- `generator/go_target.go` — `declaredVars` 추적, `ReAssign` 설정
- `generator/go_templates.go` — `get`/`post` 템플릿에 `ReAssign` 분기 추가

### 템플릿 변경

```
{{- define "get" -}}
	{{.Result.Var}}, {{if .HasTotal}}total, {{end}}err {{if .ReAssign}}={{else}}:={{end}} {{.ModelCall}}({{.ArgsCode}})
```

## 버그 2: @auth/@state inputs에서 `request.` 미변환

### 현상

`@auth` 또는 `@state`의 inputs 값에 `request.Field`가 있을 때, 생성 코드에서 `request.Field`를 그대로 출력. `request`라는 변수는 존재하지 않음.

```go
// BUG: request는 변수가 아님
authz.Input{Id: request.RoomID}

// FIX: 추출된 로컬 변수 사용
authz.Input{Id: roomID}
```

### 원인

`go_target.go:buildInputFieldsFromMap()` line 316-328에서 inputs 값을 그대로 출력.
`request.Field` → `lcFirst(Field)`로 변환해야 하지만 변환 로직이 없음.

### 수정 방안

`buildInputFieldsFromMap()`에서 예약 소스별 변환 규칙 적용:
- `request.Field` → `lcFirst(Field)` (로컬 변수로 치환)
- `currentUser.Field` → `currentUser.Field` (실제 변수, 변경 없음)
- `config.Field` → `config.Field` (실제 변수, 변경 없음)
- 일반 변수 (`reservation.Status`) → 그대로 유지

즉, `argToCode()`와 동일한 변환 로직을 inputs 값에도 적용.
현재 `argToCode()`는 Args에만 적용되고 Inputs에는 미적용된 것이 버그의 근본 원인.

변경 파일:
- `generator/go_target.go` — `buildInputFieldsFromMap()` 수정, `inputValueToCode()` 헬퍼 추출

### 변환 규칙

| inputs 값 | 변환 결과 | 이유 |
|---|---|---|
| `request.RoomID` | `roomID` | 예약 소스 → 로컬 변수 치환 |
| `request.ReservationID` | `reservationID` | 예약 소스 → 로컬 변수 치환 |
| `currentUser.ID` | `currentUser.ID` | 예약 소스, 실제 변수로 유지 |
| `config.APIKey` | `config.APIKey` | 예약 소스, 실제 변수로 유지 |
| `reservation.Status` | `reservation.Status` | 일반 변수, 변경 없음 |

## 예약 소스 공식화

DSL의 Args/Inputs에서 사용하는 `source.Field` 중 다음 3개는 사용자가 선언하지 않는 예약 소스다.
코드젠에서 각각 다른 메커니즘으로 변환된다.

| 예약 소스 | 역할 | 코드젠 변환 |
|---|---|---|
| `request` | HTTP 요청 파라미터 | `c.ShouldBindJSON()` / `c.Query()` / `c.Param()` → 로컬 변수 추출 |
| `currentUser` | 인증 컨텍스트 | `c.MustGet("currentUser").(*model.CurrentUser)` → `currentUser.Field` 접근 |
| `config` | 환경 설정 주입 | `config.Field` 접근 (DI 또는 전역 변수) |

특징:
- validator에서 변수 흐름 검증 시 선언 없이 참조 가능 (미리 declared 처리)
- generator에서 `request.Field`는 로컬 변수(`lcFirst(Field)`)로 치환. `currentUser`/`config`은 실제 변수로 유지
- `@auth`/`@state`의 `{inputs}`에서도 동일 규칙 적용 (버그 2의 근본 원인)

### 예약 소스 충돌 검증 (ERROR)

result 변수명으로 예약 소스 이름을 사용하면 ERROR:

```go
// ERROR: "request"는 예약 소스이므로 result 변수명으로 사용할 수 없습니다
// @get User request = User.FindByID(request.Email)

// ERROR: "currentUser"는 예약 소스이므로 result 변수명으로 사용할 수 없습니다
// @get User currentUser = User.FindByID(request.UserID)
```

### 예약 소스 런타임 의존성 경고 (WARNING)

`currentUser`를 사용하는데 `model/`에 `CurrentUser` 타입이 없으면 WARNING:

```
WARNING: login.go:Login — currentUser를 사용하지만 model/에 CurrentUser 타입이 정의되지 않았습니다
```

- 심볼 테이블 기반 교차 검증 (`ValidateWithSymbols`)에서만 동작
- `model/*.go`에 `type CurrentUser struct { ... }` 존재 여부 확인
- 없으면 WARNING (Go 컴파일 시 에러가 발생할 것을 사전 안내)

변경 파일:
- `validator/validator.go` — `validateReservedSourceConflict()` ERROR + `validateCurrentUserType()` WARNING
- `validator/validator_test.go` — 예약 소스 충돌 테스트 + CurrentUser 미정의 WARNING 테스트
- `CLAUDE.md` — DSL 문법 섹션에 "예약 소스" 명시
- `artifacts/manual-for-ai.md` — Args 형식에 "reserved sources" 추가
- `artifacts/manual-for-human.md` — 인자 표기법에 "예약 소스" 추가

## 변경 파일

| 파일 | 내용 |
|---|---|
| `generator/go_target.go` | declaredVars 추적, ReAssign 설정, buildInputFieldsFromMap request 변환 |
| `generator/go_templates.go` | get/post 템플릿 ReAssign 분기 |
| `generator/generator_test.go` | 재선언 테스트, @auth inputs 변환 테스트 |
| `validator/validator.go` | 예약 소스 충돌 검증 추가 |
| `validator/validator_test.go` | 예약 소스 충돌 테스트 |
| `CLAUDE.md` | 예약 소스 공식화 |
| `artifacts/manual-for-ai.md` | 예약 소스 공식화 |
| `artifacts/manual-for-human.md` | 예약 소스 공식화 |

## 검증

```bash
go test ./generator/... ./validator/... -count=1
ssac gen specs/dummy-study /tmp/dummy-study-out
# 생성 코드에서:
#   1. 두 번째 @get에서 `=` 사용 확인
#   2. authz.Input{Id: roomID} 확인 (request.RoomID 아님)
#   3. @get User request = ... → validator ERROR 확인
```

## 의존성

- Phase 003 (generator)
