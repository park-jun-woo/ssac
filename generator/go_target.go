package generator

import (
	"bytes"
	"fmt"
	"go/format"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/geul-org/ssac/parser"
	"github.com/geul-org/ssac/validator"
)

// GoTarget은 Go 언어용 코드 생성기다.
type GoTarget struct{}

// FileExtension은 Go 파일 확장자를 반환한다.
func (g *GoTarget) FileExtension() string { return ".go" }

// GenerateFunc는 단일 ServiceFunc의 Go 코드를 생성한다.
// st가 non-nil이면 DDL 타입 기반 변환 코드를 생성한다.
func (g *GoTarget) GenerateFunc(sf parser.ServiceFunc, st *validator.SymbolTable) ([]byte, error) {
	var buf bytes.Buffer

	// path parameter 결정
	var pathParams []validator.PathParam
	if st != nil {
		if op, ok := st.Operations[sf.Name]; ok {
			pathParams = op.PathParams
		}
	}
	pathParamSet := map[string]bool{}
	for _, pp := range pathParams {
		pathParamSet[pp.Name] = true
	}

	// request 파라미터 타입 결정 (path param은 제외)
	typedParams := collectTypedRequestParams(sf.Sequences, st, pathParamSet)
	imports := collectImports(sf.Sequences, typedParams)

	// package
	pkgName := "service"
	if sf.Domain != "" {
		pkgName = sf.Domain
	}
	buf.WriteString("package " + pkgName + "\n\n")

	// imports
	if len(imports) > 0 {
		buf.WriteString("import (\n")
		for _, imp := range imports {
			fmt.Fprintf(&buf, "\t%q\n", imp)
		}
		buf.WriteString(")\n\n")
	}

	// func signature
	sig := "func %s(w http.ResponseWriter, r *http.Request"
	if len(pathParams) > 0 {
		var ppArgs []string
		for _, pp := range pathParams {
			ppArgs = append(ppArgs, fmt.Sprintf("%s %s", lcFirst(pp.Name), pp.GoType))
		}
		sig += ", " + strings.Join(ppArgs, ", ")
	}
	sig += ") {\n"
	fmt.Fprintf(&buf, sig, sf.Name)

	// request 파라미터 추출 (타입 변환 포함, path param 제외)
	for _, tp := range typedParams {
		buf.WriteString(tp.extractCode)
	}
	if len(typedParams) > 0 {
		buf.WriteString("\n")
	}

	// result 타입 맵 구축 (guard 비교식 결정용)
	resultTypes := map[string]string{}
	for _, seq := range sf.Sequences {
		if seq.Result != nil {
			resultTypes[seq.Result.Var] = seq.Result.Type
		}
	}

	// QueryOpts 구성 코드 생성: List 메서드가 있고 QueryOpts가 존재할 때만
	if st != nil && hasAnyQueryOpts(st) {
		needsOpts := false
		for _, seq := range sf.Sequences {
			if seq.Type == parser.SeqGet && isListMethod(seq.Model) {
				needsOpts = true
				break
			}
		}
		if needsOpts {
			buf.WriteString("\topts := QueryOpts{}\n\n")
		}
	}

	// sequence 블록 생성
	// 타입 변환 코드에서 err를 선언했으면 이미 선언된 것으로 처리
	errDeclared := hasConversionErr(typedParams)
	funcHasTotal := false
	for i, seq := range sf.Sequences {
		data := buildTemplateData(seq, &errDeclared, resultTypes, st, sf.Name)
		if data.HasTotal {
			funcHasTotal = true
		}
		// response 시퀀스에 funcHasTotal 전달
		if strings.HasPrefix(seq.Type, "response") {
			data.HasTotal = funcHasTotal
		}

		tmplName := templateName(seq)
		var seqBuf bytes.Buffer
		if err := goTemplates.ExecuteTemplate(&seqBuf, tmplName, data); err != nil {
			return nil, fmt.Errorf("sequence[%d] %s 템플릿 실행 실패: %w", i, seq.Type, err)
		}
		buf.Write(seqBuf.Bytes())
		buf.WriteString("\n")
	}

	buf.WriteString("}\n")

	// gofmt
	formatted, err := format.Source(buf.Bytes())
	if err != nil {
		return buf.Bytes(), fmt.Errorf("gofmt 실패: %w\n--- raw ---\n%s", err, buf.String())
	}
	return formatted, nil
}

