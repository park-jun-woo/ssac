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
func CreateSession(w http.ResponseWriter, r *http.Request) {}
```

This 10-line declaration generates the following code:

```go
func CreateSession(w http.ResponseWriter, r *http.Request) {
    projectID := r.FormValue("ProjectID")
    command := r.FormValue("Command")

    project, err := projectModel.FindByID(projectID)
    if err != nil {
        http.Error(w, "Project lookup failed", http.StatusInternalServerError)
        return
    }

    if project == nil {
        http.Error(w, "project not found", http.StatusNotFound)
        return
    }

    session, err := sessionModel.Create(projectID, command)
    if err != nil {
        http.Error(w, "Session creation failed", http.StatusInternalServerError)
        return
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]interface{}{
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
| `post` | Resource creation |
| `put` | Resource update |
| `delete` | Resource deletion |
| `password` | Password comparison |
| `call` | External call (@component / @func) |
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
- Component/func existence (Go interface)
- Stale data warning: put/delete followed by response without re-fetch (WARNING level)

```bash
ssac validate specs/dummy-study      # With external validation
ssac validate specs/backend/service  # Internal validation only
```

## Code Generation Features

When external SSOT (symbol table) is available, `ssac gen` adds:
- **Type conversion**: DDL column types → `strconv.ParseInt`, `time.Parse` with 400 Bad Request early return
- **Guard value types**: Type-aware zero checks (`int` → `== 0`/`> 0`, pointer → `== nil`/`!= nil`)
- **Source resolution**: `@param Name currentUser` → `currentUser.Name`
- **Model interface derivation**: Crosses 3 SSOT sources → `<outDir>/model/models_gen.go`
  - sqlc: method names + cardinality (`:one`→`*T`, `:many`→`[]T`, `:exec`→`error`)
  - SSaC: business parameters (only methods actually used)
  - OpenAPI x-extensions → `opts QueryOpts` parameter added to model methods

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
    x-include:                          # allowed relation resources
      allowed: [room, user]
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
generator/                       # Type-based templates → Go code, model interface derivation
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
  service/*.go            # Sequence specs
  db/*.sql                # DDL (CREATE TABLE → column types)
  db/queries/*.sql        # sqlc queries (-- name: Method :cardinality)
  api/openapi.yaml        # OpenAPI 3.0 (operationId = function name, x-pagination/sort/filter/include)
  model/*.go              # Go interface (component), func
```

## Tests

```bash
go test ./parser/... ./validator/... ./generator/... -v
```

48 tests: parser 14 + generator 6 + validator 28

## License

MIT
