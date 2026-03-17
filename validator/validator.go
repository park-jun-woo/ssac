package validator

import (
	"fmt"
	"sort"
	"strings"

	"github.com/park-jun-woo/ssac/parser"
)

// Validate는 []ServiceFunc의 내부 정합성을 검증한다.
func Validate(funcs []parser.ServiceFunc) []ValidationError {
	var errs []ValidationError
	for _, sf := range funcs {
		errs = append(errs, validateFunc(sf)...)
	}
	return errs
}

// ValidateWithSymbols는 내부 검증 + 외부 SSOT 교차 검증을 수행한다.
func ValidateWithSymbols(funcs []parser.ServiceFunc, st *SymbolTable) []ValidationError {
	errs := Validate(funcs)
	for _, sf := range funcs {
		errs = append(errs, validateModel(sf, st)...)
		errs = append(errs, validateRequest(sf, st)...)

		errs = append(errs, validateQueryUsage(sf, st)...)
		errs = append(errs, validatePaginationType(sf, st)...)
		errs = append(errs, validateCallInputTypes(sf, st)...)
	}
	errs = append(errs, validateGoReservedWords(funcs, st)...)
	return errs
}

func validateFunc(sf parser.ServiceFunc) []ValidationError {
	var errs []ValidationError
	errs = append(errs, validateRequiredFields(sf)...)
	errs = append(errs, validateVariableFlow(sf)...)
	errs = append(errs, validateStaleResponse(sf)...)
	errs = append(errs, validateReservedSourceConflict(sf)...)
	errs = append(errs, validateSubscribeRules(sf)...)
	errs = append(errs, validateFKReferenceGuard(sf)...)
	return errs
}

// validateRequiredFields는 타입별 필수 필드 누락을 검증한다.
func validateRequiredFields(sf parser.ServiceFunc) []ValidationError {
	var errs []ValidationError

	for i, seq := range sf.Sequences {
		ctx := errCtx{sf.FileName, sf.Name, i}

		switch seq.Type {
		case parser.SeqGet:
			if seq.Model == "" {
				errs = append(errs, ctx.err("@get", "Model 누락"))
			}
			if seq.Result == nil {
				errs = append(errs, ctx.err("@get", "Result 누락"))
			}
			// Args는 0개 허용 (비즈니스 필터 없는 전체 조회)

		case parser.SeqPost:
			if seq.Model == "" {
				errs = append(errs, ctx.err("@post", "Model 누락"))
			}
			if seq.Result == nil {
				errs = append(errs, ctx.err("@post", "Result 누락"))
			}
			if len(seq.Inputs) == 0 {
				errs = append(errs, ctx.err("@post", "Inputs 누락"))
			}

		case parser.SeqPut:
			if seq.Model == "" {
				errs = append(errs, ctx.err("@put", "Model 누락"))
			}
			if seq.Result != nil {
				errs = append(errs, ctx.err("@put", "Result는 nil이어야 함"))
			}
			if len(seq.Inputs) == 0 {
				errs = append(errs, ctx.err("@put", "Inputs 누락"))
			}

		case parser.SeqDelete:
			if seq.Model == "" {
				errs = append(errs, ctx.err("@delete", "Model 누락"))
			}
			if seq.Result != nil {
				errs = append(errs, ctx.err("@delete", "Result는 nil이어야 함"))
			}
			if len(seq.Inputs) == 0 && !seq.SuppressWarn {
				errs = append(errs, ValidationError{
					FileName: ctx.fileName, FuncName: ctx.funcName, SeqIndex: ctx.seqIndex,
					Tag: "@delete", Message: "Inputs가 없습니다 — 전체 삭제 의도가 맞는지 확인하세요", Level: "WARNING",
				})
			}

		case parser.SeqEmpty, parser.SeqExists:
			if seq.Target == "" {
				errs = append(errs, ctx.err("@"+seq.Type, "Target 누락"))
			}
			if seq.Message == "" {
				errs = append(errs, ctx.err("@"+seq.Type, "Message 누락"))
			}

		case parser.SeqState:
			if seq.DiagramID == "" {
				errs = append(errs, ctx.err("@state", "DiagramID 누락"))
			}
			if len(seq.Inputs) == 0 {
				errs = append(errs, ctx.err("@state", "Inputs 누락"))
			}
			if seq.Transition == "" {
				errs = append(errs, ctx.err("@state", "Transition 누락"))
			}
			if seq.Message == "" {
				errs = append(errs, ctx.err("@state", "Message 누락"))
			}

		case parser.SeqAuth:
			if seq.Action == "" {
				errs = append(errs, ctx.err("@auth", "Action 누락"))
			}
			if seq.Resource == "" {
				errs = append(errs, ctx.err("@auth", "Resource 누락"))
			}
			if seq.Message == "" {
				errs = append(errs, ctx.err("@auth", "Message 누락"))
			}

		case parser.SeqCall:
			if seq.Model == "" {
				errs = append(errs, ctx.err("@call", "Model 누락"))
			}
			if seq.Result != nil && isPrimitiveType(seq.Result.Type) {
				errs = append(errs, ctx.err("@call", fmt.Sprintf("반환 타입에 기본 타입 %q은 사용할 수 없습니다 — Response struct 타입을 지정하세요", seq.Result.Type)))
			}

		case parser.SeqPublish:
			if seq.Topic == "" {
				errs = append(errs, ctx.err("@publish", "Topic 누락"))
			}
			if len(seq.Inputs) == 0 {
				errs = append(errs, ctx.err("@publish", "Payload 누락"))
			}

		case parser.SeqResponse:
			// Fields는 선택 — 빈 @response {} 허용 (DELETE 등)

		default:
			errs = append(errs, ctx.err("@sequence", fmt.Sprintf("알 수 없는 타입: %q", seq.Type)))
		}

		// Model 형식 검증: "Model.Method" 또는 "pkg.Func"
		if seq.Model != "" {
			parts := strings.SplitN(seq.Model, ".", 2)
			if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
				errs = append(errs, ctx.err("@"+seq.Type, fmt.Sprintf("\"Model.Method\" 형식이어야 함: %q", seq.Model)))
			}
		}
	}

	return errs
}

