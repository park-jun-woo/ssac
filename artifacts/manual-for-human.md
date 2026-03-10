# SSaC v2 Manual

## 개요

SSaC(Service Sequences as Code)는 Go 주석 기반 선언적 서비스 로직을 파싱하여 Go+gin 구현 코드를 생성하는 CLI 도구다.

v2에서는 **한 시퀀스 = 한 줄** 원칙으로, 파라미터 출처가 문법에 내장된 표현식 기반 DSL을 사용한다.

```
specs/service/**/*.go  →  ssac parse  →  ssac validate  →  ssac gen  →  artifacts/service/**/*.go
      (주석 DSL)            (구조체)        (정합성 검증)      (Go 코드)      (gofmt 완료)
```

## 설치 & 실행

```bash
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
ssac parse specs/dummy-study        # 프로젝트 루트 지정
```

### validate

내부 정합성 + 외부 SSOT 교차 검증을 수행한다.

```bash
ssac validate specs/backend/service   # 내부 검증만 (외부 SSOT 없음)
ssac validate specs/dummy-study       # 외부 SSOT 교차 검증 (자동 감지)
```

검증 항목:

| 구분 | 항목 |
|---|---|
| 내부 | 타입별 필수 요소 누락 |
| 내부 | Model.Method 형식 |
| 내부 | 변수 흐름 (선언 전 참조) |
| 외부 | 모델/메서드 존재 (sqlc queries, Go interface) |
| 외부 | request 필드 존재 (OpenAPI, 정방향+역방향) |
| 외부 | response 필드 매핑 (OpenAPI, 정방향+역방향) |
| 외부 | Stale 데이터 경고 (put/delete 후 재조회 없이 response 사용) |

### gen

validate → 코드 생성 → gofmt. 검증 실패 시 코드 생성을 중단한다.

```bash
ssac gen <service-dir> <out-dir>
ssac gen specs/dummy-study artifacts/test/dummy-out
```

---

## 주석 DSL 문법

### 기본 구조

```go
package service

import "myapp/auth"

// @get Course course = Course.FindByID(request.CourseID)
// @empty course "코스를 찾을 수 없습니다"
// @response {
//   course: course
// }
func GetCourse() {}
```

- 하나의 `.go` 파일에 여러 함수 가능 (각 함수 위에 `// @` 시퀀스 주석)
- 함수 위에 `// @` 주석으로 시퀀스를 한 줄씩 나열
- `@response`만 멀티라인 블록 (`{ ... }`)
- spec 파일의 Go import 선언은 생성 코드에 전달됨
- `@type!` — `!` 접미사로 해당 시퀀스의 WARNING 억제 (ERROR는 영향 없음)

### 인자(Args) 표기법

모든 CRUD/call 시퀀스의 인자는 `source.Field` 형식으로 출처를 명시한다.

| 표기 | 의미 | 예시 |
|---|---|---|
| `request.Name` | HTTP 요청 파라미터 (예약 소스) | `request.CourseID` |
| `variable.Field` | 이전 결과 변수의 필드 | `course.InstructorID` |
| `currentUser.Field` | 인증 컨텍스트 (예약 소스) | `currentUser.ID` |
| `config.Field` | 환경 설정 (예약 소스) | `config.APIKey` |
| `query` | QueryOpts (페이지네이션/정렬/필터, 예약 소스) | `query` |
| `"literal"` | 문자열 리터럴 | `"cancelled"` |

**예약 소스 (Reserved Sources)**: `request`, `currentUser`, `config`, `query`는 시스템이 사전 정의하는 특수 소스다.
result 변수명으로 사용하면 validator에서 ERROR가 발생한다.

**타입별 필수 요소:**

| 타입 | 필수 |
|---|---|
| get | Model, Result (Args 선택) |
| post | Model, Result, Args |
| put | Model, Args |
| delete | Model, Args (0-arg WARNING, `@delete!`로 억제) |
| empty, exists | Target, Message |
| state | DiagramID, Inputs, Transition, Message |
| auth | Action, Resource, Message |
| call | Model (pkg.Func 형식) |
| response | (없음, Fields 선택) |

### 도메인 폴더 구조

서비스 파일을 도메인별 폴더로 분류할 수 있다.

```
specs/service/
├── login.go                  ← flat, Domain="", package service
├── auth/
│   └── register.go           ← Domain="auth", package auth
└── course/
    └── create_course.go      ← Domain="course", package course
```

기존 flat 구조(`service/*.go`)는 변경 없이 동작한다.

---

## 시퀀스 타입 (10종)

### @get — 리소스 조회

```go
// @get {Type} {var} = {Model}.{Method}({args...})
```