// GenerateModelInterfaces는 심볼 테이블과 SSaC spec을 교차하여 Model interface를 생성한다.
func (g *GoTarget) GenerateModelInterfaces(funcs []parser.ServiceFunc, st *validator.SymbolTable, outDir string) error {
	modelDir := filepath.Join(outDir, "model")
	if err := os.MkdirAll(modelDir, 0755); err != nil {
		return fmt.Errorf("model 디렉토리 생성 실패: %w", err)
	}

	// SSaC spec에서 사용된 모델+메서드 수집
	usages := collectModelUsages(funcs)

	// 사용된 모델만 interface 생성
	interfaces := deriveInterfaces(usages, st)
	if len(interfaces) == 0 {
		return nil
	}

	code := renderInterfaces(interfaces, hasQueryOpts(st))

	formatted, err := format.Source(code)
	if err != nil {
		return fmt.Errorf("models_gen.go gofmt 실패: %w\n--- raw ---\n%s", err, string(code))
	}

	path := filepath.Join(modelDir, "models_gen.go")
	return os.WriteFile(path, formatted, 0644)
}

// --- Go 코드젠 내부 함수 ---

// templateData는 템플릿에 전달하는 데이터다.
type templateData struct {
	// 공통
	Message string
	// get, post, put, delete
	ModelVar    string
	ModelMethod string
	ParamArgs   string
	Result      *parser.Result
	// guard
	Target      string
	ZeroCheck   string // "== nil", "== 0", `== ""`, "== false"
	ExistsCheck string // "!= nil", "> 0", `!= ""`, "== true"
	// authorize
	Action   string
	Resource string
	ID       string
	// password
	Hash  string
	Plain string
	// call
	Component       string
	ComponentMethod string
	Func            string
	FirstErr        bool
	// response
	Vars []string
	// list
	HasTotal bool // many + QueryOpts → 3-tuple 반환
	// guard state
	Entity      string // @param의 entity 부분 (e.g. "course")
	StatusField string // @param의 field 부분 (e.g. "Published")
	FuncName    string // 현재 함수명 (e.g. "PublishCourse")
}

func buildTemplateData(seq parser.Sequence, errDeclared *bool, resultTypes map[string]string, st *validator.SymbolTable, funcName string) templateData {
	d := templateData{
		Message: seq.Message,
		Result:  seq.Result,
		Vars:    seq.Vars,
	}

	// 모델 분리: "Project.FindByID" → "projectModel", "FindByID"
	if seq.Model != "" {
		parts := strings.SplitN(seq.Model, ".", 2)
		d.ModelVar = lcFirst(parts[0]) + "Model"
		if len(parts) > 1 {
			d.ModelMethod = parts[1]
		}
	}

	// 기본 메시지 생성
	if d.Message == "" {
		d.Message = defaultMessage(seq)
	}

	// 파라미터 인자 문자열
	d.ParamArgs = buildParamArgs(seq.Params)

	// QueryOpts 자동 추가: List 메서드에만
	if st != nil && seq.Type == parser.SeqGet && isListMethod(seq.Model) && hasAnyQueryOpts(st) {
		if d.ParamArgs != "" {
			d.ParamArgs += ", "
		}
		d.ParamArgs += "opts"
		if seq.Result != nil && strings.HasPrefix(seq.Result.Type, "[]") {
			d.HasTotal = true
		}
	}

	// guard 대상 + 타입별 비교식
	d.Target = seq.Target
	if seq.Type == parser.SeqGuardState && len(seq.Params) > 0 {
		parts := strings.SplitN(seq.Params[0].Name, ".", 2)
		d.Entity = parts[0]
		if len(parts) > 1 {
			d.StatusField = parts[1]
		}
		d.FuncName = funcName
	}
	if seq.Type == parser.SeqGuardNil || seq.Type == parser.SeqGuardExists {
		typeName := resultTypes[seq.Target]
		d.ZeroCheck, d.ExistsCheck = zeroValueChecks(typeName)
	}

	// authorize
	d.Action = seq.Action
	d.Resource = seq.Resource
	d.ID = resolveParamRef(seq.ID)

	// password
	if seq.Type == parser.SeqPassword && len(seq.Params) >= 2 {
		d.Hash = resolveParamRef(seq.Params[0].Name)
		d.Plain = resolveParamRef(seq.Params[1].Name)
	}

	// call
	d.Component = seq.Component
	d.ComponentMethod = "Execute"
	d.Func = seq.Func

	// err 선언 추적
	switch seq.Type {
	case parser.SeqAuthorize, parser.SeqGet, parser.SeqPost:
		d.FirstErr = true
		*errDeclared = true
	case parser.SeqCall:
		if seq.Result != nil {
			d.FirstErr = true
			*errDeclared = true
		} else if !*errDeclared {
			d.FirstErr = true
			*errDeclared = true
		}
	case parser.SeqPut, parser.SeqDelete:
		if !*errDeclared {
			d.FirstErr = true
			*errDeclared = true
		}
	}

	return d
}

