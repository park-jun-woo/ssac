# SSaC v2 — AI Compact Reference

## CLI

```
ssac parse [dir]              # Print parsed sequence structure (default: specs/backend/service/)
ssac validate [dir]           # Internal validation or external SSOT cross-validation (auto-detect)
ssac gen <service-dir> <out>  # validate → codegen → gofmt (with symbol table: type conversion + model interface generation)
```

## Tech Stack

Go 1.24+, `go/ast` (parsing), `text/template` (codegen), `gopkg.in/yaml.v3` (OpenAPI), `github.com/gin-gonic/gin` (generated code target)

## DSL Syntax — One Line Per Sequence

10 sequence types. Each is a single comment line (except `@response` which is a multi-line block).

### CRUD — Model Operations

```go
// @get Type var = Model.Method(args...)        — Query (result required)
// @post Type var = Model.Method(args...)       — Create (result required)
// @put Model.Method(args...)                   — Update (no result)
// @delete Model.Method(args...)                — Delete (no result)
```

Args format: `source.Field` or `"literal"`
- `request.CourseID` — from HTTP request (reserved source)
- `course.InstructorID` — from previous result variable
- `currentUser.ID` — from auth context (reserved source)
- `config.APIKey` — from environment config (reserved source)
- `"cancelled"` — string literal

Reserved sources: `request`, `currentUser`, `config` — cannot be used as result variable names.

Required elements per type:

| Type | Required |
|---|---|
| get | Model, Result (Args optional) |
| post | Model, Result, Args |
| put | Model, Args |
| delete | Model, Args (0-arg WARNING, `@delete!` suppresses) |
| empty, exists | Target, Message |
| state | DiagramID, Inputs, Transition, Message |
| auth | Action, Resource, Message |
| call | Model (pkg.Func format) |
| response | (none, Fields optional) |

### WARNING Suppression (`!` suffix)

Append `!` to any sequence type to suppress WARNINGs for that sequence. ERRORs are unaffected.

```go
// @delete! Room.DeleteAll()              — Suppresses 0-arg WARNING
// @response! { room: room }              — Suppresses stale data WARNING
```

### Guards

```go
// @empty target "message"                      — Fail if nil/zero (404)
// @exists target "message"                     — Fail if not nil/zero (409)
```

Target: variable (`course`) or variable.field (`course.InstructorID`)

### State Transition

```go
// @state diagramID {key: var.Field, ...} "transition" "message"
```

- `{inputs}`: JSON-style input mapping to state diagram package
- Codegen: `{id}state.CanTransition({id}state.Input{...}, "transition")`

### Auth — OPA Permission Check

```go
// @auth "action" "resource" {key: var.Field, ...} "message"
```

- `{inputs}`: JSON-style context for OPA policy (ownership, org, etc.)
- Codegen: `authz.Check(currentUser, "action", "resource", authz.Input{...})`
- `currentUser` auto-extracted from `c.MustGet("currentUser")`

### Call — External Function

```go
// @call Type var = package.Func(args...)       — With result
// @call package.Func(args...)                  — Without result (guard-style error)
```

- Package name from Go import declarations in spec file
- With result: 500 on error. Without result: 401 on error.

### Response — Field Mapping Block

```go
// @response {
//   fieldName: variable,
//   fieldName: variable.Member,
//   fieldName: "literal"
// }
```

- Maps model results to OpenAPI response schema field by field
- No runtime functions (`len` etc.) — aggregation belongs in SQL
- Permission-based response differences → separate service functions (no conditionals)

## Full Example

```go
package service

import "myapp/auth"

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

## Directory Structure

```
cmd/ssac/main.go                 # CLI entrypoint
parser/                          # Comments → []ServiceFunc
  types.go                       #   IR structs (ServiceFunc, Sequence, Arg, Result)
  parser.go                      #   One-line expression parser
validator/                       # Internal + external SSOT validation
  validator.go                   #   Validation rules
  symbol.go                     #   Symbol table (DDL, OpenAPI, sqlc, model)
  errors.go                      #   ValidationError
generator/                       # Target interface-based codegen
  target.go                      #   Target interface + DefaultTarget()
  go_target.go                   #   GoTarget: Go+gin code generation
  go_templates.go                #   Go+gin templates
  generator.go                   #   Wrappers (Generate, GenerateWith) + utils
specs/                           # Declarations (input, SSOT)
  dummy-study/                   #   Study room reservation demo project
    service/ db/ db/queries/ api/ model/ states/ policy/
  plans/                         #   Implementation plans
