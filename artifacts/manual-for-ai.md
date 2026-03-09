# SSaC — AI Compact Reference

## CLI

```
ssac parse [dir]              # Print parsed sequence structure (default: specs/backend/service/)
ssac validate [dir]           # Internal validation or external SSOT cross-validation (auto-detect)
ssac gen <service-dir> <out>  # validate → codegen → gofmt (with symbol table: type conversion + model interface generation)
```

## Tech Stack

Go 1.24+, `go/ast` (parsing), `text/template` (codegen), `gopkg.in/yaml.v3` (OpenAPI), `github.com/gin-gonic/gin` (generated code target)

## DSL Syntax

```go
// @sequence <type>        — Block start. 10 types: authorize|get|guard nil|guard exists|guard state|post|put|delete|call|response
// @model <Model.Method>   — Resource model.method (get/post/put/delete)
// @param <Name> <source> [-> column]  — source: request, currentUser, variable, "literal". -> column: explicit mapping
// @result <var> <Type[.Field]>  — Result binding. Type.Field for explicit Response field extraction
// @message "msg"          — Custom error message (optional, auto-generated default)
// @var <name>             — Variable to include in response
// @action @resource @id   — authorize only (all 3 required)
// @func <package.funcName> — call only (required). e.g. auth.verifyPassword
```

Required tags per type:

| Type | Required |
|---|---|
| authorize | @action, @resource, @id |
| get, post | @model, @result |
| put, delete | @model |
| guard nil/exists | target (variable name on sequence line) |
| guard state | target (stateDiagramID), @param exactly 1 (entity.Field) |
| call | @func package.funcName (required) |
| response | (none, @var is optional) |

## Directory Structure

```
cmd/ssac/main.go                 # CLI entrypoint
parser/                          # Comments → []ServiceFunc
validator/                       # Internal + external SSOT validation
generator/                       # Target interface-based codegen (multi-language extensible)
  target.go                      #   Target interface + DefaultTarget()
  go_target.go                   #   GoTarget: Go+gin code generation
  go_templates.go                #   Go+gin templates
  generator.go                   #   Backward-compatible wrappers (Generate, GenerateWith) + utils
specs/                           # Declarations (input, SSOT)
  dummy-study/                   #   Study room reservation demo project
    service/  db/queries/  api/  model/
  plans/                         #   Implementation plans
artifacts/                       # Documentation
  manual-for-human.md            #   Detailed manual
  manual-for-ai.md               #   Compact reference
testdata/                        # Test fixtures
files/                           # Design documents
```

## External Validation Project Layout

Auto-detected by `ssac validate <project-root>`:
- `<root>/service/**/*.go` — Sequence specs (recursive, domain folder support)
- `<root>/db/*.sql` — DDL (CREATE TABLE → column types)
- `<root>/db/queries/*.sql` — sqlc queries (filename→model, `-- name: Method :cardinality`)
- `<root>/api/openapi.yaml` — OpenAPI 3.0 (operationId=function name, x-pagination/sort/filter/include)
- `<root>/model/*.go` — Go interface→model methods, func→@func, `// @dto`→DTO without DDL table

## Codegen Features (gin framework)

Generated code uses **gin** framework (`c *gin.Context`):
- Function signature: `func Name(c *gin.Context)`
- Error responses: `c.JSON(status, gin.H{"error": "msg"})`
- Success responses: `c.JSON(http.StatusOK, gin.H{...})`
- Path params: `c.Param("Name")` + type conversion in function body
- Request body: `c.ShouldBindJSON(&req)` (2+ params) or `c.Query("Name")` (single param)
- currentUser: `c.MustGet("currentUser").(*model.CurrentUser)` — auto-generated when authorize or @param currentUser used

Additional features when symbol table (external SSOT) is available:

- **Type conversion**: DDL column type-based request param conversion (int64→`strconv.ParseInt`, time.Time→`time.Parse`, 400 early return on failure)
- **`-> column` mapping**: `@param PaymentMethod request -> method` — explicit DDL column mapping instead of auto-conversion. Also used for @func Request struct field mapping: `@param user.ID -> UserID`
- **Guard value types**: Zero value comparison based on result type (int→`== 0`/`> 0`, pointer→`== nil`/`!= nil`)
- **currentUser/config source**: `@param Name currentUser` → `currentUser.Name`
- **Stale data warning**: WARNING when response uses variable after put/delete without re-fetch
- **@dto tag**: `// @dto` annotated struct → skips DDL table matching
- **DDL FK/Index parsing**: REFERENCES (inline/constraint), CREATE INDEX → `DDLTable.ForeignKeys`, `DDLTable.Indexes`
- **QueryOpts auto-pass**: x-extensions present → `opts := QueryOpts{}` + `c.Query()` based parsing + `opts` arg appended to model call
- **List 3-tuple return**: many + QueryOpts → `result, total, err :=` (includes count)
- **Model interface derivation**: Crosses 3 SSOT sources → `<outDir>/model/models_gen.go`
  - sqlc: method names, cardinality (:one→`*T`, :many→`[]T`, :exec→`error`)
  - SSaC: all @param sources included (request, currentUser, dot notation `user.ID`→`userID`, literal `"pending"`→DDL reverse-mapping)
  - OpenAPI x-: infrastructure params (x-pagination → `opts QueryOpts` added)
- **Domain folder structure**: `service/auth/login.go` → `Domain="auth"` → `outDir/auth/login.go`, `package auth`. Flat backward compatible.
- **guard state codegen**: `guard state {id}` + `@param entity.Field` → `{id}state.CanTransition(entity.Field, "FuncName")`, import `"states/{id}state"`
- **@func codegen**: `@func auth.verifyPassword` → `auth.VerifyPassword(auth.VerifyPasswordRequest{...})`. @result absent → guard-style (401), @result present → value-style (500). `@result var Type.Field` → explicit field extraction (`out.Field`)
- **Spec file imports**: Parser collects Go import declarations from spec files and passes them to generated code. @func package name is the alias of the imported package.

Singularization rules (sqlc filename → model name): `ies`→`y`, `sses`→`ss`, `xes`→`x`, otherwise remove trailing `s`

## OpenAPI x- Extensions

Infrastructure parameters are declared in OpenAPI x- extensions. SSaC specs only declare business parameters; infrastructure parameters go in x- only.

```yaml
/api/reservations:
  get:
    operationId: ListReservations
    x-pagination:                    # Pagination
      style: offset                  # offset | cursor
      defaultLimit: 20
      maxLimit: 100
    x-sort:                          # Sorting
      allowed: [start_at, created_at]
      default: start_at
      direction: desc                # asc | desc
    x-filter:                        # Filtering
      allowed: [status, room_id]
    x-include:                       # Forward FK include (FK_column:ref_table.ref_column)
      allowed: [room_id:rooms.id, user_id:users.id]
```

Codegen effects:
- Operations with x- get `opts QueryOpts` parameter in model methods
- `:many` + x-pagination → return type includes total count: `([]T, int, error)`
- `QueryOpts` struct auto-generated (Limit, Offset, Cursor, SortCol, SortDir, Filters, Includes)

## Coding Conventions

- gofmt compliant, immediate error handling (early return)
- Filenames: snake_case, variables/functions: camelCase, types: PascalCase
- Tests: `go test ./parser/... ./validator/... ./generator/... -count=1`