// validateVariableFlow는 변수가 선언 전에 참조되지 않는지 검증한다.
func validateVariableFlow(sf parser.ServiceFunc) []ValidationError {
	var errs []ValidationError
	declared := map[string]bool{
		"currentUser": true,
	}
	if sf.Subscribe != nil {
		declared["message"] = true
	}

	for i, seq := range sf.Sequences {
		ctx := errCtx{sf.FileName, sf.Name, i}

		// guard Target 검증
		if seq.Target != "" {
			rootTarget := rootVar(seq.Target)
			if !declared[rootTarget] {
				errs = append(errs, ctx.err("@"+seq.Type, fmt.Sprintf("%q 변수가 아직 선언되지 않았습니다", rootTarget)))
			}
		}

		// Args source 검증
		for _, arg := range seq.Args {
			ref := argVarRef(arg)
			if ref != "" && !declared[ref] {
				errs = append(errs, ctx.err("@"+seq.Type, fmt.Sprintf("%q 변수가 아직 선언되지 않았습니다", ref)))
			}
		}

		// Inputs value 검증
		for _, val := range seq.Inputs {
			if strings.HasPrefix(val, `"`) {
				continue // 리터럴
			}
			if strings.HasPrefix(val, "config.") {
				errs = append(errs, ctx.err("@"+seq.Type, fmt.Sprintf("config.%s — config.* 입력은 지원하지 않습니다. func 내부에서 os.Getenv()를 사용하세요", val[len("config."):])))
				continue
			}
			// @publish에서 query 사용 금지
			if seq.Type == parser.SeqPublish && (val == "query" || strings.HasPrefix(val, "query.")) {
				errs = append(errs, ctx.err("@publish", "query는 HTTP 전용입니다 — @publish에서 사용할 수 없습니다"))
				continue
			}
			ref := rootVar(val)
			if ref != "request" && ref != "currentUser" && ref != "query" && ref != "" && !declared[ref] {
				errs = append(errs, ctx.err("@"+seq.Type, fmt.Sprintf("%q 변수가 아직 선언되지 않았습니다", ref)))
			}
		}

		// @response Fields value 검증
		for _, val := range seq.Fields {
			if strings.HasPrefix(val, `"`) {
				continue // 리터럴
			}
			ref := rootVar(val)
			if ref != "" && !declared[ref] {
				errs = append(errs, ctx.err("@response", fmt.Sprintf("%q 변수가 아직 선언되지 않았습니다", ref)))
			}
		}

		// Result로 변수 선언
		if seq.Result != nil {
			declared[seq.Result.Var] = true
		}
	}

	return errs
}

