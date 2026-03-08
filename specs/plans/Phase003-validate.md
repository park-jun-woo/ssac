# Phase 3: Validate

> 파싱된 sequence가 OpenAPI, DDL 등 외부 SSOT와 정합한지 교차 검증한다.

## 목표

Phase 1의 산출물(`[]ServiceFunc`)이 참조하는 모델, 파라미터, 타입이 실제 SSOT(sqlc, OpenAPI, Go interface)에 존재하는지 심볼릭 검증한다. 코드 생성(Phase 2) 전에 실행하여 불일치를 조기 발견한다.

## 입력

1. Phase 1의 `[]ServiceFunc`
2. 심볼 테이블 소스:
   - sqlc 산출물 — DB 모델 struct, 쿼리 메서드
   - OpenAPI 산출물 — 요청/응답 struct, 필드
   - Go interface — 비-DB 모델 계약

## 산출

- 검증 성공: exit 0, 코드젠 진행 가능
- 검증 실패: 에러 목록 출력, exit 1

## 검증 항목

### 1. 모델 존재 검증

`@model Project.FindByID` → 심볼 테이블에 `Project` 타입과 `FindByID` 메서드가 존재하는가?

```
ERROR: create_session.go:3 — @model Project.FindByID: "Project" 타입을 찾을 수 없습니다
ERROR: create_session.go:3 — @model Project.FindByID: "FindByID" 메서드를 찾을 수 없습니다
```

### 2. 파라미터 타입 검증

`@param ProjectID request` → 해당 엔드포인트의 request에 `ProjectID` 필드가 존재하는가?

```
ERROR: create_session.go:4 — @param ProjectID request: OpenAPI request에 "ProjectID" 필드가 없습니다
```

### 3. 결과 타입 검증

`@result project Project` → `Project` 타입이 심볼 테이블에 존재하고, 해당 메서드의 반환 타입과 호환되는가?

```
ERROR: create_session.go:5 — @result project Project: 메서드 FindByID의 반환 타입은 *Project이 아닙니다
```

### 4. 응답 스키마 검증

`@sequence response json` + `@var session` → OpenAPI response schema에 `session` 필드가 존재하는가?

```
ERROR: create_session.go:12 — @var session: OpenAPI response에 "session" 필드가 없습니다
```

### 5. 변수 흐름 검증 (sequence 내부)

`@result`로 선언된 변수가 이후 sequence에서 참조될 때, 선언 순서가 올바른가?

```
ERROR: create_session.go:7 — guard nil project: "project" 변수가 아직 선언되지 않았습니다
```

### 6. authorize 필드 검증

`@action`, `@resource`, `@id`가 모두 존재하는가?

```
ERROR: delete_project.go:1 — authorize: @action이 누락되었습니다
```

## 심볼 테이블 구성

### sqlc 산출물에서 수집

```go
// sqlc가 생성한 querier interface를 파싱
type SymbolTable struct {
    Models  map[string]ModelSymbol  // "Project" → {Fields, Methods}
    Queries map[string]QuerySymbol  // "Project.FindByID" → {Params, Returns}
}
```

소스: `sqlc.yaml` 설정 → 산출 디렉토리의 `*.go` 파일을 `go/ast`로 파싱

### OpenAPI 산출물에서 수집

```go
type APISymbol struct {
    RequestFields  map[string]string  // "ProjectID" → "string"
    ResponseFields map[string]string  // "session" → "Session"
}
```

소스: OpenAPI spec(yaml/json)을 파싱하거나, openapi-generator 산출물의 Go struct를 `go/ast`로 파싱

### Go interface에서 수집

소스: `specs/model/*.go`의 interface 선언을 `go/ast`로 파싱

## 구현 계획

### 1. 디렉토리 구조

```
artifacts/
  internal/
    validator/
      validator.go      # 검증 메인 로직
      symbol.go          # 심볼 테이블 구성
      errors.go          # 에러 타입 정의
    validator_test.go    # 테스트
```

### 2. 심볼 테이블 로더

- sqlc 산출물 로더: `go/ast`로 struct, interface 파싱
- OpenAPI 로더: spec 파일 파싱 (yaml/json)
- Go interface 로더: `go/ast`로 interface 파싱

### 3. 검증 엔진

```
심볼 테이블 구성
  → ServiceFunc 순회
    → Sequence 순회
      → 타입별 검증 규칙 적용
        → 에러 수집
  → 에러 리포트 출력
```

### 4. CLI 연동

- `ssac validate` 명령: parse → validate 파이프라인
- `ssac gen`에서도 validate를 먼저 실행 (검증 실패 시 코드 생성 중단)

### 5. 테스트

- 정상 케이스: 기획서 예시가 검증 통과
- 실패 케이스: 존재하지 않는 모델, 누락된 파라미터, 미선언 변수 등

## 완료 기준

- [ ] sqlc, OpenAPI, Go interface에서 심볼 테이블 구성
- [ ] 6가지 검증 항목 모두 구현
- [ ] `ssac validate` 명령으로 검증 실행
- [ ] 에러 메시지에 파일명, 줄번호, 구체적 원인 포함
- [ ] `ssac gen`이 validate 실패 시 코드 생성 중단