func templateName(seq parser.Sequence) string {
	if seq.Type == parser.SeqCall {
		if seq.Component != "" {
			return "call_component"
		}
		return "call_func"
	}
	if strings.HasPrefix(seq.Type, "response") {
		return seq.Type
	}
	return seq.Type
}

// typedRequestParam은 request 파라미터의 타입과 추출 코드를 보관한다.
type typedRequestParam struct {
	name        string // PascalCase 원본명
	goType      string // "string", "int64", "time.Time" 등
	extractCode string // 추출 코드 (줄바꿈 포함)
}

// collectTypedRequestParams는 source가 "request"인 파라미터를 수집하고 DDL 타입을 결정한다.
// pathParamSet에 포함된 파라미터는 함수 인자로 이미 받으므로 제외한다.
// request 파라미터가 2개 이상이면 JSON body로 간주하여 struct decode 코드를 생성한다.
func collectTypedRequestParams(seqs []parser.Sequence, st *validator.SymbolTable, pathParamSet map[string]bool) []typedRequestParam {
	seen := map[string]bool{}
	var rawParams []struct {
		name   string
		goType string
	}
	for _, seq := range seqs {
		for _, p := range seq.Params {
			if p.Source != "request" || seen[p.Name] || pathParamSet[p.Name] {
				continue
			}
			seen[p.Name] = true

			goType := "string"
			if st != nil {
				goType = lookupDDLType(p.Name, st)
			}
			rawParams = append(rawParams, struct {
				name   string
				goType string
			}{p.Name, goType})
		}
	}

	// 심볼 테이블이 있고 request 파라미터가 2개 이상이면 JSON body struct decode
	if st != nil && len(rawParams) >= 2 {
		return buildJSONBodyParams(rawParams)
	}

	// 1개 이하면 기존 FormValue 방식
	var params []typedRequestParam
	for _, rp := range rawParams {
		varName := lcFirst(rp.name)
		code := generateExtractCode(varName, rp.name, rp.goType)
		params = append(params, typedRequestParam{
			name:        rp.name,
			goType:      rp.goType,
			extractCode: code,
		})
	}
	return params
}

// buildJSONBodyParams는 JSON body struct decode + 변수 추출 코드를 생성한다.
func buildJSONBodyParams(rawParams []struct {
	name   string
	goType string
}) []typedRequestParam {
	var buf bytes.Buffer

	// struct 정의
	buf.WriteString("\tvar req struct {\n")
	for _, rp := range rawParams {
		jsonTag := toSnakeCase(rp.name)
		buf.WriteString(fmt.Sprintf("\t\t%s %s `json:\"%s\"`\n", rp.name, rp.goType, jsonTag))
	}
	buf.WriteString("\t}\n")

	// decode
	buf.WriteString("\tif err := json.NewDecoder(r.Body).Decode(&req); err != nil {\n")
	buf.WriteString("\t\thttp.Error(w, \"invalid request body\", http.StatusBadRequest)\n")
	buf.WriteString("\t\treturn\n")
	buf.WriteString("\t}\n")

	// 변수 추출
	for _, rp := range rawParams {
		varName := lcFirst(rp.name)
		buf.WriteString(fmt.Sprintf("\t%s := req.%s\n", varName, rp.name))
	}

	// 전체 코드를 json_body 엔트리에 담고, time.Time import 힌트도 추가
	result := []typedRequestParam{{
		name:        "_json_body",
		goType:      "json_body",
		extractCode: buf.String(),
	}}
	// time.Time은 struct 필드에 사용되므로 import 필요 (strconv 등은 불필요)
	for _, rp := range rawParams {
		if rp.goType == "time.Time" {
			result = append(result, typedRequestParam{
				name:   rp.name,
				goType: rp.goType,
			})
			break
		}
	}
	return result
}