// validateStaleResponse는 put/delete 이후 갱신 없이 response에서 사용되는 변수를 경고한다.
func validateStaleResponse(sf parser.ServiceFunc) []ValidationError {
	var errs []ValidationError

	getVars := map[string]string{}   // var → model
	mutated := map[string]bool{}     // model → mutated?

	for i, seq := range sf.Sequences {
		switch seq.Type {
		case parser.SeqGet:
			if seq.Result != nil && seq.Model != "" {
				modelName := strings.SplitN(seq.Model, ".", 2)[0]
				getVars[seq.Result.Var] = modelName
				mutated[modelName] = false
			}
		case parser.SeqPut, parser.SeqDelete:
			if seq.Model != "" {
				modelName := strings.SplitN(seq.Model, ".", 2)[0]
				mutated[modelName] = true
			}
		case parser.SeqResponse:
			if seq.SuppressWarn {
				continue
			}
			ctx := errCtx{sf.FileName, sf.Name, i}
			for field, val := range seq.Fields {
				ref := rootVar(val)
				if modelName, ok := getVars[ref]; ok && mutated[modelName] {
					errs = append(errs, ValidationError{
						FileName: ctx.fileName,
						FuncName: ctx.funcName,
						SeqIndex: ctx.seqIndex,
						Tag:      "@response",
						Message:  fmt.Sprintf("%q (필드 %q)가 %s 수정 이후 갱신 없이 response에 사용됩니다", ref, field, modelName),
						Level:    "WARNING",
					})
				}
			}
		}
	}

	return errs
}

// validateFKReferenceGuard는 FK 참조 @get 후 @empty 가드 누락을 검증한다.
// FK 참조: @get의 input이 이전 result 변수의 필드를 참조 (request/currentUser 아닌 경우).
// nil pointer dereference 방지를 위해 @empty 가드가 필요하다. @get!로 억제 가능.
func validateFKReferenceGuard(sf parser.ServiceFunc) []ValidationError {
	var errs []ValidationError
	declared := map[string]bool{}
	if sf.Subscribe != nil {
		declared["message"] = true
	}

	for i, seq := range sf.Sequences {
		if seq.Type == parser.SeqGet && seq.Result != nil {
			// 슬라이스/래퍼 결과는 nil dereference 위험 없음
			if strings.HasPrefix(seq.Result.Type, "[]") || seq.Result.Wrapper != "" {
				declared[seq.Result.Var] = true
				continue
			}

			// input 중 이전 result 변수 참조가 있는지 확인
			hasFKRef := false
			for _, val := range seq.Inputs {
				if strings.HasPrefix(val, `"`) {
					continue
				}
				ref := rootVar(val)
				if ref == "request" || ref == "currentUser" || ref == "query" || ref == "message" || ref == "config" || ref == "" {
					continue
				}
				if declared[ref] {
					hasFKRef = true
					break
				}
			}

			if hasFKRef {
				// 이후 시퀀스에 @empty 가드가 있는지 확인
				hasEmptyGuard := false
				for _, laterSeq := range sf.Sequences[i+1:] {
					if laterSeq.Type == parser.SeqEmpty && rootVar(laterSeq.Target) == seq.Result.Var {
						hasEmptyGuard = true
						break
					}
				}
				if !hasEmptyGuard {
					ctx := errCtx{sf.FileName, sf.Name, i}
					errs = append(errs, ctx.err("@get", fmt.Sprintf("%q — FK 참조 조회 후 @empty 가드가 필요합니다", seq.Result.Var)))
				}
			}
		}

		if seq.Result != nil {
			declared[seq.Result.Var] = true
		}
	}

	return errs
}

