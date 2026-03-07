# SSaC — Service Sequences as Code

A CLI tool that parses declarative service logic from Go comments and generates Go implementation code.

```
specs/service/*.go  →  ssac validate  →  ssac gen  →  artifacts/service/*.go
   (comment DSL)        (validation)      (codegen)     (gofmt applied)
```

## Core Idea

Declare the execution flow inside service functions using **10 fixed sequence types**, and let symbolic codegen produce the implementation. No LLM required — runs purely on templates.

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
go build -o ssac ./artifacts/cmd/ssac

ssac parse [dir]       # Print parsed sequence structure
ssac validate [dir]    # Internal + external SSOT cross-validation
ssac gen               # validate → codegen → gofmt
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

```bash
ssac validate specs/dummy-study      # With external validation
ssac validate specs/backend/service  # Internal validation only
```

## Project Structure

```
specs/                           # Declarations (SSOT)
  backend/service/               #   Example specs
  dummy-study/                   #   Study room reservation demo project
    service/  db/queries/  api/  model/
artifacts/                       # Output (code)
  cmd/ssac/                      #   CLI entrypoint
  internal/parser/               #   Comments → []ServiceFunc
  internal/generator/            #   Type-based templates → Go code
  internal/validator/            #   Internal + external validation
  MANUAL.md                      #   Detailed manual
```

## External Validation Project Layout

```
<project>/
  service/*.go            # Sequence specs
  db/queries/*.sql        # sqlc queries (-- name: Method :type)
  api/openapi.yaml        # OpenAPI 3.0 (operationId = function name)
  model/*.go              # Go interface (component), func
```

## Tests

```bash
go test ./artifacts/internal/... -v
```

46 tests: parser 14 + generator 4 + validator 28

## License

MIT
