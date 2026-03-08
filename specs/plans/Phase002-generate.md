# Phase 2: Generate

> 파싱된 `[]ServiceFunc`를 타입별 템플릿으로 매칭하여 Go 구현 코드를 생성한다.

## 목표

Phase 1의 산출물(`[]ServiceFunc`)을 입력받아, 각 sequence 타입에 대응하는 Go 코드를 생성하고 `artifacts/backend/internal/service/`에 파일로 출력한다.

## 입력

Phase 1의 `[]ServiceFunc`

## 산출

```
artifacts/backend/internal/service/<snake_case>.go
```

파일당 하나의 서비스 함수 구현 코드. `gofmt` 적용 완료 상태.

## 타입별 템플릿 (10종)

### authorize

```go
// authorize
allowed, err := authz.Check(currentUser, "{{.Action}}", "{{.Resource}}", {{.ID}})
if err != nil {
    http.Error(w, "{{.Message}}", http.StatusInternalServerError)
    return
}
if !allowed {
    http.Error(w, "권한이 없습니다", http.StatusForbidden)
    return
}
```

### get

```go
// get
{{.Result.Var}}, err := {{.ModelVar}}.{{.ModelMethod}}({{.ParamArgs}})
if err != nil {
    http.Error(w, "{{.Message}}", http.StatusInternalServerError)
    return
}
```

### guard nil

```go
// guard nil
if {{.Target}} == nil {
    http.Error(w, "{{.Message}}", http.StatusNotFound)
    return
}
```

### guard exists

```go
// guard exists
if {{.Target}} != nil {
    http.Error(w, "{{.Message}}", http.StatusConflict)
    return
}
```

### post

```go
// post
{{.Result.Var}}, err := {{.ModelVar}}.{{.ModelMethod}}({{.ParamArgs}})
if err != nil {
    http.Error(w, "{{.Message}}", http.StatusInternalServerError)
    return
}
```

### put

```go
// put
err = {{.ModelVar}}.{{.ModelMethod}}({{.ParamArgs}})
if err != nil {
    http.Error(w, "{{.Message}}", http.StatusInternalServerError)
    return
}
```

### delete

```go
// delete
err = {{.ModelVar}}.{{.ModelMethod}}({{.ParamArgs}})
if err != nil {
    http.Error(w, "{{.Message}}", http.StatusInternalServerError)
    return
}
```

### password

```go
// password
if err := bcrypt.CompareHashAndPassword([]byte({{.Hash}}), []byte({{.Plain}})); err != nil {
    http.Error(w, "{{.Message}}", http.StatusUnauthorized)
    return
}
```

### call

#### @component

```go
// call component
{{if .Result}}{{.Result.Var}}, {{end}}err {{if .FirstErr}}:={{else}}={{end}} {{.Component}}.{{.ComponentMethod}}({{.ParamArgs}})
if err != nil {
    http.Error(w, "{{.Message}}", http.StatusInternalServerError)
    return
}
```

#### @func

```go
// call func
{{if .Result}}{{.Result.Var}}, {{end}}err {{if .FirstErr}}:={{else}}={{end}} {{.Func}}({{.ParamArgs}})
if err != nil {
    http.Error(w, "{{.Message}}", http.StatusInternalServerError)
    return
}
```

### response

#### json

```go
// response json
w.Header().Set("Content-Type", "application/json")
json.NewEncoder(w).Encode(map[string]interface{}{
    {{range .Vars}}"{{.}}": {{.}},
    {{end}}
})
```

## 구현 계획

### 1. 디렉토리 구조

```
artifacts/
  internal/
    generator/
      generator.go      # 메인 생성 로직
      templates.go       # 타입별 템플릿 정의
    generator_test.go    # 테스트
```

### 2. 템플릿 엔진

- `text/template`으로 타입별 템플릿 등록
- 각 Sequence를 템플릿 데이터로 변환하는 어댑터 함수

### 3. 코드 생성 흐름

```
ServiceFunc 순회
  → 파일 헤더(package, import) 생성
  → 함수 시그니처 생성
  → Sequences 순회: 타입별 템플릿 실행, 코드 블록 누적
  → 파일 쓰기
  → gofmt 적용 (go/format 패키지)
```

### 4. 보조 로직

- `@model Project.FindByID` → modelVar: `projectModel`, method: `FindByID`
- `@param ProjectID request` → `r.FormValue("ProjectID")` 또는 경로 파라미터 추출
- `@message` 기본값 자동 생성 (타입 + 모델명 조합)
- import 자동 수집 (사용된 패키지만 포함)

### 5. CLI 연동

- `ssac gen` 명령: parse → generate 파이프라인 실행
- 입력 디렉토리, 출력 디렉토리 플래그

### 6. 테스트

- CreateSession, DeleteProject 예시의 코드젠 결과를 기획서 기대값과 비교

## 완료 기준

- [ ] 10종 타입 모두 템플릿 구현
- [ ] `ssac gen` 실행 시 `artifacts/backend/internal/service/`에 Go 파일 생성
- [ ] 생성된 코드가 `gofmt` 통과
- [ ] 기획서 예시 2개의 코드젠 결과가 기대값과 일치