// validateModel은 Model이 심볼 테이블에 존재하는지 검증한다.
func validateModel(sf parser.ServiceFunc, st *SymbolTable) []ValidationError {
	var errs []ValidationError
	for i, seq := range sf.Sequences {
		if seq.Model == "" || seq.Type == parser.SeqCall {
			continue // @call은 외부 패키지이므로 교차검증 스킵
		}
		ctx := errCtx{sf.FileName, sf.Name, i}
		parts := strings.SplitN(seq.Model, ".", 2)
		if len(parts) < 2 {
			continue
		}
		modelName, methodName := parts[0], parts[1]

		if seq.Package != "" {
			// 패키지 접두사 모델: "pkg.Model" 키로 조회
			pkgModelKey := seq.Package + "." + modelName
			ms, ok := st.Models[pkgModelKey]
			if !ok {
				// interface가 없으면 WARNING (검증 불가)
				errs = append(errs, ValidationError{
					FileName: ctx.fileName, FuncName: ctx.funcName, SeqIndex: ctx.seqIndex,
					Tag: "@" + seq.Type, Message: fmt.Sprintf("%s.%s — 패키지 interface를 찾을 수 없습니다. 검증을 건너뜁니다", seq.Package, modelName), Level: "WARNING",
				})
				continue
			}
			if !ms.HasMethod(methodName) {
				// 사용 가능 메서드 안내
				available := make([]string, 0, len(ms.Methods))
				for m := range ms.Methods {
					available = append(available, m)
				}
				sort.Strings(available)
				errs = append(errs, ctx.err("@"+seq.Type, fmt.Sprintf("%s.%s — 메서드 %q 없음. 사용 가능: %s", seq.Package, modelName, methodName, strings.Join(available, ", "))))
				continue
			}
			// 파라미터 매칭 검증
			mi := ms.Methods[methodName]
			if len(mi.Params) > 0 {
				ifaceParamSet := make(map[string]bool, len(mi.Params))
				for _, p := range mi.Params {
					ifaceParamSet[p] = true
				}
				ssacKeys := make([]string, 0, len(seq.Inputs))
				for k := range seq.Inputs {
					ssacKeys = append(ssacKeys, k)
				}
				sort.Strings(ssacKeys)
				// SSaC에 있지만 interface에 없는 키
				for _, key := range ssacKeys {
					if !ifaceParamSet[key] {
						errs = append(errs, ctx.err("@"+seq.Type, fmt.Sprintf("SSaC ↔ Interface: %s — @model %s.%s.%s 파라미터 불일치. SSaC에 %q가 있지만 interface에 없습니다. interface 파라미터: [%s]", sf.Name, seq.Package, modelName, methodName, key, strings.Join(mi.Params, ", "))))
					}
				}
				// interface에 있지만 SSaC에 없는 파라미터
				ssacKeySet := make(map[string]bool, len(seq.Inputs))
				for k := range seq.Inputs {
					ssacKeySet[k] = true
				}
				for _, param := range mi.Params {
					if !ssacKeySet[param] {
						errs = append(errs, ctx.err("@"+seq.Type, fmt.Sprintf("SSaC ↔ Interface: %s — @model %s.%s.%s 파라미터 누락. interface에 %q가 필요하지만 SSaC에 없습니다. SSaC 파라미터: [%s]", sf.Name, seq.Package, modelName, methodName, param, strings.Join(ssacKeys, ", "))))
					}
				}
			}
			continue
		}

		ms, ok := st.Models[modelName]
		if !ok {
			errs = append(errs, ctx.err("@"+seq.Type, fmt.Sprintf("%q 모델을 찾을 수 없습니다", modelName)))
			continue
		}
		if !ms.HasMethod(methodName) {
			errs = append(errs, ctx.err("@"+seq.Type, fmt.Sprintf("%q 모델에 %q 메서드가 없습니다", modelName, methodName)))
		}
	}
	return errs
}

// validateRequest는 request 필드가 OpenAPI와 일치하는지 검증한다.
func validateRequest(sf parser.ServiceFunc, st *SymbolTable) []ValidationError {
	var errs []ValidationError
	op, ok := st.Operations[sf.Name]
	if !ok {
		return nil
	}

	usedRequestFields := make(map[string]bool)
	for i, seq := range sf.Sequences {
		ctx := errCtx{sf.FileName, sf.Name, i}
		for _, arg := range seq.Args {
			if arg.Source != "request" {
				continue
			}
			usedRequestFields[arg.Field] = true
			if !op.RequestFields[arg.Field] {
				errs = append(errs, ctx.err("@"+seq.Type, fmt.Sprintf("OpenAPI request에 %q 필드가 없습니다", arg.Field)))
			}
		}
		// @auth/@state Inputs에서 request 참조도 확인
		for _, val := range seq.Inputs {
			if strings.HasPrefix(val, "request.") {
				field := val[len("request."):]
				usedRequestFields[field] = true
				if !op.RequestFields[field] {
					errs = append(errs, ctx.err("@"+seq.Type, fmt.Sprintf("OpenAPI request에 %q 필드가 없습니다", field)))
				}
			}
		}
	}

	// 역방향: OpenAPI → SSaC (path param 제외)
	pathParams := make(map[string]bool)
	for _, pp := range op.PathParams {
		pathParams[pp.Name] = true
	}
	for field := range op.RequestFields {
		if pathParams[field] {
			continue
		}
		if !usedRequestFields[field] {
			errs = append(errs, ValidationError{
				FileName: sf.FileName,
				FuncName: sf.Name,
				SeqIndex: -1,
				Tag:      "@request",
				Message:  fmt.Sprintf("OpenAPI request에 %q 필드가 있지만 SSaC에서 사용하지 않습니다", field),
				Level:    "WARNING",
			})
		}
	}

	return errs
}

