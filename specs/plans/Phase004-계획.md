# Phase 4 구현 계획

Phase 1(Parser), Phase 2(Generator), Phase 3(Validator)에 이어, 더미 스터디 코드젠 평가에서 도출한 5개 개선 항목을 구현한다.

## 의존 관계

```
Item 1 (guard 값 타입)     ─┐
Item 2 (currentUser)       ─┼─ 독립, 병렬 가능
Item 3 (stale 경고)        ─┘
Item 4 (Model interface)   ─── 독립, 최대 규모
Item 5 (타입 변환)          ─── Item 4에 의존
```


## Item 1: guard exists + 값 타입 컴파일 에러 수정

### 문제

`@result reservationCount int` → `guard exists` → `if reservationCount != nil` 생성. int는 nil 비교 불가.

### 수정 대상

| 파일 | 변경 |
|---|---|
| `generator.go` | `templateData`에 `ZeroCheck`, `ExistsCheck` 필드 추가 |
| `generator.go` | `GenerateFunc`에서 sequence 순회 전 `resultTypes map[string]string` 구축 |
| `generator.go` | `buildTemplateData`에 `resultTypes` 전달, guard일 때 타입별 비교식 설정 |
| `templates.go` | guard nil: `{{.Target}} == nil` → `{{.Target}} {{.ZeroCheck}}` |
| `templates.go` | guard exists: `{{.Target}} != nil` → `{{.Target}} {{.ExistsCheck}}` |
| `generator_test.go` | 기존 `sessionCount != nil` 검증을 `sessionCount > 0`으로 변경 |

### 타입→비교식 매핑

```go
func zeroValueChecks(typeName string) (zeroCheck, existsCheck string) {
    switch typeName {
    case "int", "int32", "int64", "float64":
        return "== 0", "> 0"
    case "bool":
        return "== false", "== true"
    case "string":
        return `== ""`, `!= ""`
    default:
        return "== nil", "!= nil"
    }
}
```

### 테스트 케이스

- guard exists + `int` → `> 0`
- guard nil + struct → `== nil` (기존 동작 유지)
- guard exists + `bool` → `== true`


## Item 2: currentUser 파라미터 변수 미선언 수정

### 문제

`@param UserID currentUser` → `reservationModel.ListByUserID(userID)` 생성. `userID`가 선언되지 않음.

### 수정 대상

| 파일 | 변경 |
|---|---|
| `generator.go` | `buildParamArgs` 내부에서 `resolveParamRef(p.Name)` → `resolveParam(p)` 호출로 변경 |
| `generator.go` | `resolveParam(p Param)` 함수 추가 |

### 구현

```go
func resolveParam(p parser.Param) string {
    // 예약어 source → source.Name
    if p.Source == "currentUser" || p.Source == "config" {
        return p.Source + "." + p.Name
    }
    // 기존 로직
    return resolveParamRef(p.Name)
}
```

생성 결과: `@param UserID currentUser` → `currentUser.UserID`

### 테스트 케이스

- ListMyReservations: `currentUser.UserID` 포함 확인
- CreateReservation: `currentUser.UserID` 포함 확인
- 기존 request source 파라미터 동작 유지


## Item 3: put 후 stale 데이터 반환 경고

### 문제

get → put → response에서 put 전에 가져온 변수를 그대로 반환하면 클라이언트가 stale 데이터를 받는다.

### 수정 대상

| 파일 | 변경 |
|---|---|
| `validator.go` | `validateStaleResponse` 함수 추가 |
| `validator.go` | `validateFunc`에서 호출 |
| `errors.go` | `ValidationError`에 `Level` 필드 추가 (`"ERROR"` / `"WARNING"`) |
| `main.go` | WARNING은 출력하되 exit code에 영향 없음 |

### 로직

```
getVars: map[var]model       — get으로 가져온 변수 → 모델 추적
mutated: map[model]bool      — put/delete로 변경된 모델 추적

sequence 순회:
  get → getVars[result.Var] = model
  put/delete → mutated[model] = true
  get (재조회) → mutated[model] = false
  response → @var가 getVars에 있고, 해당 model이 mutated면 WARNING
```

### 테스트 케이스

- get room → put Room.Update → response @var room → WARNING
- get room → put Room.Update → get room → response @var room → 경고 없음


## Item 4: Model interface 파생 생성

### 문제

SSaC가 `userModel.FindByEmail(email)`을 생성하지만 이 interface가 정의된 곳이 없다. sqlc `Queries.FindByEmail(ctx, email)` 와 SSaC 호출 사이를 연결하는 계약이 부재.

### 핵심: 기존 3 SSOT 교차에서 파생