```go
// @get Course course = Course.FindByID(request.CourseID)
// @get User instructor = User.FindByID(course.InstructorID)
// @get []Reservation reservations = Reservation.ListByRoom(request.RoomID)
```

코드젠:
```go
course, err := courseModel.FindByID(courseID)
if err != nil {
    c.JSON(http.StatusInternalServerError, gin.H{"error": "Course 조회 실패"})
    return
}
```

### @post — 리소스 생성

```go
// @post {Type} {var} = {Model}.{Method}({args...})
```

```go
// @post Session session = Session.Create(currentUser.ID)
```

### @put — 리소스 수정

```go
// @put {Model}.{Method}({args...})
```

```go
// @put Room.Update(request.RoomID, request.Name, request.Capacity)
```

### @delete — 리소스 삭제

```go
// @delete {Model}.{Method}({args...})
```

```go
// @delete Room.Delete(request.RoomID)
// @delete! Room.DeleteAll()              — ! 접미사로 0-arg WARNING 억제
```

### @empty — null이면 종료 (404)

```go
// @empty {target} "{message}"
```

- target: 변수명(`course`) 또는 변수.필드(`course.InstructorID`)

```go
// @empty course "코스를 찾을 수 없습니다"
// @empty course.InstructorID "강사가 지정되지 않았습니다"
```

코드젠 (타입에 따라 zero value 비교):
```go
// pointer
if course == nil {
    c.JSON(http.StatusNotFound, gin.H{"error": "코스를 찾을 수 없습니다"})
    return
}
// int
if reservationCount == 0 { ... }
```

### @exists — 존재하면 종료 (409)

```go
// @exists {target} "{message}"
```

```go
// @exists conflict "해당 시간에 이미 예약이 있습니다"
```

### @state — 상태 전이 검사

```go
// @state {diagramID} {key: var.Field, ...} "{transition}" "{message}"
```

- `{diagramID}`: 상태 다이어그램 패키지 식별자 (states/ 하위)
- `{inputs}`: JSON 형식의 입력 매핑
- `{transition}`: 시도할 전이 액션
- `{message}`: 실패 시 에러 메시지

```go
// 단순
// @state reservation {status: reservation.Status} "cancel" "취소할 수 없습니다"

// 복합 입력
// @state course {status: course.Status, createdAt: course.CreatedAt} "publish" "발행할 수 없습니다"
```

코드젠:
```go
if err := reservationstate.CanTransition(reservationstate.Input{
    Status: reservation.Status,
}, "cancel"); err != nil {
    c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
    return
}
```

import: `"states/reservationstate"`

### @auth — 권한 검사 (OPA)

```go
// @auth "{action}" "{resource}" {key: var.Field, ...} "{message}"
```

- `{action}`: OPA 액션 (문자열 리터럴)
- `{resource}`: OPA 리소스 (문자열 리터럴)
- `{inputs}`: JSON 형식의 추가 컨텍스트 (소유권, 조직 등)
- `{message}`: 실패 시 에러 메시지

```go
// 기본
// @auth "delete" "room" {} "권한 없음"

// 소유권 검사
// @auth "cancel" "reservation" {id: reservation.ID, owner: reservation.UserID} "권한 없음"

// 조직 기반
// @auth "update" "course" {id: course.ID, orgId: course.OrgID} "권한 없음"
```

코드젠:
```go
currentUser := c.MustGet("currentUser").(*model.CurrentUser)
if err := authz.Check(currentUser, "cancel", "reservation", authz.Input{
    ID:    reservation.ID,
    Owner: reservation.UserID,
}); err != nil {
    c.JSON(http.StatusForbidden, gin.H{"error": "권한 없음"})
    return
}
```

### @call — 외부 함수 호출

```go
// result 있음
// @call {Type} {var} = {package}.{Func}({args...})

// result 없음
// @call {package}.{Func}({args...})
```

- spec 파일에 import를 명시해야 한다
- result 없음 → guard-style (401 Unauthorized)
- result 있음 → value-style (500 InternalServerError)

```go
// guard형 (result 없음)
// @call auth.VerifyPassword(user.PasswordHash, request.Password)

// value형 (result 있음)
// @call Token token = auth.IssueToken(user.ID)
// @call Refund refund = billing.CalculateRefund(reservation.ID, reservation.StartAt, reservation.EndAt)
```

코드젠 (guard형):
```go
if _, err = auth.VerifyPassword(auth.VerifyPasswordRequest{user.PasswordHash, password}); err != nil {
    c.JSON(http.StatusUnauthorized, gin.H{"error": "verifyPassword 호출 실패"})
    return
}
```

코드젠 (value형):
```go
refund, err := billing.CalculateRefund(billing.CalculateRefundRequest{reservation.ID, reservation.StartAt, reservation.EndAt})
if err != nil {
    c.JSON(http.StatusInternalServerError, gin.H{"error": "calculateRefund 호출 실패"})
    return
}
```

