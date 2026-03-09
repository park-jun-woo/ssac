# SSaC Manual

## 개요

SSaC(Service Sequences as Code)는 Go 주석 기반 선언적 서비스 로직을 파싱하여 Go+gin 구현 코드를 생성하는 CLI 도구다.

```
specs/service/**/*.go  →  ssac parse  →  ssac validate  →  ssac gen  →  artifacts/service/**/*.go
      (주석 DSL)            (구조체)        (정합성 검증)      (Go 코드)      (gofmt 완료)
```

## 설치 & 실행

```bash
# 프로젝트 루트에서
go run ./cmd/ssac <command>

# 또는 빌드 후
go build -o ssac ./cmd/ssac
./ssac <command>
```

## CLI 명령

### parse

주석을 파싱하여 sequence 구조를 출력한다.

```bash
ssac parse                          # 기본: specs/backend/service/
ssac parse specs/dummy-study/service  # 경로 지정
```

출력 예시:
```
=== CreateSession (create_session.go) ===
  [0] get | model=Project.FindByID | result=project Project
  [1] guard nil | message="프로젝트가 존재하지 않습니다"
  [2] post | model=Session.Create | result=session Session
  [3] response json
```

### validate

내부 정합성 + 외부 SSOT 교차 검증을 수행한다.

```bash
ssac validate specs/backend/service   # 내부 검증만 (외부 SSOT 없음)
ssac validate specs/dummy-study       # 외부 SSOT 교차 검증 (자동 감지)
```

프로젝트 루트에 `db/queries/`, `api/openapi.yaml`, `model/*.go`가 있으면 자동으로 심볼 테이블을 구성하여 교차 검증한다.

검증 항목:

| 구분 | 항목 |
|---|---|
| 내부 | 타입별 필수 태그 누락 |
| 내부 | @model "Model.Method" 형식 |
| 내부 | 변수 흐름 (선언 전 참조) |
| 외부 | 모델/메서드 존재 (sqlc queries, Go interface) |
| 외부 | request 필드 존재 (OpenAPI) |
| 외부 | response 필드 존재 (OpenAPI) |

### gen

validate → 코드 생성 → gofmt. 검증 실패 시 코드 생성을 중단한다.
프로젝트 루트에 외부 SSOT가 있으면 타입 변환 코드와 모델 인터페이스도 함께 생성한다.

```bash
ssac gen <service-dir> <out-dir>
ssac gen specs/dummy-study artifacts/test/dummy-out
```

---

## 주석 DSL 문법

### 기본 구조

```go
// @sequence <type> [subtype] [target]
// @tag value
// ...
func FuncName(w http.ResponseWriter, r *http.Request) {}
```

- 하나의 `.go` 파일에 하나의 함수
- 함수 위에 sequence 블록을 나열 (빈 줄로 구분 가능)
- `@sequence`가 블록의 시작
- spec 파일의 Go import 선언은 생성 코드에 전달됨 (`"net/http"` 제외)

### 도메인 폴더 구조

서비스 파일을 도메인별 폴더로 분류할 수 있다. `ParseDir`이 재귀 탐색하여 첫 번째 서브디렉토리를 도메인으로 인식한다.

```
specs/service/
├── login.go                  ← flat, Domain="", package service
├── auth/
│   └── register.go           ← Domain="auth", package auth
└── course/
    └── create_course.go      ← Domain="course", package course
```

생성 결과:
```
artifacts/service/
├── login.go                  ← package service
├── auth/
│   └── register.go           ← package auth
└── course/
    └── create_course.go      ← package course
```

기존 flat 구조(`service/*.go`)는 변경 없이 동작한다.

### sequence 타입 (10종)

#### authorize — 권한 검증

