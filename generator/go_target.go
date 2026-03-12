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
	"github.com/ettle/strcase"
)

// GoTarget은 Go 언어용 코드 생성기다.
type GoTarget struct{}

// FileExtension은 Go 파일 확장자를 반환한다.
func (g *GoTarget) FileExtension() string { return ".go" }

// GenerateFunc는 단일 ServiceFunc의 Go 코드를 생성한다.
func (g *GoTarget) GenerateFunc(sf parser.ServiceFunc, st *validator.SymbolTable) ([]byte, error) {
	if sf.Subscribe != nil {
		return g.generateSubscribeFunc(sf, st)
	}
	return g.generateHTTPFunc(sf, st)
}

// generateHTTPFunc는 HTTP 핸들러 함수를 생성한다.
func (g *GoTarget) generateHTTPFunc(sf parser.ServiceFunc, st *validator.SymbolTable) ([]byte, error) {
	var buf bytes.Buffer

	// 분석
	pathParams := getPathParams(sf.Name, st)
	pathParamSet := map[string]bool{}
	for _, pp := range pathParams {
		pathParamSet[pp.Name] = true
	}

	requestParams := collectRequestParams(sf.Sequences, st, pathParamSet)
	needsCU := needsCurrentUser(sf.Sequences)
	needsQO := needsQueryOpts(sf, st)
	imports := collectImports(sf, requestParams, pathParams, needsCU, needsQO)

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
	fmt.Fprintf(&buf, "func %s(c *gin.Context) {\n", sf.Name)

	// path parameters
	for _, pp := range pathParams {
		buf.WriteString(generatePathParamCode(pp))
	}
	if len(pathParams) > 0 {
		buf.WriteString("\n")
	}

	// currentUser
	if needsCU {
		var cuBuf bytes.Buffer
		goTemplates.ExecuteTemplate(&cuBuf, "currentUser", nil)
		buf.Write(cuBuf.Bytes())
		buf.WriteString("\n")
	}

	// request parameters
	for _, rp := range requestParams {
		buf.WriteString(rp.extractCode)
	}
	if len(requestParams) > 0 {
		buf.WriteString("\n")
	}

	// QueryOpts
	if needsQO {
		buf.WriteString(generateQueryOptsCode(st))
		buf.WriteString("\n")
	}

	// result types for guard checks
	resultTypes := map[string]string{}
	for _, seq := range sf.Sequences {
		if seq.Result != nil {
			resultTypes[seq.Result.Var] = seq.Result.Type
		}
	}

	// sequences
	errDeclared := hasConversionErr(requestParams)
	declaredVars := map[string]bool{}
	funcHasTotal := false
	usedVars := collectUsedVars(sf.Sequences)
	for i, seq := range sf.Sequences {
		data := buildTemplateData(seq, &errDeclared, declaredVars, resultTypes, st, sf.Name)
		if data.HasTotal {
			funcHasTotal = true
		}
		if seq.Type == parser.SeqResponse {
			data.HasTotal = funcHasTotal
		}
		// 미사용 변수 처리
		if seq.Result != nil && !usedVars[seq.Result.Var] {
			data.Unused = true
			if data.ErrDeclared {
				data.ReAssign = true // _, err = (no new vars with :=)
			}
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

	formatted, err := format.Source(buf.Bytes())
	if err != nil {
		return buf.Bytes(), fmt.Errorf("gofmt 실패: %w\n--- raw ---\n%s", err, buf.String())
	}
	return formatted, nil
}

// generateSubscribeFunc는 큐 구독 핸들러 함수를 생성한다.
func (g *GoTarget) generateSubscribeFunc(sf parser.ServiceFunc, st *validator.SymbolTable) ([]byte, error) {
	var buf bytes.Buffer

	pkgName := "service"
	if sf.Domain != "" {
		pkgName = sf.Domain
	}
	buf.WriteString("package " + pkgName + "\n\n")

	imports := collectSubscribeImports(sf)
	if len(imports) > 0 {
		buf.WriteString("import (\n")
		for _, imp := range imports {
			fmt.Fprintf(&buf, "\t%q\n", imp)
		}
		buf.WriteString(")\n\n")
	}

	msgType := sf.Subscribe.MessageType
	fmt.Fprintf(&buf, "func %s(ctx context.Context, message %s) error {\n", sf.Name, msgType)

	resultTypes := map[string]string{}
	for _, seq := range sf.Sequences {
		if seq.Result != nil {
			resultTypes[seq.Result.Var] = seq.Result.Type
		}
	}

	errDeclared := false
	declaredVars := map[string]bool{}
	usedVars := collectUsedVars(sf.Sequences)
	for i, seq := range sf.Sequences {
		data := buildTemplateData(seq, &errDeclared, declaredVars, resultTypes, st, sf.Name)
		// 미사용 변수 처리
		if seq.Result != nil && !usedVars[seq.Result.Var] {
			data.Unused = true
			if data.ErrDeclared {
				data.ReAssign = true // _, err = (no new vars with :=)
			}
		}
		tmplName := subscribeTemplateName(seq)
		var seqBuf bytes.Buffer
		if err := goTemplates.ExecuteTemplate(&seqBuf, tmplName, data); err != nil {
			return nil, fmt.Errorf("sequence[%d] %s 템플릿 실행 실패: %w", i, seq.Type, err)
		}
		buf.Write(seqBuf.Bytes())
		buf.WriteString("\n")
	}

	buf.WriteString("\treturn nil\n")
	buf.WriteString("}\n")

	formatted, err := format.Source(buf.Bytes())
	if err != nil {
		return buf.Bytes(), fmt.Errorf("gofmt 실패: %w\n--- raw ---\n%s", err, buf.String())
	}
	return formatted, nil
}

// subscribeTemplateName은 subscribe 함수 내 시퀀스의 템플릿 이름을 반환한다.
func subscribeTemplateName(seq parser.Sequence) string {
	switch seq.Type {
	case parser.SeqCall:
		if seq.Result != nil {
			return "sub_call_with_result"
		}
		return "sub_call_no_result"
	case parser.SeqPublish:
		return "sub_publish"
	default:
		return "sub_" + seq.Type
	}
}

// collectSubscribeImports는 subscribe 함수에 필요한 import를 수집한다.
func collectSubscribeImports(sf parser.ServiceFunc) []string {
	seen := map[string]bool{
		"context": true,
		"fmt":     true,
	}
	for _, seq := range sf.Sequences {
		if seq.Type == parser.SeqState {
			seen["states/"+seq.DiagramID+"state"] = true
		}
		if seq.Type == parser.SeqAuth {
			seen["authz"] = true
		}
		if seq.Type == parser.SeqPublish {
			seen["queue"] = true
		}
	}
	if needsCurrentUser(sf.Sequences) {
		seen["model"] = true
	}
	for _, imp := range sf.Imports {
		seen[imp] = true
	}
	var imports []string
	order := []string{"context", "fmt"}
	for _, imp := range order {
		if seen[imp] {
			imports = append(imports, imp)
			delete(seen, imp)
		}
	}
	var dynamic []string
	for imp := range seen {
		dynamic = append(dynamic, imp)
	}
	sort.Strings(dynamic)
	imports = append(imports, dynamic...)
	return imports
}

// GenerateModelInterfaces는 심볼 테이블과 SSaC spec을 교차하여 Model interface를 생성한다.
func (g *GoTarget) GenerateModelInterfaces(funcs []parser.ServiceFunc, st *validator.SymbolTable, outDir string) error {
	modelDir := filepath.Join(outDir, "model")
	if err := os.MkdirAll(modelDir, 0755); err != nil {
		return fmt.Errorf("model 디렉토리 생성 실패: %w", err)
	}

	usages := collectModelUsages(funcs)
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

// --- templateData ---

type templateData struct {
	// 공통
	Message  string
	FirstErr bool

	// get/post/put/delete
	ModelCall string // "courseModel.FindByID"
	ArgsCode string // "courseID, currentUser.ID"
	Result   *parser.Result

	// empty/exists
	Target      string
	ZeroCheck   string
	ExistsCheck string

	// state
	DiagramID   string
	Transition  string
	InputFields string // "Status: reservation.Status, ..."

	// auth
	Action   string
	Resource string

	// call
	PkgName    string
	FuncMethod string
	ErrStatus  string // "http.StatusInternalServerError", "http.StatusUnauthorized" 등

	// publish
	Topic      string // "order.completed"
	OptionCode string // ", queue.WithDelay(1800)" 또는 ""

	// response
	ResponseFields map[string]string

	// list
	HasTotal bool

	// reassign: result var already declared → use = instead of :=
	ReAssign bool

	// unused: result var not referenced later → use _ instead of var name
	Unused bool

	// errDeclared: err variable already declared before this sequence
	ErrDeclared bool
}

func buildTemplateData(seq parser.Sequence, errDeclared *bool, declaredVars map[string]bool, resultTypes map[string]string, st *validator.SymbolTable, funcName string) templateData {
	d := templateData{
		Message: seq.Message,
		Result:  seq.Result,
	}

	// Model call
	if seq.Model != "" {
		parts := strings.SplitN(seq.Model, ".", 2)
		if seq.Type == parser.SeqCall {
			d.PkgName = parts[0]
			if len(parts) > 1 {
				d.FuncMethod = strcase.ToGoPascal(parts[1])
			}
			if seq.ErrStatus != 0 {
				d.ErrStatus = httpStatusConst(seq.ErrStatus)
			} else {
				d.ErrStatus = "http.StatusInternalServerError"
			}
		} else {
			// 패키지 접두사 모델이든 일반 모델이든 동일: modelName + "Model." + method
			d.ModelCall = strcase.ToGoCamel(parts[0]) + "Model." + parts[1]
		}
	}

	// Default message
	if d.Message == "" {
		d.Message = defaultMessage(seq)
	}

	// Args/Inputs → code
	switch seq.Type {
	case parser.SeqGet, parser.SeqPost, parser.SeqPut, parser.SeqDelete:
		// CRUD: Inputs value만 positional로 변환 (심볼 테이블 파라미터 순서 참조)
		var paramOrder []string
		if st != nil {
			paramOrder = lookupParamOrder(seq.Model, st)
		}
		d.ArgsCode = buildArgsCodeFromInputs(seq.Inputs, paramOrder)
	default:
		d.ArgsCode = buildArgsCode(seq.Args)
	}

	// query arg → HasTotal (List + query → 3-tuple return), Wrapper 타입이면 제외
	if hasQueryInput(seq.Inputs) && seq.Result != nil && strings.HasPrefix(seq.Result.Type, "[]") && seq.Result.Wrapper == "" {
		d.HasTotal = true
	}

	// Guard
	d.Target = seq.Target
	if seq.Type == parser.SeqEmpty || seq.Type == parser.SeqExists {
		typeName := resultTypes[rootVar(seq.Target)]
		d.ZeroCheck, d.ExistsCheck = zeroValueChecks(typeName)
	}

	// State
	d.DiagramID = seq.DiagramID
	d.Transition = seq.Transition

	// Auth
	d.Action = seq.Action
	d.Resource = seq.Resource

	// ErrStatus (empty, exists, state, auth)
	switch seq.Type {
	case parser.SeqEmpty:
		if seq.ErrStatus != 0 {
			d.ErrStatus = httpStatusConst(seq.ErrStatus)
		} else {
			d.ErrStatus = "http.StatusNotFound"
		}
	case parser.SeqExists:
		if seq.ErrStatus != 0 {
			d.ErrStatus = httpStatusConst(seq.ErrStatus)
		} else {
			d.ErrStatus = "http.StatusConflict"
		}
	case parser.SeqState:
		if seq.ErrStatus != 0 {
			d.ErrStatus = httpStatusConst(seq.ErrStatus)
		} else {
			d.ErrStatus = "http.StatusConflict"
		}
	case parser.SeqAuth:
		if seq.ErrStatus != 0 {
			d.ErrStatus = httpStatusConst(seq.ErrStatus)
		} else {
			d.ErrStatus = "http.StatusForbidden"
		}
	}

	// Inputs → InputFields (for state, auth, call)
	if seq.Type == parser.SeqState || seq.Type == parser.SeqAuth || seq.Type == parser.SeqCall {
		if len(seq.Inputs) > 0 {
			inputs := seq.Inputs
			// @auth + currentUser 참조 → Role 자동 추가
			if seq.Type == parser.SeqAuth {
				if _, hasRole := inputs["Role"]; !hasRole && hasCurrentUserRef(inputs) {
					inputs = copyInputs(inputs)
					inputs["Role"] = "currentUser.Role"
				}
			}
			d.InputFields = buildInputFieldsFromMap(inputs)
		}
	}

	// Publish
	if seq.Type == parser.SeqPublish {
		d.Topic = seq.Topic
		d.InputFields = buildPublishPayload(seq.Inputs)
		d.OptionCode = buildPublishOptions(seq.Options)
	}

	// Response
	d.ResponseFields = seq.Fields

	// result var reassign tracking
	if seq.Result != nil {
		if declaredVars[seq.Result.Var] {
			d.ReAssign = true
		}
		declaredVars[seq.Result.Var] = true
	}

	// capture errDeclared state before this sequence modifies it
	d.ErrDeclared = *errDeclared

	// err declaration tracking
	switch seq.Type {
	case parser.SeqGet, parser.SeqPost:
		d.FirstErr = true
		*errDeclared = true
	case parser.SeqAuth:
		if !*errDeclared {
			d.FirstErr = true
			*errDeclared = true
		}
	case parser.SeqCall:
		if seq.Result != nil {
			d.FirstErr = true
			*errDeclared = true
		} else if !*errDeclared {
			d.FirstErr = true
			*errDeclared = true
		}
	case parser.SeqPut, parser.SeqDelete, parser.SeqPublish:
		if !*errDeclared {
			d.FirstErr = true
			*errDeclared = true
		}
	}

	return d
}

func templateName(seq parser.Sequence) string {
	switch seq.Type {
	case parser.SeqResponse:
		if seq.Target != "" {
			return "response_direct"
		}
		return "response"
	case parser.SeqCall:
		if seq.Result != nil {
			return "call_with_result"
		}
		return "call_no_result"
	case parser.SeqPublish:
		return "publish"
	default:
		return seq.Type
	}
}

// --- Args → Go code ---

func buildArgsCode(args []parser.Arg) string {
	var parts []string
	for _, a := range args {
		parts = append(parts, argToCode(a))
	}
	return strings.Join(parts, ", ")
}

func argToCode(a parser.Arg) string {
	if a.Literal != "" {
		return `"` + a.Literal + `"`
	}
	if a.Source == "query" {
		return "opts"
	}
	if a.Source == "request" {
		return strcase.ToGoCamel(a.Field)
	}
	if a.Source == "currentUser" {
		return a.Source + "." + a.Field
	}
	if a.Source != "" {
		if a.Field == "" {
			return a.Source
		}
		return a.Source + "." + a.Field
	}
	return a.Field
}

// buildInputFieldsFromMap은 map[string]string을 Go struct 리터럴 필드로 변환한다.
func buildInputFieldsFromMap(inputs map[string]string) string {
	keys := make([]string, 0, len(inputs))
	for k := range inputs {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var fields []string
	for _, k := range keys {
		fields = append(fields, strcase.ToGoPascal(k)+": "+inputValueToCode(inputs[k]))
	}
	return strings.Join(fields, ", ")
}

// hasCurrentUserRef는 inputs 값 중 currentUser. 참조가 있는지 확인한다.
func hasCurrentUserRef(inputs map[string]string) bool {
	for _, v := range inputs {
		if strings.HasPrefix(v, "currentUser.") {
			return true
		}
	}
	return false
}

// copyInputs는 inputs map의 얕은 복사본을 만든다.
func copyInputs(inputs map[string]string) map[string]string {
	cp := make(map[string]string, len(inputs)+1)
	for k, v := range inputs {
		cp[k] = v
	}
	return cp
}

// inputValueToCode는 inputs 값에 argToCode와 동일한 예약 소스 변환을 적용한다.
func inputValueToCode(val string) string {
	if val == "query" {
		return "opts"
	}
	if strings.HasPrefix(val, "request.") {
		return strcase.ToGoCamel(val[len("request."):])
	}
	// currentUser.Field, 일반 변수 → 그대로
	return val
}

// buildArgsCodeFromInputs는 Inputs map의 value만 추출하여 positional 함수 인자로 변환한다.
// paramOrder가 있으면 그 순서로 배치하고, 없으면 알파벳순 fallback.
func buildArgsCodeFromInputs(inputs map[string]string, paramOrder []string) string {
	if len(inputs) == 0 {
		return ""
	}

	var keys []string
	if len(paramOrder) > 0 {
		used := make(map[string]bool)
		for _, p := range paramOrder {
			if _, ok := inputs[p]; ok {
				keys = append(keys, p)
				used[p] = true
			}
		}
		// paramOrder에 없는 키 (query 등) → 마지막에 추가
		var extra []string
		for k := range inputs {
			if !used[k] {
				extra = append(extra, k)
			}
		}
		sort.Strings(extra)
		keys = append(keys, extra...)
	} else {
		// fallback: 알파벳순, query는 마지막
		var queryKey string
		for k := range inputs {
			if inputs[k] == "query" {
				queryKey = k
			} else {
				keys = append(keys, k)
			}
		}
		sort.Strings(keys)
		if queryKey != "" {
			keys = append(keys, queryKey)
		}
	}

	var parts []string
	for _, k := range keys {
		parts = append(parts, inputValueToCode(inputs[k]))
	}
	return strings.Join(parts, ", ")
}

// lookupParamOrder는 심볼 테이블에서 모델 메서드의 파라미터 순서를 조회한다.
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

// buildPublishPayload는 publish의 Inputs를 map[string]any 리터럴 필드로 변환한다.
func buildPublishPayload(inputs map[string]string) string {
	keys := make([]string, 0, len(inputs))
	for k := range inputs {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var fields []string
	for _, k := range keys {
		fields = append(fields, fmt.Sprintf("\t\t%q: %s,", strcase.ToGoPascal(k), inputValueToCode(inputs[k])))
	}
	return strings.Join(fields, "\n")
}

// buildPublishOptions는 publish의 Options를 Go 코드로 변환한다.
func buildPublishOptions(options map[string]string) string {
	if len(options) == 0 {
		return ""
	}
	var parts []string
	keys := make([]string, 0, len(options))
	for k := range options {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		switch k {
		case "delay":
			parts = append(parts, fmt.Sprintf("queue.WithDelay(%s)", options[k]))
		case "priority":
			parts = append(parts, fmt.Sprintf("queue.WithPriority(%q)", options[k]))
		}
	}
	if len(parts) == 0 {
		return ""
	}
	return ", " + strings.Join(parts, ", ")
}

// hasQueryInput은 Inputs map에 query 예약 소스가 있는지 확인한다.
func hasQueryInput(inputs map[string]string) bool {
	for _, v := range inputs {
		if v == "query" {
			return true
		}
	}
	return false
}


// --- request parameter extraction ---

type typedRequestParam struct {
	name        string
	goType      string
	extractCode string
}

func collectRequestParams(seqs []parser.Sequence, st *validator.SymbolTable, pathParamSet map[string]bool) []typedRequestParam {
	seen := map[string]bool{}
	var rawParams []struct {
		name   string
		goType string
	}

	for _, seq := range seqs {
		for _, a := range seq.Args {
			if a.Source != "request" || seen[a.Field] || pathParamSet[a.Field] {
				continue
			}
			seen[a.Field] = true
			goType := "string"
			if st != nil {
				goType = lookupDDLType(a.Field, st)
			}
			rawParams = append(rawParams, struct {
				name   string
				goType string
			}{a.Field, goType})
		}
		// Also check Inputs for request references
		for _, val := range seq.Inputs {
			if strings.HasPrefix(val, "request.") {
				field := val[len("request."):]
				if !seen[field] && !pathParamSet[field] {
					seen[field] = true
					goType := "string"
					if st != nil {
						goType = lookupDDLType(field, st)
					}
					rawParams = append(rawParams, struct {
						name   string
						goType string
					}{field, goType})
				}
			}
		}
	}

	// POST/PUT 핸들러이거나 2+ params → JSON body
	hasBodySeq := false
	for _, seq := range seqs {
		if seq.Type == parser.SeqPost || seq.Type == parser.SeqPut {
			hasBodySeq = true
			break
		}
	}
	if (st != nil && len(rawParams) >= 2) || (hasBodySeq && len(rawParams) >= 1) {
		return buildJSONBodyParams(rawParams)
	}

	var params []typedRequestParam
	for _, rp := range rawParams {
		varName := strcase.ToGoCamel(rp.name)
		code := generateExtractCode(varName, rp.name, rp.goType)
		params = append(params, typedRequestParam{
			name:        rp.name,
			goType:      rp.goType,
			extractCode: code,
		})
	}
	return params
}

func buildJSONBodyParams(rawParams []struct {
	name   string
	goType string
}) []typedRequestParam {
	var buf bytes.Buffer

	buf.WriteString("\tvar req struct {\n")
	for _, rp := range rawParams {
		buf.WriteString(fmt.Sprintf("\t\t%s %s `json:\"%s\"`\n", strcase.ToGoPascal(rp.name), rp.goType, rp.name))
	}
	buf.WriteString("\t}\n")
	buf.WriteString("\tif err := c.ShouldBindJSON(&req); err != nil {\n")
	buf.WriteString("\t\tc.JSON(http.StatusBadRequest, gin.H{\"error\": \"invalid request body\"})\n")
	buf.WriteString("\t\treturn\n")
	buf.WriteString("\t}\n")
	for _, rp := range rawParams {
		varName := strcase.ToGoCamel(rp.name)
		buf.WriteString(fmt.Sprintf("\t%s := req.%s\n", varName, strcase.ToGoPascal(rp.name)))
	}

	result := []typedRequestParam{{
		name:        "_json_body",
		goType:      "json_body",
		extractCode: buf.String(),
	}}
	for _, rp := range rawParams {
		if rp.goType == "time.Time" {
			result = append(result, typedRequestParam{name: rp.name, goType: rp.goType})
			break
		}
	}
	return result
}

func lookupDDLType(paramName string, st *validator.SymbolTable) string {
	snakeName := toSnakeCase(paramName)
	for _, table := range st.DDLTables {
		if goType, ok := table.Columns[snakeName]; ok {
			return goType
		}
	}
	return "string"
}

func generateExtractCode(varName, paramName, goType string) string {
	switch goType {
	case "int64":
		return fmt.Sprintf("\t%s, err := strconv.ParseInt(c.Query(%q), 10, 64)\n"+
			"\tif err != nil {\n"+
			"\t\tc.JSON(http.StatusBadRequest, gin.H{\"error\": \"%s: 유효하지 않은 값\"})\n"+
			"\t\treturn\n"+
			"\t}\n", varName, paramName, paramName)
	case "float64":
		return fmt.Sprintf("\t%s, err := strconv.ParseFloat(c.Query(%q), 64)\n"+
			"\tif err != nil {\n"+
			"\t\tc.JSON(http.StatusBadRequest, gin.H{\"error\": \"%s: 유효하지 않은 값\"})\n"+
			"\t\treturn\n"+
			"\t}\n", varName, paramName, paramName)
	case "bool":
		return fmt.Sprintf("\t%s, err := strconv.ParseBool(c.Query(%q))\n"+
			"\tif err != nil {\n"+
			"\t\tc.JSON(http.StatusBadRequest, gin.H{\"error\": \"%s: 유효하지 않은 값\"})\n"+
			"\t\treturn\n"+
			"\t}\n", varName, paramName, paramName)
	case "time.Time":
		return fmt.Sprintf("\t%s, err := time.Parse(time.RFC3339, c.Query(%q))\n"+
			"\tif err != nil {\n"+
			"\t\tc.JSON(http.StatusBadRequest, gin.H{\"error\": \"%s: 유효하지 않은 값\"})\n"+
			"\t\treturn\n"+
			"\t}\n", varName, paramName, paramName)
	default:
		return fmt.Sprintf("\t%s := c.Query(%q)\n", varName, paramName)
	}
}

func generatePathParamCode(pp validator.PathParam) string {
	varName := strcase.ToGoCamel(pp.Name)
	switch pp.GoType {
	case "int64":
		return fmt.Sprintf("\t%sStr := c.Param(%q)\n"+
			"\t%s, err := strconv.ParseInt(%sStr, 10, 64)\n"+
			"\tif err != nil {\n"+
			"\t\tc.JSON(http.StatusBadRequest, gin.H{\"error\": \"invalid path parameter\"})\n"+
			"\t\treturn\n"+
			"\t}\n", varName, pp.Name, varName, varName)
	case "float64":
		return fmt.Sprintf("\t%sStr := c.Param(%q)\n"+
			"\t%s, err := strconv.ParseFloat(%sStr, 64)\n"+
			"\tif err != nil {\n"+
			"\t\tc.JSON(http.StatusBadRequest, gin.H{\"error\": \"invalid path parameter\"})\n"+
			"\t\treturn\n"+
			"\t}\n", varName, pp.Name, varName, varName)
	default:
		return fmt.Sprintf("\t%s := c.Param(%q)\n", varName, pp.Name)
	}
}

// --- imports ---

func collectImports(sf parser.ServiceFunc, reqParams []typedRequestParam, pathParams []validator.PathParam, needsCU bool, needsQO bool) []string {
	seen := map[string]bool{
		"net/http":                  true,
		"github.com/gin-gonic/gin": true,
	}

	for _, seq := range sf.Sequences {
		if seq.Type == parser.SeqState {
			seen["states/"+seq.DiagramID+"state"] = true
		}
		if seq.Type == parser.SeqAuth {
			seen["authz"] = true
		}
		if seq.Type == parser.SeqPublish {
			seen["queue"] = true
		}
		if seq.Result != nil && seq.Result.Wrapper != "" {
			// 간단쓰기(@response varName)면 handler에서 pagination 직접 참조 없음
			hasDirectResponse := false
			for _, s := range sf.Sequences {
				if s.Type == parser.SeqResponse && s.Target != "" {
					hasDirectResponse = true
					break
				}
			}
			if !hasDirectResponse {
				seen["github.com/geul-org/fullend/pkg/pagination"] = true
			}
		}
	}

	for _, tp := range reqParams {
		switch tp.goType {
		case "int64", "float64", "bool":
			seen["strconv"] = true
		case "time.Time":
			seen["time"] = true
		}
	}

	hasNonStringPathParam := false
	for _, pp := range pathParams {
		if pp.GoType != "string" {
			hasNonStringPathParam = true
			break
		}
	}
	if hasNonStringPathParam || needsQO {
		seen["strconv"] = true
	}

	if needsCU {
		seen["model"] = true
	}

	var imports []string
	order := []string{"net/http", "strconv", "time"}
	for _, imp := range order {
		if seen[imp] {
			imports = append(imports, imp)
			delete(seen, imp)
		}
	}
	var dynamic []string
	for imp := range seen {
		dynamic = append(dynamic, imp)
	}
	sort.Strings(dynamic)
	imports = append(imports, dynamic...)

	for _, imp := range sf.Imports {
		imports = append(imports, imp)
	}
	return imports
}

// --- helpers ---

func needsCurrentUser(seqs []parser.Sequence) bool {
	for _, seq := range seqs {
		for _, a := range seq.Args {
			if a.Source == "currentUser" {
				return true
			}
		}
		for _, val := range seq.Inputs {
			if strings.HasPrefix(val, "currentUser.") {
				return true
			}
		}
	}
	return false
}

// collectUsedVars는 시퀀스에서 참조되는 변수명을 수집한다.
func collectUsedVars(seqs []parser.Sequence) map[string]bool {
	used := map[string]bool{}
	for _, seq := range seqs {
		// Guard Target
		if seq.Target != "" {
			used[rootVar(seq.Target)] = true
		}
		// Inputs values
		for _, val := range seq.Inputs {
			if strings.HasPrefix(val, "request.") || strings.HasPrefix(val, "currentUser.") ||
				strings.HasPrefix(val, `"`) || val == "query" {
				continue
			}
			used[rootVar(val)] = true
		}
		// Response Fields values
		for _, val := range seq.Fields {
			if !strings.HasPrefix(val, `"`) {
				used[rootVar(val)] = true
			}
		}
	}
	return used
}

func needsQueryOpts(sf parser.ServiceFunc, st *validator.SymbolTable) bool {
	for _, seq := range sf.Sequences {
		if hasQueryInput(seq.Inputs) {
			return true
		}
	}
	return false
}

func getPathParams(funcName string, st *validator.SymbolTable) []validator.PathParam {
	if st == nil {
		return nil
	}
	if op, ok := st.Operations[funcName]; ok {
		return op.PathParams
	}
	return nil
}

func generateQueryOptsCode(st *validator.SymbolTable) string {
	var buf bytes.Buffer
	buf.WriteString("\topts := QueryOpts{}\n")

	if st == nil {
		return buf.String()
	}

	hasPagination := false
	hasSort := false
	for _, op := range st.Operations {
		if op.XPagination != nil {
			hasPagination = true
		}
		if op.XSort != nil {
			hasSort = true
		}
	}

	if hasPagination {
		buf.WriteString("\tif v := c.Query(\"limit\"); v != \"\" {\n")
		buf.WriteString("\t\topts.Limit, _ = strconv.Atoi(v)\n")
		buf.WriteString("\t}\n")
		buf.WriteString("\tif v := c.Query(\"offset\"); v != \"\" {\n")
		buf.WriteString("\t\topts.Offset, _ = strconv.Atoi(v)\n")
		buf.WriteString("\t}\n")
	}
	if hasSort {
		buf.WriteString("\tif v := c.Query(\"sort\"); v != \"\" {\n")
		buf.WriteString("\t\topts.SortCol = v\n")
		buf.WriteString("\t}\n")
	}

	return buf.String()
}

func defaultMessage(seq parser.Sequence) string {
	modelName := ""
	if seq.Model != "" {
		parts := strings.SplitN(seq.Model, ".", 2)
		modelName = parts[0]
		if seq.Package != "" {
			modelName = seq.Package + "." + modelName
		}
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
	case parser.SeqEmpty:
		return seq.Target + "가 존재하지 않습니다"
	case parser.SeqExists:
		return seq.Target + "가 이미 존재합니다"
	case parser.SeqState:
		return "상태 전이가 허용되지 않습니다"
	case parser.SeqAuth:
		return "권한이 없습니다"
	case parser.SeqCall:
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

func hasConversionErr(params []typedRequestParam) bool {
	for _, p := range params {
		if p.goType != "string" && p.goType != "json_body" {
			return true
		}
	}
	return false
}

func rootVar(s string) string {
	if idx := strings.Index(s, "."); idx >= 0 {
		return s[:idx]
	}
	return s
}

func httpStatusConst(code int) string {
	switch code {
	case 400:
		return "http.StatusBadRequest"
	case 401:
		return "http.StatusUnauthorized"
	case 402:
		return "http.StatusPaymentRequired"
	case 403:
		return "http.StatusForbidden"
	case 404:
		return "http.StatusNotFound"
	case 409:
		return "http.StatusConflict"
	case 422:
		return "http.StatusUnprocessableEntity"
	case 429:
		return "http.StatusTooManyRequests"
	case 500:
		return "http.StatusInternalServerError"
	case 502:
		return "http.StatusBadGateway"
	case 503:
		return "http.StatusServiceUnavailable"
	default:
		return fmt.Sprintf("%d", code)
	}
}

// --- Model 인터페이스 파생 ---

type modelUsage struct {
	ModelName  string
	MethodName string
	Inputs     map[string]string
	Result     *parser.Result
	FuncName   string
}

func collectModelUsages(funcs []parser.ServiceFunc) []modelUsage {
	var usages []modelUsage
	for _, sf := range funcs {
		for _, seq := range sf.Sequences {
			if seq.Model == "" || seq.Type == parser.SeqCall || seq.Package != "" {
				continue // @call과 패키지 접두사 모델은 models_gen.go에 포함하지 않음
			}
			parts := strings.SplitN(seq.Model, ".", 2)
			if len(parts) < 2 {
				continue
			}
			usages = append(usages, modelUsage{
				ModelName:  parts[0],
				MethodName: parts[1],
				Inputs:     seq.Inputs,
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

			var inputKeys []string
			if len(mi.Params) > 0 {
				// sqlc/interface 파라미터 순서대로
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

			for _, k := range inputKeys {
				val := usage.Inputs[k]
				if val == "query" {
					dm.HasQueryOpts = true
					continue
				}
				dp := derivedParam{
					Name:   strcase.ToGoCamel(k),
					GoType: resolveInputParamType(val, usage.ModelName, st),
				}
				if dp.Name != "" {
					dm.Params = append(dm.Params, dp)
				}
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

func resolveArgParamName(a parser.Arg) string {
	if a.Literal != "" {
		return strcase.ToGoCamel(a.Literal) // 리터럴은 값 자체를 이름으로 사용
	}
	if a.Source == "request" || a.Source == "currentUser" {
		return strcase.ToGoCamel(a.Field)
	}
	if a.Source != "" {
		return a.Source + strcase.ToGoPascal(a.Field)
	}
	return strcase.ToGoCamel(a.Field)
}

func resolveArgParamType(a parser.Arg, modelName string, st *validator.SymbolTable) string {
	if a.Literal != "" {
		return "string"
	}

	// source.Field → source 테이블의 field 컬럼 조회
	if a.Source != "" && a.Source != "request" && a.Source != "currentUser" {
		refTable := toSnakeCase(a.Source) + "s"
		refCol := toSnakeCase(a.Field)
		if table, ok := st.DDLTables[refTable]; ok {
			if goType, ok := table.Columns[refCol]; ok {
				return goType
			}
		}
	}

	snakeName := toSnakeCase(a.Field)

	// 해당 모델 테이블
	tableName := toSnakeCase(modelName) + "s"
	if table, ok := st.DDLTables[tableName]; ok {
		if goType, ok := table.Columns[snakeName]; ok {
			return goType
		}
	}

	// {Model}ID 패턴
	if strings.HasSuffix(a.Field, "ID") {
		refModel := a.Field[:len(a.Field)-2]
		refTable := toSnakeCase(refModel) + "s"
		if table, ok := st.DDLTables[refTable]; ok {
			if goType, ok := table.Columns["id"]; ok {
				return goType
			}
		}
	}

	// 전체 순회
	for _, table := range st.DDLTables {
		if goType, ok := table.Columns[snakeName]; ok {
			return goType
		}
	}

	return "string"
}

// resolveInputParamType는 Inputs value에서 Go 타입을 추론한다.
// value 형식: "request.Field", "source.Field", "\"literal\"", "currentUser.Field"
func resolveInputParamType(val string, modelName string, st *validator.SymbolTable) string {
	// 리터럴
	if strings.HasPrefix(val, `"`) {
		return "string"
	}

	dotIdx := strings.IndexByte(val, '.')
	if dotIdx < 0 {
		return "string"
	}
	source := val[:dotIdx]
	field := val[dotIdx+1:]

	// source.Field → source 테이블의 field 컬럼 조회
	if source != "request" && source != "currentUser" {
		refTable := toSnakeCase(source) + "s"
		refCol := toSnakeCase(field)
		if table, ok := st.DDLTables[refTable]; ok {
			if goType, ok := table.Columns[refCol]; ok {
				return goType
			}
		}
	}

	snakeName := toSnakeCase(field)

	// 해당 모델 테이블
	tableName := toSnakeCase(modelName) + "s"
	if table, ok := st.DDLTables[tableName]; ok {
		if goType, ok := table.Columns[snakeName]; ok {
			return goType
		}
	}

	// {Model}ID 패턴
	if strings.HasSuffix(field, "ID") {
		refModel := field[:len(field)-2]
		refTable := toSnakeCase(refModel) + "s"
		if table, ok := st.DDLTables[refTable]; ok {
			if goType, ok := table.Columns["id"]; ok {
				return goType
			}
		}
	}

	// 전체 순회
	for _, table := range st.DDLTables {
		if goType, ok := table.Columns[snakeName]; ok {
			return goType
		}
	}

	return "string"
}

func deriveReturnType(mi validator.MethodInfo, usage modelUsage, hasQueryOpts bool) string {
	// Wrapper 타입 (Page[T], Cursor[T])
	if usage.Result != nil && usage.Result.Wrapper != "" {
		return fmt.Sprintf("(*pagination.%s[%s], error)", usage.Result.Wrapper, usage.Result.Type)
	}

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

	needTime := needsTimeImport(interfaces)
	needPagination := needsPaginationImport(interfaces)
	if needTime || needPagination {
		buf.WriteString("import (\n")
		if needTime {
			buf.WriteString("\t\"time\"\n")
		}
		if needPagination {
			buf.WriteString("\t\"github.com/geul-org/fullend/pkg/pagination\"\n")
		}
		buf.WriteString(")\n\n")
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
	Limit   int
	Offset  int
	Cursor  string
	SortCol string
	SortDir string
	Filters map[string]string
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

func needsPaginationImport(interfaces []derivedInterface) bool {
	for _, iface := range interfaces {
		for _, m := range iface.Methods {
			if strings.Contains(m.ReturnType, "pagination.") {
				return true
			}
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