### @response — 응답 반환 (필드 매핑 블록)

```go
// @response {
//   {필드명}: {변수},
//   {필드명}: {변수}.{멤버},
//   {필드명}: "{리터럴}"
// }
```

서비스 레이어의 책임: 모델 결과를 OpenAPI response schema에 맞춰 필드별 매핑.
권한별 응답 차이는 서비스 함수 분리로 해결 (조건 분기 금지).

허용되는 값 표현:
- 변수 직접 매핑: `course: course`
- 변수 필드 매핑: `instructor_name: instructor.Name`
- 리터럴: `status: "success"`
- 런타임 함수(`len` 등) 금지 — 집계는 SQL에서 처리

```go
// @response {
//   course: course,
//   instructor_name: instructor.Name,
//   reviews: reviews
// }
```

코드젠:
```go
c.JSON(http.StatusOK, gin.H{
    "course":          course,
    "instructor_name": instructor.Name,
    "reviews":         reviews,
})
```

### @message 기본값

명시적 메시지를 생략하면 타입과 컨텍스트로 자동 생성된다:

| 타입 | 기본 메시지 |
|---|---|
| get + Course.FindByID | "Course 조회 실패" |
| post + Session.Create | "Session 생성 실패" |
| put + Room.Update | "Room 수정 실패" |
| delete + Room.Delete | "Room 삭제 실패" |
| empty (course) | "course가 존재하지 않습니다" |
| exists (conflict) | "conflict가 이미 존재합니다" |
| state | "상태 전이가 허용되지 않습니다" |
| auth | "권한이 없습니다" |
| call (auth.verify) | "verify 호출 실패" |

---

## 프로젝트 구조

외부 검증을 사용하려면 다음 디렉토리 구조를 따른다:

```
<project-root>/
  service/          # sequence 주석 파일 (*.go, 재귀 탐색, 도메인 폴더 지원)
  db/*.sql          # DDL (CREATE TABLE → 컬럼 타입)
  db/queries/       # sqlc 쿼리 파일 (*.sql)
  api/openapi.yaml  # OpenAPI 3.0 spec
  model/            # Go interface, func (*.go)
  states/           # 상태 다이어그램 (*.md, Mermaid stateDiagram-v2)
  policy/           # OPA Rego 정책 파일 (*.rego)
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

request 인자의 PascalCase 이름을 snake_case로 변환하여 DDL 컬럼과 매칭:
- `request.RoomID` → `room_id` → DDL: `BIGINT` → `int64` → `strconv.ParseInt(...)` + 400 early return
- `request.StartAt` → `start_at` → DDL: `TIMESTAMPTZ` → `time.Time` → `time.Parse(time.RFC3339, ...)` + 400 early return

### sqlc 쿼리 규칙

파일명이 모델명이 된다 (복수형 → 단수화 + PascalCase):
- `users.sql` → `User`
- `reservations.sql` → `Reservation`

단수화 규칙:
1. `ies` → `y` (categories → category)
2. `sses` → `ss` (addresses → address)
3. `xes` → `x` (boxes → box)
4. 나머지 `s` → 제거 (users → user)

메서드는 `-- name:` 주석에서 추출. 카디널리티로 반환 타입 결정:
```sql
-- name: FindByID :one       → (*User, error)
-- name: ListByUserID :many  → ([]Reservation, error)
-- name: UpdateStatus :exec  → error
```

### OpenAPI 규칙

- `operationId`가 서비스 함수명과 매칭
- request body의 `$ref` schema → request 필드
- path/query parameters → request 필드
- response 200의 `$ref` schema → response 필드
- x- 확장 지원 (아래 참조)

### @dto 태그

DDL 테이블이 없는 순수 DTO 타입:

```go
// @dto
type Token struct {
    AccessToken string
    ExpiresAt   string
}
```

---

## OpenAPI x- 확장

SSaC에는 비즈니스 파라미터만 선언하고, 인프라 파라미터는 OpenAPI x- 확장에 선언한다.

### x-pagination

```yaml
x-pagination:
  style: offset        # offset | cursor
  defaultLimit: 20
  maxLimit: 100
```

### x-sort

```yaml
x-sort:
  allowed: [start_at, created_at]
  default: start_at
  direction: desc       # asc | desc
```

### x-filter

```yaml
x-filter:
  allowed: [status, room_id]
```

### x-include

```yaml
x-include:
  allowed: [room_id:rooms.id, user_id:users.id]   # FK컬럼:참조테이블.참조컬럼