```go
// @sequence authorize
// @action <action>        // 필수: create, read, update, delete, cancel 등
// @resource <resource>    // 필수: 리소스명
// @id <ParamName>         // 필수: 식별자 파라미터
// @message "커스텀 메시지"  // 선택: Forbidden 메시지 (기본: "권한이 없습니다"). 내부 에러는 "권한 확인 실패" 고정.
```

코드젠 결과:
```go
currentUser := c.MustGet("currentUser").(*model.CurrentUser)
allowed, err := authz.Check(currentUser, "create", "course", nil)
```

#### get — 리소스 조회

```go
// @sequence get
// @model <Model.Method>   // 필수: e.g. "User.FindByEmail"
// @param <Name> <source>  // 0개 이상
// @result <var> <Type>    // 필수: 결과 바인딩
```

#### guard nil — null이면 종료

```go
// @sequence guard nil <target>   // target: 검사할 변수명
// @message "커스텀 메시지"         // 선택 (기본: "<target>가 존재하지 않습니다")
```

#### guard exists — 존재하면 종료

```go
// @sequence guard exists <target>
// @message "커스텀 메시지"          // 선택 (기본: "<target>가 이미 존재합니다")
```

#### guard state — 상태 전이 검사

```go
// @sequence guard state <stateDiagramID>   // stateDiagramID: Mermaid stateDiagram 파일 식별자
// @param <entity>.<StatusField>            // 필수: 상태 필드 (dot notation)
// @message "커스텀 메시지"                   // 선택 (기본: "상태 전이가 허용되지 않습니다")
```

함수명이 전이 이벤트로 사용된다. 코드젠 결과:
```go
if !coursestate.CanTransition(course.Published, "PublishCourse") {
    c.JSON(http.StatusConflict, gin.H{"error": "상태 전이가 허용되지 않습니다"})
    return
}
```

import: `"states/{stateDiagramID}state"` (상태 머신 패키지는 fullend가 생성)

#### post — 리소스 생성

```go
// @sequence post
// @model <Model.Method>
// @param <Name> <source>
// @result <var> <Type>    // 필수
```

#### put — 리소스 수정

```go
// @sequence put
// @model <Model.Method>   // 필수
// @param <Name> <source>
```

#### delete — 리소스 삭제

```go
// @sequence delete
// @model <Model.Method>   // 필수
// @param <Name> <source>
```

#### call — 외부 함수 호출

```go
// @sequence call
// @func <package>.<funcName>   // 필수. e.g. auth.verifyPassword
// @param <args> [-> FieldName] // -> 로 Request struct 필드명 명시 가능
// @result <var> <Type[.Field]> // 선택: 없으면 guard형 (401), 있으면 value형 (500). Type.Field로 Response 필드 지정
// @message "커스텀 메시지"       // 선택
```

spec 파일에 import를 명시해야 한다:
```go
package service

import (
    "net/http"
    "github.com/geul-org/fullend/pkg/auth"
)

// @sequence call
// @func auth.verifyPassword
// @param user.PasswordHash
// @param Password request
// @message "비밀번호가 일치하지 않습니다"
```

코드젠 결과:
```go
// guard형 (@result 없음)
_, err = auth.VerifyPassword(auth.VerifyPasswordRequest{PasswordHash: user.PasswordHash, Password: password})
if err != nil {
    c.JSON(http.StatusUnauthorized, gin.H{"error": "비밀번호가 일치하지 않습니다"})
    return
}

// value형 (@result 있음, Type.Field로 필드 지정)
// @result token Token.AccessToken
out, err := auth.IssueToken(auth.IssueTokenRequest{UserID: user.ID})
if err != nil {
    c.JSON(http.StatusInternalServerError, gin.H{"error": "issueToken 호출 실패"})
    return
}
token := out.AccessToken
```

#### response — 응답 반환

```go
// @sequence response json
// @var <varName>           // 0개 이상: 반환할 변수
```

### @param source 규칙