artifacts/                       # Documentation
v1/                              # Archived v1 code (reference only)
```

## External Validation Project Layout

Auto-detected by `ssac validate <project-root>`:
- `<root>/service/**/*.go` — Sequence specs (recursive, domain folder support)
- `<root>/db/*.sql` — DDL (CREATE TABLE → column types)
- `<root>/db/queries/*.sql` — sqlc queries (filename→model, `-- name: Method :cardinality`)
- `<root>/api/openapi.yaml` — OpenAPI 3.0 (operationId=function name, x-pagination/sort/filter/include)
- `<root>/model/*.go` — Go interface→model methods, `// @dto`→DTO without DDL table
- `<root>/states/*.md` — State diagram definitions (mermaid stateDiagram-v2)
- `<root>/policy/*.rego` — OPA Rego policy files

## Codegen Features (gin framework)

Generated code uses **gin** framework (`c *gin.Context`):
- Function signature: `func Name(c *gin.Context)`
- Error responses: `c.JSON(status, gin.H{"error": "msg"})`
- Success responses: `c.JSON(http.StatusOK, gin.H{...})` with field mapping from `@response`
- Path params: `c.Param("Name")` + type conversion
- Request body: `c.ShouldBindJSON(&req)` (2+ request params) or `c.Query("Name")` (single)
- currentUser: `c.MustGet("currentUser").(*model.CurrentUser)` — auto-generated when @auth or args reference currentUser

Additional features when symbol table (external SSOT) is available:

- **Type conversion**: DDL column type → `strconv.ParseInt`, `time.Parse` with 400 early return
- **Guard value types**: Zero value comparison based on result type (int→`== 0`/`> 0`, pointer→`== nil`/`!= nil`)
- **Stale data warning**: WARNING when response uses variable after put/delete without re-fetch (suppressed by `@response!`)
- **`:=` vs `=` tracking**: Go variable re-declaration uses `=` for already-declared variables
- **Go naming conventions**: Initialism-aware `lcFirst`/`ucFirst` (e.g. `ID`→`id`, `URL`→`url`)
- **@dto tag**: `// @dto` annotated struct → skips DDL table matching
- **DDL FK/Index parsing**: REFERENCES (inline/constraint), CREATE INDEX → `DDLTable.ForeignKeys`, `DDLTable.Indexes`
- **QueryOpts auto-pass**: x-extensions present → `opts := QueryOpts{}` + `c.Query()` based parsing + `opts` arg appended
- **List 3-tuple return**: many + QueryOpts → `result, total, err :=` (includes count)
- **Model interface derivation**: 3 SSOT sources → `<outDir>/model/models_gen.go`
  - sqlc: method names, cardinality (:one→`*T`, :many→`[]T`, :exec→`error`)
  - SSaC: all args included (request, currentUser, variable refs, literals→DDL reverse-mapping)
  - OpenAPI x-: infrastructure params (x-pagination → `opts QueryOpts` added)
- **Domain folder structure**: `service/auth/login.go` → `Domain="auth"` → `outDir/auth/login.go`, `package auth`
- **@state codegen**: `@state {id} {inputs} "transition"` → `{id}state.CanTransition({id}state.Input{...}, "transition")`, import `"states/{id}state"`
- **@auth codegen**: `@auth "action" "resource" {inputs}` → `authz.Check(currentUser, "action", "resource", authz.Input{...})`
- **@call codegen**: `@call pkg.Func(args)` → `pkg.Func(pkg.FuncRequest{...})`. No result → guard-style (401), with result → value-style (500)
- **Spec file imports**: Parser collects Go import declarations from spec files and passes them to generated code

Singularization rules (sqlc filename → model name): `ies`→`y`, `sses`→`ss`, `xes`→`x`, otherwise remove trailing `s`

## OpenAPI x- Extensions

Infrastructure parameters declared in OpenAPI x- extensions. SSaC specs only declare business parameters.

```yaml
/api/reservations:
  get:
    operationId: ListReservations
    x-pagination:                    # style: offset|cursor, defaultLimit, maxLimit
      style: offset
      defaultLimit: 20
      maxLimit: 100
    x-sort:                          # allowed columns, default, direction
      allowed: [start_at, created_at]
      default: start_at
      direction: desc
    x-filter:                        # allowed filter columns
      allowed: [status, room_id]
    x-include:                       # FK_column:ref_table.ref_column
      allowed: [room_id:rooms.id, user_id:users.id]
```

Codegen effects:
- Operations with x- get `opts QueryOpts` parameter in model methods
- `:many` + x-pagination → return type includes total count: `([]T, int, error)`
- `QueryOpts` struct auto-generated (Limit, Offset, Cursor, SortCol, SortDir, Filters)

## Coding Conventions

- gofmt compliant, immediate error handling (early return)
- Filenames: snake_case, variables/functions: camelCase, types: PascalCase
- Go common initialisms: `ID`, `URL`, `HTTP`, `API` etc. — all-caps (exported) or all-lowercase (unexported first word)
- Tests: `go test ./parser/... ./validator/... ./generator/... -count=1`
- 78 tests: parser 25 + validator 33 + generator 20