// validateQueryUsage는 SSaC의 query 사용과 OpenAPI x-extensions 간 교차 검증을 수행한다.
func validateQueryUsage(sf parser.ServiceFunc, st *SymbolTable) []ValidationError {
	if st == nil {
		return nil
	}

	op, hasOp := st.Operations[sf.Name]
	opHasQueryOpts := hasOp && op.HasQueryOpts()

	specHasQuery := false
	for _, seq := range sf.Sequences {
		for _, val := range seq.Inputs {
			if val == "query" {
				specHasQuery = true
				break
			}
		}
		if specHasQuery {
			break
		}
	}

	var errs []ValidationError
	ctx := errCtx{sf.FileName, sf.Name, -1}

	if specHasQuery && !opHasQueryOpts {
		errs = append(errs, ctx.err("@query", "SSaC에 query가 사용되었지만 OpenAPI에 x-pagination/sort/filter가 없습니다"))
	}
	if opHasQueryOpts && !specHasQuery {
		errs = append(errs, ValidationError{
			FileName: ctx.fileName, FuncName: ctx.funcName, SeqIndex: ctx.seqIndex,
			Tag: "@query", Message: "OpenAPI에 x-pagination/sort/filter가 있지만 SSaC에 query가 사용되지 않았습니다", Level: "WARNING",
		})
	}

	return errs
}

// validatePaginationType은 x-pagination style과 Result.Wrapper 타입의 일치를 검증한다.
func validatePaginationType(sf parser.ServiceFunc, st *SymbolTable) []ValidationError {
	if st == nil {
		return nil
	}

	op, ok := st.Operations[sf.Name]
	if !ok {
		return nil
	}

	var errs []ValidationError
	for i, seq := range sf.Sequences {
		if seq.Result == nil || seq.Result.Wrapper == "" && !strings.HasPrefix(seq.Result.Type, "[]") {
			continue
		}
		ctx := errCtx{sf.FileName, sf.Name, i}

		if op.XPagination != nil {
			// x-pagination 있음 → Wrapper 필수
			switch op.XPagination.Style {
			case "offset":
				if seq.Result.Wrapper != "Page" {
					errs = append(errs, ctx.err("@get", "x-pagination style: offset이지만 반환 타입이 Page[T]가 아닙니다"))
				}
			case "cursor":
				if seq.Result.Wrapper != "Cursor" {
					errs = append(errs, ctx.err("@get", "x-pagination style: cursor이지만 반환 타입이 Cursor[T]가 아닙니다"))
				}
			}
		} else {
			// x-pagination 없음 → Wrapper 사용 불가
			if seq.Result.Wrapper != "" {
				errs = append(errs, ctx.err("@get", fmt.Sprintf("OpenAPI에 x-pagination이 없지만 %s[T] 타입을 사용했습니다. []T를 사용하세요", seq.Result.Wrapper)))
			}
		}
	}

	return errs
}

