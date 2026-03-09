✅ 완료

# Phase 012: generator gin 프레임워크 전환

## 목표

SSaC generator의 Go 코드 생성을 `net/http` 기반에서 `gin` 프레임워크 기반으로 전환한다.
parser/validator는 변경 없음. **generator만 변경**.

## 변경 사항

### 1. 함수 시그니처 변경

**`go_target.go`**: `GenerateFunc`의 함수 시그니처 생성 부분

현재:
```go
func Login(w http.ResponseWriter, r *http.Request) {
func GetCourse(w http.ResponseWriter, r *http.Request, courseID int64) {
```

변경:
```go
func Login(c *gin.Context) {
func GetCourse(c *gin.Context) {
```

- path param을 함수 인자에서 제거
- `w http.ResponseWriter, r *http.Request` → `c *gin.Context`

### 2. 경로 파라미터 추출 (함수 본문 상단)

현재: path param이 함수 인자로 전달됨 (glue-gen 라우터가 파싱)

변경: 함수 본문 상단에서 `c.Param()` + 타입 변환 코드 생성

```go
// int64 타입
courseIDStr := c.Param("CourseID")
courseID, err := strconv.ParseInt(courseIDStr, 10, 64)
if err != nil {
    c.JSON(http.StatusBadRequest, gin.H{"error": "invalid path parameter"})
    return
}

// string 타입
slug := c.Param("Slug")
```

**`go_target.go`**: `GenerateFunc`에서 pathParams를 시그니처 대신 본문 코드로 생성

### 3. 요청 본문 파싱 변경

**`go_target.go`**:

- `buildJSONBodyParams`: `json.NewDecoder(r.Body).Decode(&req)` → `c.ShouldBindJSON(&req)`
- `generateExtractCode`: `r.FormValue(...)` → `c.Query(...)` (1개 이하 request param)
- JSON body decode 에러: `http.Error(w, ...)` → `c.JSON(http.StatusBadRequest, gin.H{...})`
- 타입 변환 에러도 동일하게 `c.JSON` 패턴

### 4. 에러 응답 변경

**`go_templates.go`**: 모든 템플릿의 `http.Error(w, "msg", status)` → `c.JSON(status, gin.H{"error": "msg"})`

| 템플릿 | 변경 |
|--------|------|
| `authorize` | `http.Error(w, ...)` → `c.JSON(status, gin.H{...})` (2곳: 내부에러 + Forbidden) |
| `get` | `http.Error` → `c.JSON` |
| `guard nil` | `http.Error` → `c.JSON` |
| `guard exists` | `http.Error` → `c.JSON` |
| `guard state` | `http.Error` → `c.JSON` |
| `post` | `http.Error` → `c.JSON` |
| `put` | `http.Error` → `c.JSON` |
| `delete` | `http.Error` → `c.JSON` |
| `call_component` | `http.Error` → `c.JSON` |
| `call_func` | `http.Error` → `c.JSON` (`{{.FuncErrStatus}}` 유지) |

### 5. 성공 응답 변경

**`go_templates.go`**: `response json` 템플릿

현재:
```go
w.Header().Set("Content-Type", "application/json")
json.NewEncoder(w).Encode(map[string]interface{}{...})
```

변경:
```go
c.JSON(http.StatusOK, gin.H{
    "var": var,
})
```

### 6. authorize 시퀀스 — currentUser 추출

**`go_templates.go`**: `authorize` 템플릿에 currentUser 추출 코드 추가

```go
currentUser := c.MustGet("currentUser").(*model.CurrentUser)
```

**`go_target.go`**: currentUser 추출은 함수 내에서 한 번만 생성.
- authorize가 있으면 authorize 블록 상단에서 생성
- authorize 없이 `@param ... currentUser`만 있으면 첫 사용 전에 생성
- `needsCurrentUser` 플래그로 관리

### 7. import 변경

**`go_target.go`**: `collectImports` 수정

- `"github.com/gin-gonic/gin"` 항상 포함
- `"net/http"` 유지 (status code 상수 사용)
- `"encoding/json"` 트리거 변경:
  - `response json`에서 더 이상 필요 없음 (`c.JSON` 사용)
  - `json_body`에서도 불필요 (`c.ShouldBindJSON` 사용)
  - → `"encoding/json"` import 트리거 제거

### 8. QueryOpts 파싱

**`go_target.go`**: QueryOpts 구성 코드 변경

현재: `opts := QueryOpts{}` 빈 struct만 생성

변경: `c.Query()`/`c.DefaultQuery()`로 query param 파싱 코드 생성
```go
opts := QueryOpts{}
if v := c.Query("limit"); v != "" {
    opts.Limit, _ = strconv.Atoi(v)
}
if v := c.Query("offset"); v != "" {
    opts.Offset, _ = strconv.Atoi(v)
}
if v := c.Query("cursor"); v != "" {
    opts.Cursor = v
}
if v := c.Query("sort"); v != "" {
    opts.Sort = v
}
```

## 변경 파일 목록

| 파일 | 변경 |
|---|---|
| `generator/go_target.go` | 함수 시그니처 gin 전환, path param 본문 추출, `buildJSONBodyParams` ShouldBindJSON, `generateExtractCode` c.Query, `collectImports` gin 추가 + encoding/json 제거, QueryOpts c.Query 파싱, currentUser 추출 |
| `generator/go_templates.go` | 전 템플릿 `http.Error` → `c.JSON`, authorize에 currentUser 추출, `response json` → `c.JSON(200, gin.H{...})` |
| `generator/generator_test.go` | 전 테스트 기대값 gin 패턴으로 업데이트 |
| `testdata/backend-service/*.go` | spec 파일 자체는 변경 없음 (parser 미변경) |

## 의존성

- 없음 (gin은 생성 코드의 의존성이지 ssac 자체의 의존성이 아님)

## 검증

```bash
go test ./parser/... ./validator/... ./generator/... -count=1
```

## 테스트 계획

### generator_test.go 업데이트

모든 기존 테스트의 기대값을 gin 패턴으로 변경:

- `TestGenerateCreateSession`: `func CreateSession(c *gin.Context)`, `c.Query(...)`, `c.JSON(http.StatusOK, gin.H{...})`
- `TestGenerateDeleteProject`: `func DeleteProject(c *gin.Context)`, `c.JSON` 에러 패턴
- `TestGenerateTypedRequestParams`: `c.ShouldBindJSON`, `c.JSON` 에러, path param은 `c.Param()`
- `TestGeneratePathParamSignature`: `func GetReservation(c *gin.Context)` (path param이 시그니처에 없음), 본문에 `c.Param("ReservationID")`
- `TestGenerateQueryOptsAndTotal`: `c.Query("limit")` 등 QueryOpts 파싱 코드 확인
- `TestGenerateCustomMessages`: `c.JSON` 패턴의 커스텀 메시지
- `TestGenerateGuardState`: `c.JSON(http.StatusConflict, ...)` 패턴
