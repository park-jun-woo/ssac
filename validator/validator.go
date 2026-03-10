package validator

import (
	"fmt"
	"strings"

	"github.com/geul-org/ssac/parser"
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
		errs = append(errs, validateResponse(sf, st)...)
		errs = append(errs, validateCurrentUserType(sf, st)...)
	}
	return errs
}

func validateFunc(sf parser.ServiceFunc) []ValidationError {
	var errs []ValidationError
	errs = append(errs, validateRequiredFields(sf)...)
	errs = append(errs, validateVariableFlow(sf)...)
	errs = append(errs, validateStaleResponse(sf)...)
	errs = append(errs, validateReservedSourceConflict(sf)...)
	return errs
}

// validateRequiredFields는 타입별 필수 필드 누락을 검증한다.
func validateRequiredFields(sf parser.ServiceFunc) []ValidationError {
	var errs []ValidationError

	for i, seq := range sf.Sequences {
		ctx := errCtx{sf.FileName, sf.Name, i}

		switch seq.Type {
		case parser.SeqGet, parser.SeqPost:
			if seq.Model == "" {
				errs = append(errs, ctx.err("@"+seq.Type, "Model 누락"))
			}
			if seq.Result == nil {
				errs = append(errs, ctx.err("@"+seq.Type, "Result 누락"))
			}
			if len(seq.Args) == 0 {
				errs = append(errs, ctx.err("@"+seq.Type, "Args 누락"))
			}

		case parser.SeqPut, parser.SeqDelete:
			if seq.Model == "" {
				errs = append(errs, ctx.err("@"+seq.Type, "Model 누락"))
			}
			if seq.Result != nil {
				errs = append(errs, ctx.err("@"+seq.Type, "Result는 nil이어야 함"))
			}
			if len(seq.Args) == 0 {
				errs = append(errs, ctx.err("@"+seq.Type, "Args 누락"))
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
		"config":      true,
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

		// @state/@auth Inputs value 검증
		for _, val := range seq.Inputs {
			ref := rootVar(val)
			if ref != "request" && ref != "currentUser" && ref != "" && !declared[ref] {
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

// validateResponse는 @response Fields가 OpenAPI response와 일치하는지 검증한다.
func validateResponse(sf parser.ServiceFunc, st *SymbolTable) []ValidationError {
	var errs []ValidationError
	op, ok := st.Operations[sf.Name]
	if !ok {
		return nil
	}

	var responseFields map[string]bool
	var responseSeqIdx int
	for i, seq := range sf.Sequences {
		if seq.Type != parser.SeqResponse {
			continue
		}
		responseSeqIdx = i
		responseFields = make(map[string]bool)
		ctx := errCtx{sf.FileName, sf.Name, i}

		// 정방향: SSaC @response → OpenAPI
		for field := range seq.Fields {
			responseFields[field] = true
			if !op.ResponseFields[field] {
				errs = append(errs, ctx.err("@response", fmt.Sprintf("OpenAPI response에 %q 필드가 없습니다", field)))
			}
		}
	}

	// 역방향: OpenAPI → SSaC @response
	if responseFields != nil && len(op.ResponseFields) > 0 {
		ctx := errCtx{sf.FileName, sf.Name, responseSeqIdx}
		for field := range op.ResponseFields {
			if field == "total" && op.XPagination != nil {
				continue
			}
			if !responseFields[field] {
				errs = append(errs, ValidationError{
					FileName: ctx.fileName,
					FuncName: ctx.funcName,
					SeqIndex: ctx.seqIndex,
					Tag:      "@response",
					Message:  fmt.Sprintf("OpenAPI response에 %q 필드가 있지만 SSaC @response에 없습니다\n  → @response에 %s 필드를 추가하고, %s를 조회하는 시퀀스도 함께 작성하세요", field, field, field),
				})
			}
		}
	}

	return errs
}

// reservedSources는 사용자가 result 변수명으로 사용할 수 없는 예약 소스다.
var reservedSources = map[string]bool{
	"request":     true,
	"currentUser": true,
	"config":      true,
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

// validateCurrentUserType는 currentUser를 사용하는데 model/에 CurrentUser 타입이 없으면 WARNING을 반환한다.
func validateCurrentUserType(sf parser.ServiceFunc, st *SymbolTable) []ValidationError {
	if st == nil {
		return nil
	}

	usesCurrentUser := false
	for _, seq := range sf.Sequences {
		if seq.Type == parser.SeqAuth {
			usesCurrentUser = true
			break
		}
		for _, a := range seq.Args {
			if a.Source == "currentUser" {
				usesCurrentUser = true
				break
			}
		}
		for _, val := range seq.Inputs {
			if strings.HasPrefix(val, "currentUser.") {
				usesCurrentUser = true
				break
			}
		}
		if usesCurrentUser {
			break
		}
	}

	if !usesCurrentUser {
		return nil
	}

	if !st.HasCurrentUserType {
		return []ValidationError{{
			FileName: sf.FileName,
			FuncName: sf.Name,
			SeqIndex: -1,
			Tag:      "@currentUser",
			Message:  "currentUser를 사용하지만 model/에 CurrentUser 타입이 정의되지 않았습니다",
			Level:    "WARNING",
		}}
	}
	return nil
}

// argVarRef는 Arg가 변수 참조인 경우 루트 변수명을 반환한다.
func argVarRef(a parser.Arg) string {
	if a.Literal != "" {
		return ""
	}
	if a.Source == "request" || a.Source == "currentUser" || a.Source == "" {
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