// lookupDDLType은 PascalCase 파라미터명을 snake_case로 변환하여 DDL 컬럼 타입을 조회한다.
func lookupDDLType(paramName string, st *validator.SymbolTable) string {
	snakeName := toSnakeCase(paramName)
	for _, table := range st.DDLTables {
		if goType, ok := table.Columns[snakeName]; ok {
			return goType
		}
	}
	return "string"
}

// generateExtractCode는 타입별 request 파라미터 추출 코드를 생성한다.
func generateExtractCode(varName, paramName, goType string) string {
	switch goType {
	case "int64":
		return fmt.Sprintf("\t%s, err := strconv.ParseInt(r.FormValue(%q), 10, 64)\n"+
			"\tif err != nil {\n"+
			"\t\thttp.Error(w, \"%s: 유효하지 않은 값\", http.StatusBadRequest)\n"+
			"\t\treturn\n"+
			"\t}\n", varName, paramName, paramName)
	case "float64":
		return fmt.Sprintf("\t%s, err := strconv.ParseFloat(r.FormValue(%q), 64)\n"+
			"\tif err != nil {\n"+
			"\t\thttp.Error(w, \"%s: 유효하지 않은 값\", http.StatusBadRequest)\n"+
			"\t\treturn\n"+
			"\t}\n", varName, paramName, paramName)
	case "bool":
		return fmt.Sprintf("\t%s, err := strconv.ParseBool(r.FormValue(%q))\n"+
			"\tif err != nil {\n"+
			"\t\thttp.Error(w, \"%s: 유효하지 않은 값\", http.StatusBadRequest)\n"+
			"\t\treturn\n"+
			"\t}\n", varName, paramName, paramName)
	case "time.Time":
		return fmt.Sprintf("\t%s, err := time.Parse(time.RFC3339, r.FormValue(%q))\n"+
			"\tif err != nil {\n"+
			"\t\thttp.Error(w, \"%s: 유효하지 않은 값\", http.StatusBadRequest)\n"+
			"\t\treturn\n"+
			"\t}\n", varName, paramName, paramName)
	default: // string
		return fmt.Sprintf("\t%s := r.FormValue(%q)\n", varName, paramName)
	}
}

// collectImports는 사용된 패키지를 수집한다.
func collectImports(seqs []parser.Sequence, typedParams []typedRequestParam) []string {
	seen := map[string]bool{
		"net/http": true, // 항상 사용
	}

	for _, seq := range seqs {
		switch {
		case strings.HasPrefix(seq.Type, "response json"):
			seen["encoding/json"] = true
		case seq.Type == parser.SeqPassword:
			seen["golang.org/x/crypto/bcrypt"] = true
		case seq.Type == parser.SeqGuardState:
			seen["states/"+seq.Target+"state"] = true
		}
	}

	for _, tp := range typedParams {
		switch tp.goType {
		case "int64", "float64", "bool":
			seen["strconv"] = true
		case "time.Time":
			seen["time"] = true
		case "json_body":
			seen["encoding/json"] = true
		}
	}

	var imports []string
	order := []string{"encoding/json", "net/http", "strconv", "time", "golang.org/x/crypto/bcrypt"}
	for _, imp := range order {
		if seen[imp] {
			imports = append(imports, imp)
			delete(seen, imp)
		}
	}
	// 동적 import (states/*state 등)
	for imp := range seen {
		imports = append(imports, imp)
	}
	return imports
}

// buildParamArgs는 Param 슬라이스를 함수 호출 인자 문자열로 변환한다.
func buildParamArgs(params []parser.Param) string {
	var args []string
	for _, p := range params {
		args = append(args, resolveParam(p))
	}
	return strings.Join(args, ", ")
}

// resolveParam은 Param의 source를 고려하여 Go 표현식으로 변환한다.
func resolveParam(p parser.Param) string {
	if p.Source == "currentUser" || p.Source == "config" {
		return p.Source + "." + p.Name
	}
	return resolveParamRef(p.Name)
}

// resolveParamRef는 파라미터 참조를 Go 표현식으로 변환한다.
func resolveParamRef(name string) string {
	if name == "" {
		return ""
	}
	if name == "new" {
		return "nil"
	}
	if strings.HasPrefix(name, `"`) {
		return name
	}
	if strings.Contains(name, ".") {
		return name
	}
	return lcFirst(name)
}

