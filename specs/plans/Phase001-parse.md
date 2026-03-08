# Phase 1: Parse

> `specs/backend/service/*.go`의 sequence 주석을 파싱하여 Go 구조체 리스트로 변환한다.

## 목표

주석 DSL을 읽어 `[]Sequence` 슬라이스를 산출한다. 이후 Phase 2(codegen)의 입력이 된다.

## 입력

```
specs/backend/service/*.go
```

각 파일은 하나의 서비스 함수를 포함하며, 함수 위에 sequence 주석 블록이 나열된다.

### 주석 예시

```go
// @sequence get
// @model Project.FindByID
// @param ProjectID request
// @result project Project

// @sequence guard nil project
// @message "프로젝트가 존재하지 않습니다"

// @sequence response json
// @var session
func CreateSession(w http.ResponseWriter, r *http.Request) {}
```

## 산출

`[]ServiceFunc` — 파일 단위 파싱 결과

```go
type ServiceFunc struct {
    Name       string     // 함수명 (e.g. "CreateSession")
    FileName   string     // 원본 파일명 (e.g. "create_session.go")
    Sequences  []Sequence // 순서 보존된 sequence 리스트
}
```

`Sequence` — 개별 sequence 블록

```go
type Sequence struct {
    Type      string            // authorize, get, guard nil, guard exists, post, put, delete, password, call, response
    Model     string            // @model 값 (e.g. "Project.FindByID")
    Params    []Param           // @param 리스트
    Result    *Result           // @result (없을 수 있음)
    Message   string            // @message (없으면 빈 문자열)
    Vars      []string          // @var 리스트 (response용)
    // authorize 전용
    Action    string            // @action
    Resource  string            // @resource
    ID        string            // @id
    // call 전용
    Component string            // @component
    Func      string            // @func
}

type Param struct {
    Name   string // 파라미터명 (e.g. "ProjectID")
    Source string // 소스 (e.g. "request", 변수명, 리터럴)
}

type Result struct {
    Var  string // 변수명 (e.g. "project")
    Type string // 타입명 (e.g. "Project")
}
```

## 구현 계획

### 1. 프로젝트 초기화

- `go mod init`
- 디렉토리 구조:
  ```
  artifacts/
    cmd/ssac/main.go        # CLI 진입점
    internal/
      parser/
        parser.go           # 파싱 로직
        types.go            # Sequence, Param, Result 등 구조체 정의
      parser_test.go        # 테스트
  ```

### 2. 구조체 정의 (`types.go`)

- `ServiceFunc`, `Sequence`, `Param`, `Result` 정의
- sequence 타입 상수 (10종)

### 3. 파서 구현 (`parser.go`)

- `go/parser`로 Go 파일 AST 파싱
- `go/ast`에서 함수 선언의 `Doc` 코멘트 그룹 추출
- 주석 라인을 순회하며 `@` 태그 파싱:
  1. `@sequence <type>` → 새 Sequence 블록 시작
  2. 이후 태그(`@model`, `@param`, `@result`, `@message`, `@var`, `@action`, `@resource`, `@id`, `@component`, `@func`)를 현재 블록에 누적
  3. 다음 `@sequence` 또는 함수 선언을 만나면 블록 종료

### 4. 엣지 케이스

- `guard nil`, `guard exists`처럼 type이 두 단어인 경우 처리
- `@message`의 따옴표 제거
- `@param`의 리터럴(따옴표 감싸진 값) 처리
- 빈 주석 라인 무시
- 주석 블록 사이의 빈 줄 허용

### 5. 테스트

- 기획서의 두 예시(CreateSession, DeleteProject)를 `specs/backend/service/`에 배치
- 파싱 결과를 기대값과 비교

## 완료 기준

- [x] `specs/backend/service/*.go` 파일을 읽어 `[]ServiceFunc`를 반환
- [x] 10종 sequence 타입 모두 파싱 가능
- [x] 기획서 예시 2개(CreateSession, DeleteProject) 파싱 테스트 통과