```

SSaC spec에서 `query` 예약 소스를 명시적으로 사용해야 한다:

```go
// @get []Reservation reservations = Reservation.ListByUserID(currentUser.ID, query)
```

코드젠 효과:
- SSaC에 `query` 인자가 있는 메서드에만 `opts QueryOpts` 파라미터 추가 (암묵적 삽입 없음)
- `query` + `[]Type` 결과 → 반환 타입: `([]T, int, error)` (total count 포함)
- `QueryOpts` struct 자동 생성 (Limit, Offset, Cursor, SortCol, SortDir, Filters)

교차 검증:
- OpenAPI에 x-extensions 있는데 SSaC에 `query` 없으면 **WARNING**
- SSaC에 `query` 있는데 OpenAPI에 x-extensions 없으면 **ERROR**

---

## 모델 인터페이스 파생

`ssac gen`에서 심볼 테이블이 있으면 3가지 SSOT를 교차하여 `<outDir>/model/models_gen.go`에 생성한다.

- **sqlc**: 메서드명 + 카디널리티 (`:one`→포인터, `:many`→슬라이스, `:exec`→error)
- **SSaC**: 모든 인자 포함 (request, currentUser, 변수 참조, 리터럴 DDL 역매핑)
- **OpenAPI x-**: 인프라 파라미터 (`opts QueryOpts` 추가)

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

type QueryOpts struct {
    Limit   int
    Offset  int
    Cursor  string
    SortCol string
    SortDir string
    Filters map[string]string
}
```

---

## 검증 레벨

| 레벨 | 동작 |
|---|---|
| ERROR | 코드 생성 중단, exit code 1 |
| WARNING | 메시지 출력, 코드 생성 계속 |

WARNING 예시: put/delete 후 갱신 없이 response에서 이전 변수를 사용하면 stale 데이터 경고

### WARNING 억제 (`!` 접미사)

모든 시퀀스 타입에 `!` 접미사를 붙이면 해당 시퀀스의 WARNING을 억제한다. ERROR는 영향 없음.

```go
// @delete! Room.DeleteAll()              — 0-arg 전체 삭제 WARNING 억제
// @response! { room: room }              — stale 데이터 WARNING 억제
```

---

## 전체 예시

### spec 파일

```go
// specs/service/cancel_reservation.go
package service

import "myapp/billing"

// @auth "cancel" "reservation" {id: request.ReservationID} "권한 없음"
// @get Reservation reservation = Reservation.FindByID(request.ReservationID)
// @empty reservation "예약을 찾을 수 없습니다"
// @state reservation {status: reservation.Status} "cancel" "취소할 수 없습니다"
// @call Refund refund = billing.CalculateRefund(reservation.ID, reservation.StartAt, reservation.EndAt)
// @put Reservation.UpdateStatus(request.ReservationID, "cancelled")
// @get Reservation reservation = Reservation.FindByID(request.ReservationID)
// @response {
//   reservation: reservation,
//   refund: refund
// }
func CancelReservation() {}
```

### 생성 코드

```go
// artifacts/service/cancel_reservation.go
package service

import (
    "net/http"

    "github.com/gin-gonic/gin"
    "myapp/billing"
    "states/reservationstate"
)

func CancelReservation(c *gin.Context) {
    reservationIDStr := c.Param("ReservationID")
    reservationID, err := strconv.ParseInt(reservationIDStr, 10, 64)
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "invalid path parameter"})
        return
    }

    // auth
    currentUser := c.MustGet("currentUser").(*model.CurrentUser)
    if err := authz.Check(currentUser, "cancel", "reservation", authz.Input{
        ID: reservationID,
    }); err != nil {
        c.JSON(http.StatusForbidden, gin.H{"error": "권한 없음"})
        return
    }

    // get
    reservation, err := reservationModel.FindByID(reservationID)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Reservation 조회 실패"})
        return
    }

    // empty
    if reservation == nil {
        c.JSON(http.StatusNotFound, gin.H{"error": "예약을 찾을 수 없습니다"})
        return
    }

    // state
    if err := reservationstate.CanTransition(reservationstate.Input{
        Status: reservation.Status,
    }, "cancel"); err != nil {
        c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
        return
    }

    // call
    out, err := billing.CalculateRefund(billing.CalculateRefundRequest{
        ID:      reservation.ID,
        StartAt: reservation.StartAt,
        EndAt:   reservation.EndAt,
    })
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "calculateRefund 호출 실패"})
        return
    }
    refund := out

    // put
    err = reservationModel.UpdateStatus(reservationID, "cancelled")
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Reservation 수정 실패"})
        return
    }

    // get (re-fetch)
    reservation, err = reservationModel.FindByID(reservationID)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Reservation 조회 실패"})
        return
    }

    // response
    c.JSON(http.StatusOK, gin.H{
        "reservation": reservation,
        "refund":      refund,
    })
}
```