| source | 의미 | 코드젠 결과 |
|---|---|---|
| `request` | HTTP 요청 파라미터 | `c.Query("Name")` (DDL 타입에 따라 변환 코드 추가) |
| `currentUser` | 인증 컨텍스트 (예약어) | `currentUser.Name` (자동으로 `c.MustGet` 추출) |
| `config` | 환경 설정 (예약어) | `config.Name` |
| (없음) | 변수 참조 | 변수 그대로 |
| dot notation | 필드 참조 | `user.Email` 그대로 |
| `"리터럴"` | 문자열 리터럴 | `"리터럴"` 그대로 |

#### `-> column` 명시적 매핑

DDL 컬럼 매핑:
```go
// @param PaymentMethod request -> method
```

@func Request struct 필드 매핑:
```go
// @param user.ID -> UserID
// → auth.IssueTokenRequest{UserID: user.ID}
```

### @message 기본값

`@message`를 생략하면 타입과 모델명으로 자동 생성된다:

| 타입 + 모델 | 기본 메시지 |
|---|---|
| get + Project.FindByID | "Project 조회 실패" |
| post + Session.Create | "Session 생성 실패" |
| put + Room.Update | "Room 수정 실패" |
| delete + Room.Delete | "Room 삭제 실패" |
| guard nil (project) | "project가 존재하지 않습니다" |
| guard exists (conflict) | "conflict가 이미 존재합니다" |
| guard state | "상태 전이가 허용되지 않습니다" |
| authorize | "권한이 없습니다" (Forbidden), "권한 확인 실패" (내부 에러, 고정) |
| call @func auth.verify | "verify 호출 실패" |

---

## 프로젝트 구조

외부 검증을 사용하려면 다음 디렉토리 구조를 따른다:

```
<project-root>/
  service/          # sequence 주석 파일 (*.go, 재귀 탐색, 도메인 폴더 지원)
  db/queries/       # sqlc 쿼리 파일 (*.sql)
  api/openapi.yaml  # OpenAPI 3.0 spec
  model/            # Go interface, func (*.go)
```

### DDL 규칙

`<root>/db/*.sql`의 `CREATE TABLE` 문에서 컬럼 타입을 추출한다.

타입 매핑:

| PostgreSQL | Go |
|---|---|
| `BIGINT`, `INTEGER`, `SERIAL`, `BIGSERIAL` | `int64` |
| `VARCHAR`, `TEXT`, `UUID` | `string` |
| `BOOLEAN` | `bool` |
| `TIMESTAMPTZ`, `TIMESTAMP` | `time.Time` |
| `NUMERIC`, `DECIMAL`, `FLOAT` | `float64` |

FK 관계와 인덱스도 파싱한다:

```sql
-- 인라인 FK
user_id BIGINT NOT NULL REFERENCES users(id)

-- 독립 FK
CONSTRAINT fk_user FOREIGN KEY (user_id) REFERENCES users(id)

-- 인덱스
CREATE INDEX idx_reservations_room_time ON reservations (room_id, start_at, end_at);
```

request 파라미터의 PascalCase 이름을 snake_case로 변환하여 DDL 컬럼과 매칭한다:
- `RoomID` → `room_id` → DDL: `BIGINT` → `int64` → `strconv.ParseInt(...)` + 400 early return
- `StartAt` → `start_at` → DDL: `TIMESTAMPTZ` → `time.Time` → `time.Parse(time.RFC3339, ...)` + 400 early return

### sqlc 쿼리 규칙

파일명이 모델명이 된다 (복수형 → 단수화 + PascalCase):
- `users.sql` → `User`
- `rooms.sql` → `Room`
- `reservations.sql` → `Reservation`
- `courses.sql` → `Course`

단수화 규칙:
1. `ies` → `y` (categories → category)
2. `sses` → `ss` (addresses → address, classes → class)
3. `xes` → `x` (boxes → box)
4. 나머지 `s` → 제거 (users → user, courses → course)

