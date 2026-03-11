# SSaC — Service Sequences as Code

Service logic is a series of decisions: which model to query, what to guard against, when to reject, what to return. These decisions belong to the person who understands the business — but they get buried in boilerplate, scattered across layers, and lost in rewrites.

SSaC preserves those decisions as a declarative spec. You declare **what** happens and **in what order**. The tool generates the implementation.

```
specs/service/<domain>/*.ssac  →  ssac validate  →  ssac gen  →  artifacts/<domain>/*.go
        (comment DSL)                (validation)      (codegen)     (gofmt applied)
```

## Core Idea

Every service function is a sequence of steps. Each step follows a binary contract: **succeed → next line, fail → return**. This is not an abstraction we invented — it's how service logic already works. SSaC makes it explicit.

11 fixed sequence types + 1 trigger cover all service-layer operations that follow this contract. If something doesn't fit, delegate it to `call`. The set is closed by design.

No LLM, no inference — pure symbolic codegen from templates. The spec is the source of truth.

```go
// @get Project project = Project.FindByID({projectID: request.ProjectID})
// @empty project "project not found"
// @post Session session = Session.Create({projectID: request.ProjectID, command: request.Command})
// @response {
//   session: session
// }
func CreateSession() {}
```

This 5-line declaration generates the following code (gin framework):

```go
func CreateSession(c *gin.Context) {
    var req struct {
        ProjectID int64  `json:"ProjectID"`
        Command   string `json:"Command"`
    }
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
        return
    }
    projectID := req.ProjectID
    command := req.Command

    project, err := projectModel.FindByID(projectID)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Project 조회 실패"})
        return
    }

    if project == nil {
        c.JSON(http.StatusNotFound, gin.H{"error": "project not found"})
        return
    }

    session, err := sessionModel.Create(projectID, command)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Session 생성 실패"})
        return
    }

    c.JSON(http.StatusOK, gin.H{
        "session": session,
    })
}
```

## Sequence Types (11) + Trigger (1)

| Type | Syntax | Role |
|---|---|---|
| `get` | `@get Type var = Model.Method({Key: value})` | Resource lookup (result required) |
| `post` | `@post Type var = Model.Method({Key: value})` | Resource creation (result required) |
| `put` | `@put Model.Method({Key: value})` | Resource update |
| `delete` | `@delete Model.Method({Key: value})` | Resource deletion |
| `empty` | `@empty target "message"` | Exit if nil/zero (404) |
| `exists` | `@exists target "message"` | Exit if exists (409) |
| `state` | `@state id {inputs} "transition" "msg"` | State transition check (409) |
| `auth` | `@auth "action" "resource" {inputs} "msg"` | Permission check (403) |
| `call` | `@call [Type var =] pkg.Func({Key: value})` | External function call |
| `publish` | `@publish "topic" {Key: value} [{options}]` | Async event publishing |
| `response` | `@response { field: var }` or `@response var` | Return response (block or shorthand) |
| **Trigger** | | |
| `subscribe` | `@subscribe "topic"` | Queue event trigger (message type from func param) |

All sequence types use unified `{Key: value}` syntax. Value format: `source.Field` (e.g. `request.CourseID`, `course.InstructorID`, `currentUser.ID`), `query` (QueryOpts), or `"literal"`. Result types support generic wrappers: `Page[T]` (offset pagination), `Cursor[T]` (cursor pagination).

Reserved sources (`request`, `currentUser`, `config`, `query`, `message`) cannot be used as result variable names. `message` is the queue equivalent of `request` — used in `@subscribe` functions only, typed via Go struct in .ssac file. Function signature: `func Name(message TypeName) {}`. Append `!` to any type to suppress WARNINGs (e.g. `@delete!`, `@response!`).

## Install & Run

```bash
go build -o ssac ./cmd/ssac

ssac parse [dir]              # Print parsed sequence structure
ssac validate [dir]           # Internal + external SSOT cross-validation
ssac gen <service-dir> <out>  # validate → codegen → gofmt
```

## Validation

Internal validation (always):
- Missing required elements per type
- Model format (`Model.Method`)
- Variable flow (reference before declaration)

External SSOT cross-validation (when project structure detected):
- Model/method existence (sqlc queries, Go interface)
- Request/response field matching (OpenAPI, forward + reverse)
- Stale data warning: put/delete followed by response without re-fetch (WARNING level, suppressed by `@response!`)
- Reserved source conflict: result variable named `request`/`currentUser`/`config` (ERROR)
- @delete 0-input warning: delete with no inputs (WARNING, suppressed by `@delete!`)

```bash
ssac validate specs/dummy-study      # With external validation
ssac validate specs/backend/service  # Internal validation only
```

## Code Generation Features

Generated code uses **gin** framework (`func Name(c *gin.Context)`):
- Path params: `c.Param()` + type conversion
- Request body: `c.ShouldBindJSON(&req)` (2+ params, or 1+ in POST/PUT) or `c.Query()`
- currentUser: `c.MustGet("currentUser").(*model.CurrentUser)` — auto-generated when needed
- Error responses: `c.JSON(status, gin.H{"error": "msg"})` with early return
- Success responses: `c.JSON(http.StatusOK, gin.H{...})` with field mapping from `@response`