```
sqlc 쿼리    →  메서드명, 카디널리티(:one/:many/:exec), DDL 컬럼 타입
SSaC spec   →  비즈니스 파라미터명과 출처
OpenAPI x-  →  인프라 파라미터 (x-pagination, x-sort, x-filter, x-include)
```

### Step 4.1: ModelSymbol 확장

```go
// symbol.go

type ModelSymbol struct {
    Methods map[string]MethodInfo  // 기존 map[string]bool에서 변경
}

type MethodInfo struct {
    Cardinality string          // "one", "many", "exec"
    ParamTypes  []ParamTypeInfo
}

type ParamTypeInfo struct {
    Name   string // 컬럼명
    GoType string // "string", "int64", "time.Time" 등
}
```

**영향**: `validator.go`의 `ms.Methods[methodName]` (bool 체크) → `_, ok := ms.Methods[methodName]` 으로 변경.

### Step 4.2: sqlc 카디널리티 파싱

`loadSqlcQueries`에서 `-- name: FindByEmail :one`의 3번째 토큰(카디널리티) 추출.

```go
// 현재: ms.Methods[parts[2]] = true
// 변경: ms.Methods[parts[2]] = MethodInfo{Cardinality: strings.TrimPrefix(parts[3], ":")}
```

### Step 4.3: DDL 컬럼 타입 파싱

`loadDDL(dir string)` 추가. `<root>/db/` 디렉토리에서 DDL `.sql` 파일을 읽어 `CREATE TABLE` 문의 컬럼 타입 추출.

```go
type DDLTable struct {
    Columns map[string]string // snake_case 컬럼명 → Go 타입
}
```

PostgreSQL → Go 타입 매핑:

| PostgreSQL | Go |
|---|---|
| `BIGINT`, `BIGSERIAL`, `INTEGER`, `SERIAL` | `int64` |
| `VARCHAR`, `TEXT`, `UUID` | `string` |
| `BOOLEAN` | `bool` |
| `TIMESTAMPTZ`, `TIMESTAMP` | `time.Time` |
| `NUMERIC`, `DECIMAL` | `float64` |

### Step 4.4: OpenAPI x- 확장 파싱

`openAPIOperation`에 x- 필드 추가:

```go
type openAPIOperation struct {
    // ... 기존 필드
    XPagination *XPagination `yaml:"x-pagination"`
    XSort       *XSort       `yaml:"x-sort"`
    XFilter     *XFilter     `yaml:"x-filter"`
    XInclude    *XInclude    `yaml:"x-include"`
}

type XPagination struct {
    Style        string `yaml:"style"`        // "offset" | "cursor"
    DefaultLimit int    `yaml:"defaultLimit"`
    MaxLimit     int    `yaml:"maxLimit"`
}

type XSort struct {
    Allowed   []string `yaml:"allowed"`
    Default   string   `yaml:"default"`
    Direction string   `yaml:"direction"`
}

type XFilter struct {
    Allowed []string `yaml:"allowed"`
}

type XInclude struct {
    Allowed []string `yaml:"allowed"`
}
```

`OperationSymbol`에 저장:

```go
type OperationSymbol struct {
    RequestFields  map[string]bool
    ResponseFields map[string]bool
    XPagination    *XPagination
    XSort          *XSort
    XFilter        *XFilter
    XInclude       *XInclude
}
```

### Step 4.5: interface 파생 로직

새 파일 `artifacts/internal/generator/model_interface.go`:

1. 심볼 테이블의 각 모델 순회
2. 메서드별로 SSaC spec에서 사용 패턴 조회 → 비즈니스 파라미터 결정
3. operationId 매칭으로 OpenAPI x- 존재 여부 확인 → `opts QueryOpts` 추가 여부 결정
4. 카디널리티로 반환 타입 결정:
   - `:one` → `(*Type, error)`
   - `:many` (x- 없음) → `([]Type, error)`
   - `:many` (x-pagination 있음) → `([]Type, int, error)` (total 포함)
   - `:exec` → `error`

### Step 4.6: QueryOpts struct 생성

x- 확장을 사용하는 operation이 하나라도 있으면 생성:

```go
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

### Step 4.7: 산출물

`<outDir>/model/models_gen.go` — 별도 패키지로 생성. 서비스 코드와 같은 패키지에 두면 나중에 패키지 분리 시 순환 의존이 생길 수 있으므로 처음부터 분리한다.

```go
package model

type UserModel interface {
    FindByEmail(email string) (*User, error)
    FindByID(id int64) (*User, error)
}