// validateCallInputTypes는 @call inputs의 필드 타입을 func Request struct 필드 타입과 비교한다.
func validateCallInputTypes(sf parser.ServiceFunc, st *SymbolTable) []ValidationError {
	if st == nil {
		return nil
	}

	// result 변수 → 모델명 매핑 (DDL 타입 추적용)
	resultModels := map[string]string{}
	for _, seq := range sf.Sequences {
		if seq.Result != nil && seq.Model != "" {
			parts := strings.SplitN(seq.Model, ".", 2)
			resultModels[seq.Result.Var] = parts[0]
		}
	}

	var errs []ValidationError
	for i, seq := range sf.Sequences {
		if seq.Type != parser.SeqCall || seq.Model == "" {
			continue
		}
		parts := strings.SplitN(seq.Model, ".", 2)
		if len(parts) < 2 {
			continue
		}
		pkgName, funcName := parts[0], parts[1]

		// pkg.Model 키로 심볼 테이블에서 ParamTypes 조회
		// @call은 func이므로 모델 키를 찾아야 함
		var paramTypes map[string]string
		for modelKey, ms := range st.Models {
			if !strings.HasPrefix(modelKey, pkgName+".") {
				continue
			}
			if mi, ok := ms.Methods[funcName]; ok && mi.ParamTypes != nil {
				paramTypes = mi.ParamTypes
				break
			}
		}
		if paramTypes == nil {
			continue // Request struct가 파싱되지 않았으면 스킵
		}

		ctx := errCtx{sf.FileName, sf.Name, i}
		for key, val := range seq.Inputs {
			expectedType, ok := paramTypes[key]
			if !ok {
				continue // 필드가 Request struct에 없으면 다른 검증에서 처리
			}
			actualType := resolveCallInputType(val, resultModels, st)
			if actualType != "" && actualType != expectedType {
				errs = append(errs, ctx.err("@call", fmt.Sprintf("입력 %q의 타입 불일치: %s은 %s, Request 필드는 %s", key, val, actualType, expectedType)))
			}
		}
	}
	return errs
}

// resolveCallInputType는 @call input value에서 Go 타입을 결정한다.
func resolveCallInputType(val string, resultModels map[string]string, st *SymbolTable) string {
	// 리터럴
	if strings.HasPrefix(val, `"`) {
		return "string"
	}

	dotIdx := strings.IndexByte(val, '.')
	if dotIdx < 0 {
		return ""
	}
	source := val[:dotIdx]
	field := val[dotIdx+1:]

	// currentUser → 현재는 타입 추적 불가, 스킵
	if source == "currentUser" {
		return ""
	}
	// request → DDL에서 역추적
	if source == "request" {
		snakeName := toSnakeCase(field)
		for _, table := range st.DDLTables {
			if goType, ok := table.Columns[snakeName]; ok {
				return goType
			}
		}
		return ""
	}
	// 변수.Field → 해당 변수의 모델 테이블에서 Field 컬럼 타입
	modelName, ok := resultModels[source]
	if !ok {
		return ""
	}
	tableName := toSnakeCase(modelName) + "s"
	snakeName := toSnakeCase(field)
	if table, ok := st.DDLTables[tableName]; ok {
		if goType, ok := table.Columns[snakeName]; ok {
			return goType
		}
	}
	// ID 패턴 fallback
	if strings.HasSuffix(field, "ID") {
		refModel := field[:len(field)-2]
		refTable := toSnakeCase(refModel) + "s"
		if table, ok := st.DDLTables[refTable]; ok {
			if goType, ok := table.Columns["id"]; ok {
				return goType
			}
		}
	}
	return ""
}

// goReservedWords는 Go 예약어 25개다.
var goReservedWords = map[string]bool{
	"break": true, "case": true, "chan": true, "const": true,
	"continue": true, "default": true, "defer": true, "else": true,
	"fallthrough": true, "for": true, "func": true, "go": true,
	"goto": true, "if": true, "import": true, "interface": true,
	"map": true, "package": true, "range": true, "return": true,
	"select": true, "struct": true, "switch": true, "type": true,
	"var": true,
}

// validateGoReservedWords는 SSaC Inputs 키가 Go 예약어와 충돌하면 ERROR를 반환한다.
func validateGoReservedWords(funcs []parser.ServiceFunc, st *SymbolTable) []ValidationError {
	var errs []ValidationError
	seen := map[string]bool{} // 중복 에러 방지: "table.column"

	for _, sf := range funcs {
		for i, seq := range sf.Sequences {
			if seq.Package != "" || seq.Type == parser.SeqCall {
				continue // 패키지 모델과 @call은 models_gen.go 대상 아님
			}
			for key := range seq.Inputs {
				paramName := toLowerFirst(key)
				if !goReservedWords[paramName] {
					continue
				}
				// DDL 테이블에서 컬럼 역추적
				snakeName := toSnakeCase(key)
				tableName, found := findColumnTable(snakeName, seq.Model, st)
				ctx := errCtx{sf.FileName, sf.Name, i}
				dedup := tableName + "." + snakeName
				if seen[dedup] {
					continue
				}
				seen[dedup] = true
				if found {
					errs = append(errs, ctx.err("@"+seq.Type, fmt.Sprintf("DDL column %q in table %q is a Go reserved word — rename the column (e.g. %q)", snakeName, tableName, "tx_"+snakeName)))
				} else {
					errs = append(errs, ctx.err("@"+seq.Type, fmt.Sprintf("parameter name %q is a Go reserved word — rename the DDL column", paramName)))
				}
			}
		}
	}
	return errs
}