func defaultMessage(seq parser.Sequence) string {
	modelName := ""
	if seq.Model != "" {
		parts := strings.SplitN(seq.Model, ".", 2)
		modelName = parts[0]
	}

	switch seq.Type {
	case parser.SeqGet:
		return modelName + " 조회 실패"
	case parser.SeqPost:
		return modelName + " 생성 실패"
	case parser.SeqPut:
		return modelName + " 수정 실패"
	case parser.SeqDelete:
		return modelName + " 삭제 실패"
	case parser.SeqGuardNil:
		return seq.Target + "가 존재하지 않습니다"
	case parser.SeqGuardExists:
		return seq.Target + "가 이미 존재합니다"
	case parser.SeqGuardState:
		return "상태 전이가 허용되지 않습니다"
	case parser.SeqAuthorize:
		return "권한 확인 실패"
	case parser.SeqPassword:
		return "비밀번호가 일치하지 않습니다"
	case parser.SeqCall:
		if seq.Component != "" {
			return seq.Component + " 호출 실패"
		}
		if seq.Func != "" {
			return seq.Func + " 호출 실패"
		}
		return "호출 실패"
	}
	return "처리 실패"
}

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

// isListMethod는 모델 메서드명이 List로 시작하는지 확인한다.
func isListMethod(model string) bool {
	parts := strings.SplitN(model, ".", 2)
	if len(parts) < 2 {
		return false
	}
	return strings.HasPrefix(parts[1], "List")
}

// hasAnyQueryOpts는 심볼 테이블에 QueryOpts를 가진 operation이 있는지 확인한다.
func hasAnyQueryOpts(st *validator.SymbolTable) bool {
	if st == nil {
		return false
	}
	for _, op := range st.Operations {
		if op.HasQueryOpts() {
			return true
		}
	}
	return false
}

func hasConversionErr(params []typedRequestParam) bool {
	for _, p := range params {
		if p.goType != "string" && p.goType != "json_body" {
			return true
		}
	}
	return false
}

// --- Model 인터페이스 파생 ---

type modelUsage struct {
	ModelName  string
	MethodName string
	Params     []parser.Param
	Result     *parser.Result
	FuncName   string
}

func collectModelUsages(funcs []parser.ServiceFunc) []modelUsage {
	var usages []modelUsage
	for _, sf := range funcs {
		for _, seq := range sf.Sequences {
			if seq.Model == "" {
				continue
			}
			parts := strings.SplitN(seq.Model, ".", 2)
			if len(parts) < 2 {
				continue
			}
			usages = append(usages, modelUsage{
				ModelName:  parts[0],
				MethodName: parts[1],
				Params:     seq.Params,
				Result:     seq.Result,
				FuncName:   sf.Name,
			})
		}
	}
	return usages
}

type derivedInterface struct {
	Name    string
	Methods []derivedMethod
}

type derivedMethod struct {
	Name         string
	Params       []derivedParam
	HasQueryOpts bool
	ReturnType   string
}

type derivedParam struct {
	Name   string
	GoType string
}

