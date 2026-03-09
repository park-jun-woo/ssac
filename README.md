# SSaC — Service Sequences as Code

Service logic is a series of decisions: which model to query, what to guard against, when to reject, what to return. These decisions belong to the person who understands the business — but they get buried in boilerplate, scattered across layers, and lost in rewrites.

SSaC preserves those decisions as a declarative spec. You declare **what** happens and **in what order**. The tool generates the implementation.

```
specs/service/*.go  →  ssac validate  →  ssac gen  →  artifacts/service/*.go
   (comment DSL)        (validation)      (codegen)     (gofmt applied)
```

## Core Idea

Every service function is a sequence of steps. Each step follows a binary contract: **succeed → next line, fail → return**. This is not an abstraction we invented — it's how service logic already works. SSaC makes it explicit.

10 fixed sequence types cover all service-layer operations that follow this contract. If something doesn't fit, delegate it to `call`. The set is closed by design.

No LLM, no inference — pure symbolic codegen from templates. The spec is the source of truth.

```go
// @sequence get
// @model Project.FindByID
// @param ProjectID request
// @result project Project

// @sequence guard nil project
// @message "project not found"

// @sequence post
// @model Session.Create
// @param ProjectID request
// @param Command request
// @result session Session

// @sequence response json
// @var session
func CreateSession(c *gin.Context) {}
```

This 10-line declaration generates the following code (gin framework):

```go
func CreateSession(c *gin.Context) {
    projectID := c.Query("ProjectID")
    command := c.Query("Command")

    project, err := projectModel.FindByID(projectID)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Project lookup failed"})
        return
    }

    if project == nil {
        c.JSON(http.StatusNotFound, gin.H{"error": "project not found"})
        return
    }

    session, err := sessionModel.Create(projectID, command)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Session creation failed"})
        return
    }

    c.JSON(http.StatusOK, gin.H{
        "session": session,
    })
}
```

## Sequence Types (10)

| Type | Role |
|---|---|
| `authorize` | Permission check (OPA, etc.) |
| `get` | Resource lookup |
| `guard nil` | Exit if null |
| `guard exists` | Exit if exists |
| `guard state` | State transition check |
| `post` | Resource creation |
| `put` | Resource update |
| `delete` | Resource deletion |
| `call` | External function call (@func package.funcName) |
| `response` | Return response (json) |

## Install & Run

```bash
go build -o ssac ./cmd/ssac

ssac parse [dir]              # Print parsed sequence structure
ssac validate [dir]           # Internal + external SSOT cross-validation
ssac gen <service-dir> <out>  # validate → codegen → gofmt
```

## Validation

Internal validation (always):
- Missing required tags per type
- `@model` format (`Model.Method`)
- Variable flow (reference before declaration)

External SSOT cross-validation (when project structure detected):
- Model/method existence (sqlc queries, Go interface)
- Request/response field existence (OpenAPI)
- `@func` target existence (Go func declarations in model/*.go)
- Stale data warning: put/delete followed by response without re-fetch (WARNING level)

```bash
ssac validate specs/dummy-study      # With external validation
ssac validate specs/backend/service  # Internal validation only
```

## Code Generation Features

When external SSOT (symbol table) is available, `ssac gen` adds:
- **Type conversion**: DDL column types → `strconv.ParseInt`, `time.Parse` with 400 Bad Request early return
- **`-> column` mapping**: `@param PaymentMethod request -> method` — explicit DDL column mapping
- **Guard value types**: Type-aware zero checks (`int` → `== 0`/`> 0`, pointer → `== nil`/`!= nil`)
- **Source resolution**: `@param Name currentUser` → `currentUser.Name`
- **`@dto` tag**: `// @dto` on struct types without DDL tables — skips DDL table matching in cross-validation
- **DDL FK/Index parsing**: REFERENCES (inline/constraint), CREATE INDEX → available for cross-validation
- **QueryOpts auto-pass**: x-extensions present → `opts := QueryOpts{}` + `opts` arg appended to model call
- **List 3-tuple return**: `:many` + QueryOpts → `result, total, err :=` (includes count)
- **Model interface derivation**: Crosses 3 SSOT sources → `<outDir>/model/models_gen.go`
  - sqlc: method names + cardinality (`:one`→`*T`, `:many`→`[]T`, `:exec`→`error`)
  - SSaC: all @param sources included (request, currentUser, dot notation `user.ID`→`userID`, literal `"pending"`→DDL reverse-mapping)
  - OpenAPI x-extensions → `opts QueryOpts` parameter added to model methods
- **Domain folder structure**: `service/auth/login.go` → outputs to `outDir/auth/login.go` with `package auth`
  - Flat structure (`service/login.go`) backward compatible (Domain="")
- **@func codegen**: `@func auth.verifyPassword` → `auth.VerifyPassword(auth.VerifyPasswordRequest{...})`
  - `@result` absent → guard-style (401 Unauthorized), `@result` present → value-style (500 InternalServerError)
  - `@result var Type.Field` → explicit field extraction (`out.Field`)
  - `@param user.ID -> UserID` → Request struct field mapping
- **Spec file imports**: Go import declarations in spec files are passed to generated code. `@func` package name is the alias of the imported package.
- **`-> column` mapping**: Also used for @func Request struct field mapping: `@param user.ID -> UserID`

## OpenAPI x- Extensions

Infrastructure parameters (pagination, sorting, filtering, relation includes) are declared in OpenAPI `x-` extensions, not in SSaC specs. SSaC only declares business parameters. The codegen reads `x-` and automatically constructs `QueryOpts`.

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

Effects on codegen:
- Methods with x- get `opts QueryOpts` parameter in model interface
- `:many` + x-pagination → return type includes total: `([]T, int, error)`
- `QueryOpts` struct auto-generated in `models_gen.go`

## Project Structure

```
cmd/ssac/                        # CLI entrypoint
parser/                          # Comments → []ServiceFunc
validator/                       # Internal + external SSOT validation
generator/                       # Target interface → multi-language codegen (Go default)
  target.go                      #   Target interface + DefaultTarget()
  go_target.go                   #   GoTarget: Go code generation
  go_templates.go                #   Go templates
  generator.go                   #   Backward-compatible wrappers + utils
specs/                           # Declarations (SSOT)
  dummy-study/                   #   Study room reservation demo project
    service/  db/queries/  api/  model/
  plans/                         #   Implementation plans
artifacts/                       # Documentation
  manual-for-human.md            #   Detailed manual
  manual-for-ai.md               #   Compact AI reference
testdata/                        # Test fixtures
files/                           # Design documents
```

## External Validation Project Layout

```
<project>/
  service/**/*.go         # Sequence specs (recursive, domain folders supported)
  db/*.sql                # DDL (CREATE TABLE → column types)
  db/queries/*.sql        # sqlc queries (-- name: Method :cardinality)
  api/openapi.yaml        # OpenAPI 3.0 (operationId = function name, x-pagination/sort/filter/include)
  model/*.go              # Go interface → model methods, func → @func targets, // @dto → DTO
```

## Tests

```bash
go test ./parser/... ./validator/... ./generator/... -v
```

59 tests: parser 16 + generator 11 + validator 32

## License

MIT