type ReservationModel interface {
    FindByID(id int64) (*Reservation, error)
    ListByUserID(userID int64, opts QueryOpts) ([]Reservation, int, error)
    Create(userID int64, roomID int64, startAt time.Time, endAt time.Time) (*Reservation, error)
    CountByRoomID(roomID int64) (int, error)
    UpdateStatus(id int64, status string) error
    FindConflict(roomID int64, startAt time.Time, endAt time.Time) (*Reservation, error)
}

type QueryOpts struct { ... }
```

### Step 4.8: main.go 연동

`runGen`에서 프로젝트 루트가 있고 심볼 테이블 로드 성공 시, 서비스 코드젠 이후 interface 파일도 생성.

### 테스트 케이스

- 더미 스터디 기준 UserModel, RoomModel, ReservationModel, SessionModel interface 검증
- `:one` → 포인터 반환, `:many` → 슬라이스 반환, `:exec` → error만 반환
- x-pagination 있는 operation → `QueryOpts` 파라미터 포함


## Item 5: 타입 변환 코드젠 (Item 4 의존)

### 문제

`r.FormValue("Capacity")`가 string인데 실제로 int여야 한다. 변환 코드가 없다.

### Step 5.1: generator에 심볼 테이블 전달

```go
// 기존
func Generate(funcs []parser.ServiceFunc, outDir string) error
// 변경
func Generate(funcs []parser.ServiceFunc, outDir string, st *validator.SymbolTable) error
```

`st`가 nil이면 기존 동작(string only). non-nil이면 타입 변환 코드 생성.

### Step 5.2: request 파라미터 타입 결정

`@param Name request`에서 Name(PascalCase) → snake_case로 변환 → DDL 컬럼 타입 조회.

```
RoomID → room_id → DDL: BIGINT → Go: int64
StartAt → start_at → DDL: TIMESTAMPTZ → Go: time.Time
Capacity → capacity → DDL: INTEGER → Go: int64
```

### Step 5.3: 타입별 추출 코드 생성

변환 실패 시 400 Bad Request로 early return한다. SSaC의 early return 철학과 일치하며, OpenAPI(Controller)의 입력 검증과는 별개의 Service 레벨 방어다.

| Go 타입 | 생성 코드 |
|---|---|
| `string` | `name := r.FormValue("Name")` |
| `int64` | `name, err := strconv.ParseInt(r.FormValue("Name"), 10, 64)` + early return |
| `bool` | `name, err := strconv.ParseBool(r.FormValue("Name"))` + early return |
| `time.Time` | `name, err := time.Parse(time.RFC3339, r.FormValue("Name"))` + early return |
| `float64` | `name, err := strconv.ParseFloat(r.FormValue("Name"), 64)` + early return |

생성 예시:

```go
capacity, err := strconv.ParseInt(r.FormValue("Capacity"), 10, 64)
if err != nil {
    http.Error(w, "Capacity: 유효하지 않은 값", http.StatusBadRequest)
    return
}
```

### Step 5.4: import 자동 추가

`collectImports`에서 사용된 변환 함수에 따라 `"strconv"`, `"time"` 추가.

### Step 5.5: x- 인프라 파라미터 추출 코드 생성

operation에 x- 확장이 있으면 함수 상단에 삽입:

```go
// x-pagination (offset)
limit := clampLimit(r.URL.Query().Get("limit"), 20, 100)
offset := parseOffset(r.URL.Query().Get("offset"))

// x-sort
sortCol := validateSort(r.URL.Query().Get("sort"), []string{"start_at", "created_at"}, "start_at")
sortDir := validateDirection(r.URL.Query().Get("direction"), "desc")

// x-filter
filters := parseFilters(r.URL.Query(), []string{"status", "room_id"})
```

Model 호출 시 `QueryOpts{Limit: limit, Offset: offset, ...}` 인자 추가.

### Step 5.6: 헬퍼 함수 생성

`<outDir>/helpers_gen.go`에 `clampLimit`, `parseOffset`, `validateSort`, `validateDirection`, `parseFilters`, `parseIncludes` 생성. 정적 코드이므로 한 번만 생성.

### 테스트 케이스

- RoomID → `strconv.ParseInt` 생성 확인
- StartAt → `time.Parse` 생성 확인
- x-pagination 있는 operation → QueryOpts 구성 코드 확인


## 구현 순서

```
[병렬 가능]
  ├─ Item 1: guard 값 타입 (generator.go, templates.go)
  ├─ Item 2: currentUser (generator.go)
  └─ Item 3: stale 경고 (validator.go)

[순차]
  Item 4: Model interface 파생 (symbol.go, model_interface.go)
  └─ Item 5: 타입 변환 (generator.go, templates.go)
```