func deriveInterfaces(usages []modelUsage, st *validator.SymbolTable) []derivedInterface {
	type methodKey struct {
		model, method string
	}
	methodMap := map[methodKey]modelUsage{}
	modelNames := map[string]bool{}

	for _, u := range usages {
		key := methodKey{u.ModelName, u.MethodName}
		if _, exists := methodMap[key]; !exists {
			methodMap[key] = u
			modelNames[u.ModelName] = true
		}
	}

	var interfaces []derivedInterface
	sortedModels := sortedKeys(modelNames)

	for _, modelName := range sortedModels {
		ms, ok := st.Models[modelName]
		if !ok {
			continue
		}

		iface := derivedInterface{Name: modelName + "Model"}

		var usedMethods []string
		for _, u := range usages {
			if u.ModelName == modelName {
				found := false
				for _, m := range usedMethods {
					if m == u.MethodName {
						found = true
						break
					}
				}
				if !found {
					usedMethods = append(usedMethods, u.MethodName)
				}
			}
		}
		sort.Strings(usedMethods)

		for _, methodName := range usedMethods {
			mi, methodExists := ms.Methods[methodName]
			if !methodExists {
				mi = validator.MethodInfo{}
			}
			key := methodKey{modelName, methodName}
			usage := methodMap[key]

			dm := derivedMethod{Name: methodName}

			// 이름 있는 파라미터의 snake_case 수집 (리터럴 역매핑용)
			usedColumns := map[string]bool{}
			tableName := toSnakeCase(modelName) + "s"
			for _, p := range usage.Params {
				if !strings.HasPrefix(p.Name, `"`) {
					if strings.Contains(p.Name, ".") {
						parts := strings.SplitN(p.Name, ".", 2)
						usedColumns[toSnakeCase(parts[1])] = true
						// 복합 컬럼명도 추가: enrollment.ID → enrollment_id
						usedColumns[toSnakeCase(parts[0])+"_"+toSnakeCase(parts[1])] = true
					} else {
						snake := toSnakeCase(p.Name)
						usedColumns[snake] = true
						// 변수 참조: DDL 컬럼과 부분 매칭으로 제외
						if p.Source == "" {
							if table, ok := st.DDLTables[tableName]; ok {
								if _, exists := table.Columns[snake]; !exists {
									words := splitCamelWords(p.Name)
									for col := range table.Columns {
										for _, w := range words {
											if strings.Contains(col, strings.ToLower(w)) {
												usedColumns[col] = true
											}
										}
									}
								}
							}
						}
					}
				}
			}

			for _, p := range usage.Params {
				dp := derivedParam{
					Name:   resolveParamName(p),
					GoType: resolveParamType(p, usage.ModelName, st),
				}
				// 리터럴 파라미터: DDL 역매핑으로 이름 결정
				if strings.HasPrefix(p.Name, `"`) {
					dp.Name = resolveLiteralParamName(modelName, usedColumns, st)
					if dp.Name != "" {
						usedColumns[toSnakeCase(dp.Name)] = true
					}
				}
				dm.Params = append(dm.Params, dp)
			}

			if op, ok := st.Operations[usage.FuncName]; ok && op.HasQueryOpts() {
				dm.HasQueryOpts = true
			}

			dm.ReturnType = deriveReturnType(mi, usage, dm.HasQueryOpts)

			iface.Methods = append(iface.Methods, dm)
		}

		if len(iface.Methods) > 0 {
			interfaces = append(interfaces, iface)
		}
	}

	return interfaces
}

func resolveParamName(p parser.Param) string {
	name := p.Name
	if strings.HasPrefix(name, `"`) {
		return "" // 리터럴 → deriveInterfaces에서 DDL 역매핑으로 이름 결정
	}
	if strings.Contains(name, ".") {
		// "enrollment.ID" → "enrollmentID", "course.Price" → "coursePrice"
		parts := strings.SplitN(name, ".", 2)
		field := parts[1]
		if len(field) > 0 {
			field = strings.ToUpper(field[:1]) + field[1:]
		}
		return parts[0] + field
	}
	return lcFirst(name)
}

func resolveParamType(p parser.Param, modelName string, st *validator.SymbolTable) string {
	if strings.HasPrefix(p.Name, `"`) {
		return "string"
	}

	// dot notation: "enrollment.ID" → enrollments 테이블의 id 컬럼
	if strings.Contains(p.Name, ".") {
		parts := strings.SplitN(p.Name, ".", 2)
		refTable := toSnakeCase(parts[0]) + "s"
		refCol := toSnakeCase(parts[1])
		if table, ok := st.DDLTables[refTable]; ok {
			if goType, ok := table.Columns[refCol]; ok {
				return goType
			}
		}
		return "string"
	}

	snakeName := toSnakeCase(p.Name)

	// 1. 해당 모델의 테이블에서 직접 조회
	tableName := toSnakeCase(modelName) + "s"
	if table, ok := st.DDLTables[tableName]; ok {
		if goType, ok := table.Columns[snakeName]; ok {
			return goType
		}
	}

	// 2. {Model}ID 패턴: CourseID → courses.id
	if strings.HasSuffix(p.Name, "ID") {
		refModel := p.Name[:len(p.Name)-2]
		refTable := toSnakeCase(refModel) + "s"
		if table, ok := st.DDLTables[refTable]; ok {
			if goType, ok := table.Columns["id"]; ok {
				return goType
			}
		}
	}

	// 3. 전체 테이블 순회
	for _, table := range st.DDLTables {
		if goType, ok := table.Columns[snakeName]; ok {
			return goType
		}
	}

	return "string"
}