// toLowerFirst는 첫 글자를 소문자로 변환한다.
func toLowerFirst(s string) string {
	if s == "" {
		return s
	}
	return strings.ToLower(s[:1]) + s[1:]
}

// toSnakeCase는 PascalCase/camelCase를 snake_case로 변환한다.
func toSnakeCase(s string) string {
	var result []byte
	for i, c := range s {
		if c >= 'A' && c <= 'Z' {
			if i > 0 {
				prev := s[i-1]
				if prev >= 'a' && prev <= 'z' {
					result = append(result, '_')
				} else if prev >= 'A' && prev <= 'Z' && i+1 < len(s) && s[i+1] >= 'a' && s[i+1] <= 'z' {
					result = append(result, '_')
				}
			}
			result = append(result, byte(c)+32)
		} else {
			result = append(result, byte(c))
		}
	}
	return string(result)
}

// findColumnTable는 snake_case 컬럼명이 존재하는 DDL 테이블을 찾는다.
func findColumnTable(snakeCol, model string, st *SymbolTable) (string, bool) {
	if st == nil {
		return "", false
	}
	// 모델명에서 테이블명 유추: "Transaction.Create" → "transactions"
	if model != "" {
		parts := strings.SplitN(model, ".", 2)
		tableName := toSnakeCase(parts[0]) + "s"
		if table, ok := st.DDLTables[tableName]; ok {
			if _, ok := table.Columns[snakeCol]; ok {
				return tableName, true
			}
		}
	}
	// 전체 DDL 테이블에서 검색
	for tableName, table := range st.DDLTables {
		if _, ok := table.Columns[snakeCol]; ok {
			return tableName, true
		}
	}
	return "", false
}

// reservedSources는 사용자가 result 변수명으로 사용할 수 없는 예약 소스다.
var reservedSources = map[string]bool{
	"request":     true,
	"currentUser": true,
	"config":      true,
	"query":       true,
	"message":     true,
}

// validateReservedSourceConflict는 result 변수명이 예약 소스와 충돌하면 ERROR를 반환한다.
func validateReservedSourceConflict(sf parser.ServiceFunc) []ValidationError {
	var errs []ValidationError
	for i, seq := range sf.Sequences {
		if seq.Result == nil {
			continue
		}
		if reservedSources[seq.Result.Var] {
			ctx := errCtx{sf.FileName, sf.Name, i}
			errs = append(errs, ctx.err("@"+seq.Type, fmt.Sprintf("%q는 예약 소스이므로 result 변수명으로 사용할 수 없습니다", seq.Result.Var)))
		}
	}
	return errs
}