메서드는 `-- name:` 주석에서 추출. 카디널리티로 반환 타입이 결정된다:
```sql
-- name: FindByID :one       → (*User, error)
-- name: ListByUserID :many  → ([]Reservation, error)
-- name: UpdateStatus :exec  → error
```

### OpenAPI 규칙

- `operationId`가 서비스 함수명과 매칭된다
- request body의 `$ref` schema → request 필드
- path/query parameters → request 필드
- response 200의 `$ref` schema → response 필드
- x- 확장 지원 (아래 별도 섹션 참조)

### OpenAPI x- 확장 문법

SSaC에는 비즈니스 파라미터만 선언하고, 페이지네이션/정렬/필터/관계 포함 같은 인프라 파라미터는 OpenAPI x- 확장에 선언한다. 코드젠이 x-를 읽어 자동으로 `QueryOpts`를 구성한다.

#### x-pagination — 페이지네이션

```yaml
x-pagination:
  style: offset        # offset | cursor
  defaultLimit: 20     # 기본 반환 건수
  maxLimit: 100        # 최대 반환 건수
```

코드젠 결과:
```go
opts := QueryOpts{}
if v := c.Query("limit"); v != "" {
    opts.Limit, _ = strconv.Atoi(v)
}
if v := c.Query("offset"); v != "" {
    opts.Offset, _ = strconv.Atoi(v)
}
```

#### x-sort — 정렬

```yaml
x-sort:
  allowed: [start_at, created_at]   # 정렬 가능 컬럼
  default: start_at                 # 기본 정렬 컬럼 (없으면 allowed[0])
  direction: desc                   # 기본 방향: asc | desc
```

코드젠 결과:
```go
if v := c.Query("sort"); v != "" {
    opts.SortCol = v
}
```

#### x-filter — 필터

```yaml
x-filter:
  allowed: [status, room_id]       # 필터 가능 컬럼
```

#### x-include — 정방향 FK include

```yaml
x-include:
  allowed: [room_id:rooms.id, user_id:users.id]   # FK컬럼:참조테이블.참조컬럼
```

#### 복합 예시

```yaml
/api/reservations:
  get:
    operationId: ListReservations
    x-pagination:
      style: offset
      defaultLimit: 20
      maxLimit: 100
    x-sort:
      allowed: [start_at, created_at]
      default: start_at
      direction: desc
    x-filter:
      allowed: [status, room_id]
    x-include:
      allowed: [room_id:rooms.id, user_id:users.id]
```

대응하는 SSaC — 비즈니스 파라미터(`UserID`)만 선언:
```go
// @sequence get
// @model Reservation.ListByUserID
// @param UserID currentUser
// @result reservations []Reservation

// @sequence response json
// @var reservations
func ListReservations(w http.ResponseWriter, r *http.Request) {}
```

모델 인터페이스에 미치는 영향:
- x- 있는 operation의 메서드에 `opts QueryOpts` 파라미터 추가
- `:many` + x-pagination → 반환 타입: `([]T, int, error)` (total count 포함)
- `QueryOpts` struct가 `models_gen.go`에 자동 생성됨

### Go interface 규칙

- interface 메서드 → 모델 메서드로 등록
- `@func`은 `package.funcName` 형식 필수 (e.g. `auth.verifyPassword`). 외부 패키지 함수를 호출하는 코드만 생성. import 경로는 spec 파일의 Go import 블록에서 가져옴

### @dto 태그

DDL 테이블이 없는 순수 DTO 타입은 `// @dto` 주석을 달아 선언한다:

```go
// @dto
type Token struct {
    AccessToken string
    ExpiresAt   string
}
```

`@dto` 태그가 있으면 SymbolTable에 DTO로 등록되어, 교차 검증에서 DDL 테이블 매칭을 건너뛴다.

---

## 모델 인터페이스 파생 생성

`ssac gen`에서 심볼 테이블이 있으면 3가지 SSOT를 교차하여 모델 인터페이스를 `<outDir>/model/models_gen.go`에 생성한다.