// resolveLiteralParamName은 리터럴 파라미터의 이름을 DDL 역매핑으로 결정한다.
// 모델 테이블에서 이미 사용된 컬럼을 제외하고 DDL 정의 순서로 첫 번째 string 컬럼을 선택한다.
func resolveLiteralParamName(modelName string, usedColumns map[string]bool, st *validator.SymbolTable) string {
	tableName := toSnakeCase(modelName) + "s"
	table, ok := st.DDLTables[tableName]
	if !ok {
		return ""
	}

	autoColumns := map[string]bool{
		"id": true, "created_at": true, "updated_at": true, "deleted_at": true,
	}

	// DDL 정의 순서로 순회
	for _, col := range table.ColumnOrder {
		goType := table.Columns[col]
		if autoColumns[col] || usedColumns[col] || goType != "string" {
			continue
		}
		return lcFirst(snakeToCamel(col))
	}
	return ""
}

// splitCamelWords는 camelCase 문자열을 단어로 분리한다.
// "hashedPassword" → ["hashed", "Password"]
func splitCamelWords(s string) []string {
	var words []string
	start := 0
	for i := 1; i < len(s); i++ {
		if s[i] >= 'A' && s[i] <= 'Z' {
			words = append(words, s[start:i])
			start = i
		}
	}
	words = append(words, s[start:])
	return words
}

// snakeToCamel은 snake_case를 camelCase로 변환한다.
func snakeToCamel(s string) string {
	parts := strings.Split(s, "_")
	for i := range parts {
		if len(parts[i]) > 0 {
			parts[i] = strings.ToUpper(parts[i][:1]) + parts[i][1:]
		}
	}
	return strings.Join(parts, "")
}

func deriveReturnType(mi validator.MethodInfo, usage modelUsage, hasQueryOpts bool) string {
	switch mi.Cardinality {
	case "exec":
		return "error"
	case "many":
		typeName := "interface{}"
		if usage.Result != nil {
			typeName = usage.Result.Type
			if strings.HasPrefix(typeName, "[]") {
				typeName = typeName[2:]
			}
		}
		if hasQueryOpts {
			return fmt.Sprintf("([]%s, int, error)", typeName)
		}
		return fmt.Sprintf("([]%s, error)", typeName)
	default:
		typeName := "interface{}"
		if usage.Result != nil {
			typeName = usage.Result.Type
		}
		return fmt.Sprintf("(*%s, error)", typeName)
	}
}

func renderInterfaces(interfaces []derivedInterface, needQueryOpts bool) []byte {
	var buf bytes.Buffer
	buf.WriteString("package model\n\n")

	if needsTimeImport(interfaces) {
		buf.WriteString("import \"time\"\n\n")
	}

	for _, iface := range interfaces {
		fmt.Fprintf(&buf, "type %s interface {\n", iface.Name)
		for _, m := range iface.Methods {
			params := renderParams(m.Params)
			if m.HasQueryOpts {
				if params != "" {
					params += ", "
				}
				params += "opts QueryOpts"
			}
			fmt.Fprintf(&buf, "\t%s(%s) %s\n", m.Name, params, m.ReturnType)
		}
		buf.WriteString("}\n\n")
	}

	if needQueryOpts {
		buf.WriteString(`type QueryOpts struct {
	Limit    int
	Offset   int
	Cursor   string
	SortCol  string
	SortDir  string
	Filters  map[string]string
	Includes []string
}
`)
	}

	return buf.Bytes()
}

func renderParams(params []derivedParam) string {
	var parts []string
	for _, p := range params {
		if p.Name == "" {
			continue
		}
		parts = append(parts, p.Name+" "+p.GoType)
	}
	return strings.Join(parts, ", ")
}

func hasQueryOpts(st *validator.SymbolTable) bool {
	for _, op := range st.Operations {
		if op.HasQueryOpts() {
			return true
		}
	}
	return false
}

func needsTimeImport(interfaces []derivedInterface) bool {
	for _, iface := range interfaces {
		for _, m := range iface.Methods {
			for _, p := range m.Params {
				if p.GoType == "time.Time" {
					return true
				}
			}
		}
	}
	return false
}

func sortedKeys(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func sortedMethodKeys(m map[string]validator.MethodInfo) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