When external SSOT (symbol table) is available, `ssac gen` adds:
- **Type conversion**: DDL column types → `strconv.ParseInt`, `time.Parse` with 400 early return
- **Guard value types**: Type-aware zero checks (`int` → `== 0`/`> 0`, pointer → `== nil`/`!= nil`)
- **`:=` vs `=` tracking**: Go variable re-declaration uses `=` for already-declared variables
- **Go naming conventions**: Initialism-aware naming (e.g. `ID`→`id`, `URL`→`url`)
- **QueryOpts**: `query` reserved source in args → `opts := QueryOpts{}` + `c.Query()` parsing (no implicit injection)
- **List 3-tuple return**: `query` arg + `[]Type` result → `result, total, err :=` (includes count). Not used with Page[T]/Cursor[T].
- **Page[T]/Cursor[T] generics**: `Result.Wrapper` tracks generic wrapper. Model returns `(*pagination.Page[T], error)`.
- **x-pagination type validation**: `offset` ↔ `Page[T]`, `cursor` ↔ `Cursor[T]` cross-check
- **@response shorthand**: `@response var` → `c.JSON(200, var)`. Wrapper fields validated against OpenAPI (Page: items/total, Cursor: items/next_cursor/has_next)
- **Query cross-validation**: OpenAPI x-extensions ↔ SSaC `query` mismatch detection (ERROR/WARNING)
- **Model interface derivation**: Crosses 3 SSOT sources → `<outDir>/model/models_gen.go`
  - sqlc: method names + cardinality (`:one`→`*T`, `:many`→`[]T`, `:exec`→`error`)
  - SSaC: all inputs included (request, currentUser, variable refs, literals→DDL reverse-mapping, query→`opts QueryOpts`)
  - OpenAPI x-extensions: validated against SSaC `query` usage
- **Domain folder structure**: `service/<domain>/*.go` required (flat service/*.go is ERROR). `service/auth/login.go` → outputs to `outDir/auth/login.go` with `package auth`
- **@call codegen**: `pkg.Func(pkg.FuncRequest{Key: value, ...})`. No result → `_, err` guard-style (401), with result → value-style (500)
- **@state codegen**: `err := {id}state.CanTransition({id}state.Input{...}, "transition")` (returns error, not bool)
- **@auth codegen**: `authz.Check(currentUser, "action", "resource", authz.Input{...})`
- **Spec file imports**: Go import declarations in spec files are passed to generated code
- **Package prefix model**: `pkg.Model.Method({...})` — non-DDL models validated against Go interface. Missing interface → WARNING, missing method → ERROR with available methods. Parameter matching: SSaC keys ↔ interface params (`context.Context` excluded). Excluded from `models_gen.go`
- **@publish codegen**: `queue.Publish(c.Request.Context(), "topic", map[string]any{...})`. Options: `queue.WithDelay()`, `queue.WithPriority()`. Import `"queue"` auto-added
- **@subscribe codegen**: `func Name(ctx context.Context, message T) error` — separate from gin handler. Message type is Go struct in .ssac file. Errors → `return fmt.Errorf(...)`. Validation: param required, struct/field existence checked

## OpenAPI x- Extensions

Infrastructure parameters (pagination, sorting, filtering, relation includes) are declared in OpenAPI `x-` extensions, not in SSaC specs. SSaC only declares business parameters.

```yaml
/api/reservations:
  get:
    operationId: ListReservations
    x-pagination:                       # style: offset|cursor, defaultLimit, maxLimit
      style: offset
      defaultLimit: 20
      maxLimit: 100
    x-sort:                             # allowed columns, default, direction
      allowed: [start_at, created_at]
      default: start_at
      direction: desc
    x-filter:                           # allowed filter columns
      allowed: [status, room_id]
    x-include:                          # FK_column:ref_table.ref_column
      allowed: [room_id:rooms.id, user_id:users.id]
```

## Project Structure

```
cmd/ssac/                        # CLI entrypoint
parser/                          # Comments → []ServiceFunc (one-line expression parser)
validator/                       # Internal + external SSOT validation
generator/                       # Target interface → multi-language codegen (Go+gin default)
  target.go                      #   Target interface + DefaultTarget()
  go_target.go                   #   GoTarget: Go+gin code generation
  go_templates.go                #   Go+gin templates
  generator.go                   #   Backward-compatible wrappers + utils
specs/                           # Declarations (SSOT)
  dummy-study/                   #   Study room reservation demo project
    service/  db/queries/  api/  model/
  plans/                         #   Implementation plans
artifacts/                       # Documentation
  manual-for-human.md            #   Detailed manual
  manual-for-ai.md               #   Compact AI reference
testdata/                        # Test fixtures
v1/                              # Archived v1 code (reference only, do not delete)
files/                           # Design documents
```

## External Validation Project Layout

```
<project>/
  service/<domain>/*.ssac  # Sequence specs (domain subfolder required, flat is ERROR)
  db/*.sql                # DDL (CREATE TABLE → column types, FK, indexes)
  db/queries/*.sql        # sqlc queries (-- name: Method :cardinality)
  api/openapi.yaml        # OpenAPI 3.0 (operationId = function name, x-extensions)
  model/*.go              # Go interface → model methods, // @dto → DTO without DDL table
```

## Tests

```bash
go test ./parser/... ./validator/... ./generator/... -count=1
```

146 tests: parser 39 + validator 72 + generator 35

## License

MIT
