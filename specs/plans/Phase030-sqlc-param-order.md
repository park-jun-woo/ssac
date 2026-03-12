✅ 완료

# Phase030: buildArgsCodeFromInputs 알파벳순 정렬 → sqlc 파라미터 순서

## 목표

CRUD 모델 호출의 인자 순서를 sqlc 쿼리의 `$1, $2, $3` 파라미터 순서에 맞춘다.
현재 `buildArgsCodeFromInputs`는 Inputs key를 알파벳순 정렬하지만, 실제 모델 시그니처는 SQL 파라미터 순서를 따르므로 불일치가 발생한다.

## 핵심 아이디어

`loadSqlcQueries`에서 SQL 본문을 파싱하여 `$N ↔ 컬럼명` 매핑을 추출하고, `MethodInfo.Params`에 순서를 저장한다. 코드젠 시 이 순서를 참조하여 인자를 배치한다.

## SQL 파라미터 추출 패턴

실제 sqlc 쿼리는 3가지 정형 패턴만 존재:

### INSERT
```sql
INSERT INTO table (col1, col2, col3) VALUES ($1, $2, $3)
```
→ `$1=col1, $2=col2, $3=col3` (컬럼 목록과 VALUES 위치가 1:1)

### WHERE
```sql
WHERE col1 = $1 AND col2 = $2
```
→ `$1=col1, $2=col2` (`col = $N` 패턴)

### SET + WHERE
```sql
SET col1 = $1, col2 = $2 WHERE col3 = $3
```
→ `$1=col1, $2=col2, $3=col3` (SET절 + WHERE절 순서)

### 부등호 포함
```sql
WHERE room_id = $1 AND end_at > $2 AND start_at < $3
```
→ `$1=room_id, $2=end_at, $3=start_at` (`col > $N`, `col < $N` 패턴)

파싱 전략: 정규식 `(\w+)\s*[=<>!]+\s*\$(\d+)` + INSERT 컬럼 목록 위치 매칭.

## 변경 파일

| 파일 | 변경 내용 |
|---|---|
| `validator/symbol.go` | `loadSqlcQueries`에서 SQL 본문 파싱 → `MethodInfo.Params` 채우기 |
| `validator/symbol_test.go` | sqlc 파라미터 순서 파싱 테스트 (신규) |
| `generator/go_target.go` | `buildArgsCodeFromInputs`에 파라미터 순서 전달, `deriveInterfaces`도 동일 순서 사용 |
| `generator/generator_test.go` | 파라미터 순서 검증 테스트 |

## 상세 변경

### 1. `validator/symbol.go` — SQL 파라미터 순서 추출

#### `loadSqlcQueries` 수정

현재는 `-- name:` 줄만 파싱. SQL 본문도 읽어 `$N ↔ 컬럼명` 매핑 추출.

```go
func (st *SymbolTable) loadSqlcQueries(dir string) error {
    // ... 기존 파일 순회 ...
    scanner := bufio.NewScanner(f)
    var currentMethod string
    var currentSQL strings.Builder

    for scanner.Scan() {
        line := strings.TrimSpace(scanner.Text())
        if strings.HasPrefix(line, "-- name:") {
            // 이전 메서드의 SQL 처리
            if currentMethod != "" {
                params := extractSqlcParams(currentSQL.String())
                if mi, ok := ms.Methods[currentMethod]; ok && len(params) > 0 {
                    mi.Params = params
                    ms.Methods[currentMethod] = mi
                }
            }
            // 새 메서드 시작
            parts := strings.Fields(line)
            // ... 기존 파싱 ...
            currentMethod = methodName
            currentSQL.Reset()
        } else {
            currentSQL.WriteString(line + " ")
        }
    }
    // 마지막 메서드 처리
    if currentMethod != "" {
        params := extractSqlcParams(currentSQL.String())
        if mi, ok := ms.Methods[currentMethod]; ok && len(params) > 0 {
            mi.Params = params
            ms.Methods[currentMethod] = mi
        }
    }
}
```