교차 규칙:
- **sqlc**: 메서드명과 카디널리티 (`:one`→포인터, `:many`→슬라이스, `:exec`→error만)
- **SSaC**: 모든 @param 소스 포함 (request, currentUser, dot notation, 리터럴 DDL 역매핑. 실제 사용된 메서드만)
- **OpenAPI x-**: 인프라 파라미터 (`x-pagination` 있으면 `opts QueryOpts` 추가, `:many`+x-pagination → total count 포함)

모든 `@param` 소스가 인터페이스에 포함된다:

| 소스 | 이름 결정 | 타입 결정 |
|---|---|---|
| `request` | 파라미터명 lcFirst | DDL 컬럼 타입 |
| `currentUser` | 파라미터명 lcFirst | DDL 컬럼 타입 |
| dot notation (`user.ID`) | 결합 (`userID`) | 참조 테이블 DDL 조회 |
| 리터럴 (`"pending"`) | DDL 역매핑 (미사용 string 컬럼) | `string` |

생성 예시:
```go
package model

import "time"

type ReservationModel interface {
    CountByRoomID(roomID int64) (*int, error)
    Create(userID int64, roomID int64, startAt time.Time, endAt time.Time) (*Reservation, error)
    FindByID(reservationID int64) (*Reservation, error)
    FindConflict(roomID int64, startAt time.Time, endAt time.Time) (*Reservation, error)
    ListByUserID(userID int64, opts QueryOpts) ([]Reservation, int, error)
    UpdateStatus(reservationID int64, status string) error
}

type SessionModel interface {
    Create(userID int64) (*Token, error)
}

type QueryOpts struct {
    Limit    int
    Offset   int
    Cursor   string
    SortCol  string
    SortDir  string
    Filters  map[string]string
    Includes []string
}
```

### 검증 레벨

| 레벨 | 동작 |
|---|---|
| ERROR | 코드 생성 중단, exit code 1 |
| WARNING | 메시지 출력, 코드 생성 계속 |

WARNING 예시: put/delete 후 갱신 없이 response에서 이전 변수를 사용하면 stale 데이터 경고

---

## 전체 예시

### spec 파일

```go
// specs/service/login.go
package service

import (
    "net/http"
    "github.com/geul-org/fullend/pkg/auth"
)

// @sequence get
// @model User.FindByEmail
// @param Email request
// @result user User

// @sequence guard nil user
// @message "사용자를 찾을 수 없습니다"

// @sequence call
// @func auth.verifyPassword
// @param user.PasswordHash
// @param Password request
// @message "비밀번호가 일치하지 않습니다"

// @sequence post
// @model Session.Create
// @param user.ID
// @result token Token

// @sequence response json
// @var token
func Login(w http.ResponseWriter, r *http.Request) {}
```

### 생성 코드

```go
// artifacts/service/login.go
package service

import (
    "net/http"

    "github.com/gin-gonic/gin"
    "github.com/geul-org/fullend/pkg/auth"
)

func Login(c *gin.Context) {
    email := c.Query("Email")
    password := c.Query("Password")

    // get
    user, err := userModel.FindByEmail(email)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "User 조회 실패"})
        return
    }

    // guard nil
    if user == nil {
        c.JSON(http.StatusNotFound, gin.H{"error": "사용자를 찾을 수 없습니다"})
        return
    }

    // call func
    _, err = auth.VerifyPassword(auth.VerifyPasswordRequest{PasswordHash: user.PasswordHash, Password: password})
    if err != nil {
        c.JSON(http.StatusUnauthorized, gin.H{"error": "비밀번호가 일치하지 않습니다"})
        return
    }

    // post
    token, err := sessionModel.Create(user.ID)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Session 생성 실패"})
        return
    }

    // response json
    c.JSON(http.StatusOK, gin.H{
        "token": token,
    })
}
```