// validateSubscribeRules는 subscribe/HTTP 트리거와 관련된 규칙을 검증한다.
func validateSubscribeRules(sf parser.ServiceFunc) []ValidationError {
	var errs []ValidationError

	if sf.Subscribe != nil {
		ctx := errCtx{sf.FileName, sf.Name, -1}

		// subscribe 함수에 파라미터 필수
		if sf.Param == nil {
			errs = append(errs, ctx.err("@subscribe", "@subscribe 함수에 파라미터가 필요합니다 — func Name(TypeName message) {}"))
		}

		// 파라미터 변수명은 반드시 "message"
		if sf.Param != nil && sf.Param.VarName != "message" {
			errs = append(errs, ctx.err("@subscribe", fmt.Sprintf("파라미터 변수명은 \"message\"여야 합니다 — 현재: %q", sf.Param.VarName)))
		}

		// MessageType이 파일 내 struct로 존재하는지
		if sf.Subscribe.MessageType != "" {
			found := false
			for _, si := range sf.Structs {
				if si.Name == sf.Subscribe.MessageType {
					found = true
					break
				}
			}
			if !found {
				errs = append(errs, ctx.err("@subscribe", fmt.Sprintf("메시지 타입 %q이 파일 내에 struct로 선언되지 않았습니다", sf.Subscribe.MessageType)))
			}
		}

		// subscribe 함수에 @response 있으면 ERROR
		for i, seq := range sf.Sequences {
			if seq.Type == parser.SeqResponse {
				seqCtx := errCtx{sf.FileName, sf.Name, i}
				errs = append(errs, seqCtx.err("@subscribe", "@subscribe 함수에 @response를 사용할 수 없습니다"))
			}
		}
		// subscribe 함수에서 request 사용하면 ERROR
		for i, seq := range sf.Sequences {
			for _, val := range seq.Inputs {
				if strings.HasPrefix(val, "request.") {
					seqCtx := errCtx{sf.FileName, sf.Name, i}
					errs = append(errs, seqCtx.err("@subscribe", "@subscribe 함수에서 request를 사용할 수 없습니다 — message를 사용하세요"))
					break
				}
			}
		}
		// subscribe 함수에서 query 사용하면 ERROR
		for i, seq := range sf.Sequences {
			for _, val := range seq.Inputs {
				if val == "query" || strings.HasPrefix(val, "query.") {
					seqCtx := errCtx{sf.FileName, sf.Name, i}
					errs = append(errs, seqCtx.err("@subscribe", "query는 HTTP 전용입니다 — @subscribe 함수에서 사용할 수 없습니다"))
					break
				}
			}
		}

		// message.Field 검증: struct 필드 존재 확인
		if sf.Subscribe.MessageType != "" {
			for i, seq := range sf.Sequences {
				for _, val := range seq.Inputs {
					if strings.HasPrefix(val, "message.") {
						field := val[len("message."):]
						if !hasStructField(sf.Structs, sf.Subscribe.MessageType, field) {
							seqCtx := errCtx{sf.FileName, sf.Name, i}
							errs = append(errs, seqCtx.err("@"+seq.Type, fmt.Sprintf("message.%s — 메시지 타입 %q에 %q 필드가 없습니다", field, sf.Subscribe.MessageType, field)))
						}
					}
				}
			}
		}
	} else {
		// HTTP 함수에서 message 사용하면 ERROR
		for i, seq := range sf.Sequences {
			for _, val := range seq.Inputs {
				if strings.HasPrefix(val, "message.") {
					ctx := errCtx{sf.FileName, sf.Name, i}
					errs = append(errs, ctx.err("@sequence", "HTTP 함수에서 message를 사용할 수 없습니다 — @subscribe 함수에서만 사용 가능합니다"))
					break
				}
			}
		}
	}

	return errs
}

// hasStructField는 struct에 지정된 필드가 존재하는지 확인한다.
func hasStructField(structs []parser.StructInfo, typeName, fieldName string) bool {
	for _, si := range structs {
		if si.Name == typeName {
			for _, f := range si.Fields {
				if f.Name == fieldName {
					return true
				}
			}
			return false
		}
	}
	return false
}

// argVarRef는 Arg가 변수 참조인 경우 루트 변수명을 반환한다.
func argVarRef(a parser.Arg) string {
	if a.Literal != "" {
		return ""
	}
	if a.Source == "request" || a.Source == "currentUser" || a.Source == "query" || a.Source == "" {
		return ""
	}
	return a.Source
}

// rootVar는 "project.OwnerEmail" → "project"
func rootVar(s string) string {
	if idx := strings.Index(s, "."); idx >= 0 {
		return s[:idx]
	}
	return s
}

type errCtx struct {
	fileName string
	funcName string
	seqIndex int
}

func (c errCtx) err(tag, msg string) ValidationError {
	return ValidationError{
		FileName: c.fileName,
		FuncName: c.funcName,
		SeqIndex: c.seqIndex,
		Tag:      tag,
		Message:  msg,
	}
}

// primitiveTypes는 Go 기본 타입 집합이다.
var primitiveTypes = map[string]bool{
	"string": true, "int": true, "int32": true, "int64": true,
	"float32": true, "float64": true, "bool": true, "byte": true,
	"rune": true, "time.Time": true,
}

// isPrimitiveType는 Go 기본 타입 여부를 반환한다.
func isPrimitiveType(s string) bool {
	return primitiveTypes[s]
}