#### `extractSqlcParams` 신규 함수

SQL 본문에서 `$N ↔ 컬럼명` 매핑을 추출하여 `$1, $2, $3` 순서의 `[]string` 반환.

```go
func extractSqlcParams(sql string) []string {
    sql = strings.ToUpper(sql) 체크 안 함 — 원본 유지

    // 1) INSERT 패턴: 컬럼 목록의 위치 = $N 위치
    if isInsert(sql) {
        return extractInsertParams(sql)
    }

    // 2) WHERE/SET 패턴: col = $N, col > $N, col < $N
    return extractWhereSetParams(sql)
}
```

**INSERT 파라미터 추출**:
```go
func extractInsertParams(sql string) []string {
    // "INSERT INTO table (col1, col2, col3)" → ["col1", "col2", "col3"]
    // 컬럼 순서 = $1, $2, $3 순서 (sqlc 규약)
    parenStart := strings.Index(sql, "(")
    parenEnd := strings.Index(sql, ")")
    cols := strings.Split(sql[parenStart+1:parenEnd], ",")
    // snake_case → PascalCase 변환
    var params []string
    for _, col := range cols {
        params = append(params, snakeToPascal(strings.TrimSpace(col)))
    }
    return params
}
```

**WHERE/SET 파라미터 추출**:
```go
func extractWhereSetParams(sql string) []string {
    // 정규식: (\w+)\s*[=<>!]+\s*\$(\d+)
    // $N 순서대로 정렬하여 반환
    re := regexp.MustCompile(`(\w+)\s*[=<>!]+\s*\$(\d+)`)
    matches := re.FindAllStringSubmatch(sql, -1)
    // $N 기준 정렬 → 컬럼명 snake→Pascal
}
```

컬럼명 변환: `snake_case` → `PascalCase` (SSaC Inputs key = PascalCase 컬럼명)

### 2. `generator/go_target.go` — 파라미터 순서 참조

#### `buildArgsCodeFromInputs` 시그니처 변경

```go
// before
func buildArgsCodeFromInputs(inputs map[string]string) string

// after
func buildArgsCodeFromInputs(inputs map[string]string, paramOrder []string) string
```

`paramOrder`가 있으면 그 순서로 배치, 없으면 기존 알파벳순 fallback.

```go
func buildArgsCodeFromInputs(inputs map[string]string, paramOrder []string) string {
    if len(inputs) == 0 {
        return ""
    }

    var keys []string
    if len(paramOrder) > 0 {
        // paramOrder 순서대로, inputs에 있는 키만 추출
        used := make(map[string]bool)
        for _, p := range paramOrder {
            if _, ok := inputs[p]; ok {
                keys = append(keys, p)
                used[p] = true
            }
        }
        // paramOrder에 없는 키 (query 등) → 마지막에 추가
        for k := range inputs {
            if !used[k] {
                keys = append(keys, k)
            }
        }
    } else {
        // fallback: 알파벳순 (심볼 테이블 없을 때)
        for k := range inputs {
            keys = append(keys, k)
        }
        sort.Strings(keys)
    }

    var parts []string
    for _, k := range keys {
        parts = append(parts, inputValueToCode(inputs[k]))
    }
    return strings.Join(parts, ", ")
}
```

#### `buildTemplateData` 호출부 수정

```go
case parser.SeqGet, parser.SeqPost, parser.SeqPut, parser.SeqDelete:
    var paramOrder []string
    if st != nil {
        paramOrder = lookupParamOrder(seq.Model, st)
    }
    d.ArgsCode = buildArgsCodeFromInputs(seq.Inputs, paramOrder)
```

```go
func lookupParamOrder(model string, st *validator.SymbolTable) []string {
    parts := strings.SplitN(model, ".", 2)
    if len(parts) < 2 {
        return nil
    }
    ms, ok := st.Models[parts[0]]
    if !ok {
        return nil
    }
    mi, ok := ms.Methods[parts[1]]
    if !ok {
        return nil
    }
    return mi.Params
}
```

#### `deriveInterfaces` 수정

```go
// line 1199-1203: 기존 알파벳순 → mi.Params 순서 사용
var inputKeys []string
if len(mi.Params) > 0 {
    // sqlc 파라미터 순서대로
    used := make(map[string]bool)
    for _, p := range mi.Params {
        if _, ok := usage.Inputs[p]; ok {
            inputKeys = append(inputKeys, p)
            used[p] = true
        }
    }
    for k := range usage.Inputs {
        if !used[k] {
            inputKeys = append(inputKeys, k)
        }
    }
} else {
    for k := range usage.Inputs {
        inputKeys = append(inputKeys, k)
    }
    sort.Strings(inputKeys)
}
```

이렇게 하면 models_gen.go의 인터페이스 파라미터 순서와 핸들러 호출 코드의 인자 순서가 모두 sqlc 파라미터 순서를 따른다.

### 3. Phase029 query 특수 처리 제거

`buildArgsCodeFromInputs`에서 `queryKey` 분리 로직을 제거한다. `query`는 models_gen.go에서 `opts QueryOpts`로 항상 마지막에 배치되며(`renderParams` + `HasQueryOpts`), sqlc 파라미터 순서에 `query`가 포함되지 않으므로 자연스럽게 마지막으로 간다.

### 4. 컬럼명 변환 유틸

`snake_case` → `PascalCase` 변환: `strcase.ToGoPascal` 사용 (이미 의존성 있음).
SSaC Inputs key와 매칭: `{UserID: currentUser.ID}` — key는 PascalCase.

## 테스트

### `validator/symbol_test.go` — 신규

```go
func TestExtractSqlcParamsInsert(t *testing.T) {
    sql := "INSERT INTO users (email, password_hash, name) VALUES ($1, $2, $3) RETURNING *;"
    params := extractSqlcParams(sql)
    // ["Email", "PasswordHash", "Name"] — snake→Pascal
}

func TestExtractSqlcParamsWhere(t *testing.T) {
    sql := "SELECT * FROM reservations WHERE room_id = $1 AND end_at > $2 AND start_at < $3 LIMIT 1;"
    params := extractSqlcParams(sql)
    // ["RoomID", "EndAt", "StartAt"] — $1, $2, $3 순서
}

func TestExtractSqlcParamsUpdate(t *testing.T) {
    sql := "UPDATE gigs SET status = $1 WHERE id = $2;"
    params := extractSqlcParams(sql)
    // ["Status", "ID"]
}
```

### `generator/generator_test.go`

```go
func TestGenerateArgsOrderMatchesSqlc(t *testing.T) {
    // DDL: INSERT INTO users (email, password_hash, name)
    // SSaC: @post User.Create({Name: request.name, Email: request.email, PasswordHash: hp.HashedPassword})
    // 기대: userModel.Create(email, hp.HashedPassword, name) — SQL 순서 (E, P, N)
    // 알파벳순이었다면: userModel.Create(email, hp.HashedPassword, name) — 동일 (E < N < P)
    //
    // 불일치 사례:
    // DDL: UPDATE gigs SET status = $1 WHERE id = $2
    // SSaC: @put Gig.UpdateStatus({ID: request.GigID, Status: "published"})
    // 기대: gigModel.UpdateStatus("published", gigID) — SQL 순서 ($1=status, $2=id)
    // 알파벳순이었다면: gigModel.UpdateStatus(gigID, "published") — 반대!
}
```

### 기존 테스트 수정

`TestGenerateWithQueryOpts`의 assertion은 Phase029에서 이미 `ListByUserID(currentUser.ID, opts)`로 수정됨. sqlc 파라미터 순서에서도 `query`가 마지막이므로 호환.

## 의존성

- `regexp` — `extractWhereSetParams`에서 사용 (표준 라이브러리)
- `github.com/ettle/strcase` — 이미 의존성 있음

## 검증

```bash
go test ./validator/... ./generator/... -count=1
```
