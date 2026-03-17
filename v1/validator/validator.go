package validator

import (
	"fmt"
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
		errs = append(errs, validateResponse(sf, st)...)
	}
	return errs
}

func validateFunc(sf parser.ServiceFunc) []ValidationError {
	var errs []ValidationError
	errs = append(errs, validateRequiredFields(sf)...)
	errs = append(errs, validateVariableFlow(sf)...)
	errs = append(errs, validateStaleResponse(sf)...)
	return errs
}

// validateModel은 @model이 심볼 테이블에 존재하는지 검증한다.
func validateModel(sf parser.ServiceFunc, st *SymbolTable) []ValidationError {
	var errs []ValidationError
	for i, seq := range sf.Sequences {
		if seq.Model == "" {
			continue
		}
		ctx := errCtx{sf.FileName, sf.Name, i}
		parts := strings.SplitN(seq.Model, ".", 2)
		if len(parts) < 2 {
			continue // 형식 에러는 내부 검증에서 처리
		}
		modelName, methodName := parts[0], parts[1]

		ms, ok := st.Models[modelName]
		if !ok {
			errs = append(errs, ctx.err("@model", fmt.Sprintf("%q 모델을 찾을 수 없습니다", modelName)))
			continue
		}
		if !ms.HasMethod(methodName) {
			errs = append(errs, ctx.err("@model", fmt.Sprintf("%q 모델에 %q 메서드가 없습니다", modelName, methodName)))
		}
	}
	return errs
}

// validateRequest는 source가 "request"인 @param이 OpenAPI request에 존재하는지 검증한다.
func validateRequest(sf parser.ServiceFunc, st *SymbolTable) []ValidationError {
	var errs []ValidationError
	op, ok := st.Operations[sf.Name]
	if !ok {
		return nil // 매칭되는 operation이 없으면 스킵
	}

	// SSaC에서 사용하는 request 필드 수집
	usedRequestFields := make(map[string]bool)
	for i, seq := range sf.Sequences {
		ctx := errCtx{sf.FileName, sf.Name, i}
		for _, p := range seq.Params {
			if p.Source != "request" {
				continue
			}
			usedRequestFields[p.Name] = true
			// 정방향: SSaC @param → OpenAPI request
			if !op.RequestFields[p.Name] {
				errs = append(errs, ctx.err("@param", fmt.Sprintf("OpenAPI request에 %q 필드가 없습니다", p.Name)))
			}
		}
	}

	// 역방향: OpenAPI request → SSaC @param
	// path parameter는 제외 (라우팅에서 자동 바인딩)
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
				Tag:      "@param",
				Message:  fmt.Sprintf("OpenAPI request에 %q 필드가 있지만 SSaC @param에서 사용하지 않습니다", field),
				Level:    "WARNING",
			})
		}
	}

	return errs
}

// validateResponse는 @var가 OpenAPI response에 존재하는지 검증한다.
func validateResponse(sf parser.ServiceFunc, st *SymbolTable) []ValidationError {
	var errs []ValidationError
	op, ok := st.Operations[sf.Name]
	if !ok {
		return nil
	}

	var responseVars map[string]bool
	var responseSeqIdx int
	for i, seq := range sf.Sequences {
		if !strings.HasPrefix(seq.Type, "response") {
			continue
		}
		responseSeqIdx = i
		responseVars = make(map[string]bool)
		for _, v := range seq.Vars {
			responseVars[v] = true
		}

		// 정방향: SSaC @var → OpenAPI response
		ctx := errCtx{sf.FileName, sf.Name, i}
		for _, v := range seq.Vars {
			if !op.ResponseFields[v] {
				errs = append(errs, ctx.err("@var", fmt.Sprintf("OpenAPI response에 %q 필드가 없습니다", v)))
			}
		}
	}

	// 역방향: OpenAPI response → SSaC @var
	if responseVars != nil && len(op.ResponseFields) > 0 {
		ctx := errCtx{sf.FileName, sf.Name, responseSeqIdx}
		for field := range op.ResponseFields {
			// x-pagination이 있으면 "total"은 코드젠이 자동 생성하므로 제외
			if field == "total" && op.XPagination != nil {
				continue
			}
			if !responseVars[field] {
				errs = append(errs, ValidationError{
					FileName: ctx.fileName,
					FuncName: ctx.funcName,
					SeqIndex: ctx.seqIndex,
					Tag:      "@var",
					Message:  fmt.Sprintf("OpenAPI response에 %q 필드가 있지만 SSaC @var에 없습니다\n  → @sequence response json에 // @var %s를 추가하고, %s를 조회하는 시퀀스도 함께 작성하세요", field, field, field),
				})
			}
		}
	}

	return errs
}

// validateRequiredFields는 타입별 필수 태그 누락을 검증한다.
func validateRequiredFields(sf parser.ServiceFunc) []ValidationError {
	var errs []ValidationError

	for i, seq := range sf.Sequences {
		ctx := errCtx{sf.FileName, sf.Name, i}

		switch seq.Type {
		case parser.SeqAuthorize:
			if seq.Action == "" {
				errs = append(errs, ctx.err("@action", "누락"))
			}
			if seq.Resource == "" {
				errs = append(errs, ctx.err("@resource", "누락"))
			}
			if seq.ID == "" {
				errs = append(errs, ctx.err("@id", "누락"))
			}

		case parser.SeqGet, parser.SeqPost:
			if seq.Model == "" {
				errs = append(errs, ctx.err("@model", "누락"))
			}
			if seq.Result == nil {
				errs = append(errs, ctx.err("@result", "누락"))
			}

		case parser.SeqPut, parser.SeqDelete:
			if seq.Model == "" {
				errs = append(errs, ctx.err("@model", "누락"))
			}

		case parser.SeqGuardNil, parser.SeqGuardExists:
			if seq.Target == "" {
				errs = append(errs, ctx.err("@sequence", "guard 대상 변수 누락"))
			}

		case parser.SeqGuardState:
			if seq.Target == "" {
				errs = append(errs, ctx.err("@sequence", "guard state 대상 누락"))
			}
			if len(seq.Params) != 1 {
				errs = append(errs, ctx.err("@param", fmt.Sprintf("guard state는 @param 1개 필요, %d개 있음", len(seq.Params))))
			} else if !strings.Contains(seq.Params[0].Name, ".") {
				errs = append(errs, ctx.err("@param", "entity.Field 형식이어야 함"))
			}

		case parser.SeqCall:
			if seq.Func == "" {
				errs = append(errs, ctx.err("@func", "누락"))
			}
			if seq.Func != "" && seq.Package == "" {
				errs = append(errs, ctx.err("@func", "@func는 package.funcName 형식이어야 함"))
			}

		default:
			if strings.HasPrefix(seq.Type, "response") {
				// response는 필수 필드 없음 (vars는 선택)
			} else {
				errs = append(errs, ctx.err("@sequence", fmt.Sprintf("알 수 없는 타입: %q", seq.Type)))
			}
		}

		// @model 형식 검증: "Model.Method" 패턴
		if seq.Model != "" {
			parts := strings.SplitN(seq.Model, ".", 2)
			if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
				errs = append(errs, ctx.err("@model", fmt.Sprintf("\"Model.Method\" 형식이어야 함: %q", seq.Model)))
			}
		}
	}

	return errs
}

// validateVariableFlow는 변수가 선언 전에 참조되지 않는지 검증한다.
func validateVariableFlow(sf parser.ServiceFunc) []ValidationError {
	var errs []ValidationError
	declared := map[string]bool{}

	// 예약어: 선언 없이 사용 가능
	declared["currentUser"] = true
	declared["config"] = true

	for i, seq := range sf.Sequences {
		ctx := errCtx{sf.FileName, sf.Name, i}

		// guard: Target 참조 검증 (guard state의 Target은 stateDiagramID이므로 제외)
		if seq.Target != "" && seq.Type != parser.SeqGuardState && !declared[seq.Target] {
			errs = append(errs, ctx.err("guard", fmt.Sprintf("%q 변수가 아직 선언되지 않았습니다", seq.Target)))
		}

		// @param source가 변수 참조인 경우 검증
		for _, p := range seq.Params {
			ref := paramVarRef(p)
			if ref != "" && !declared[ref] {
				errs = append(errs, ctx.err("@param", fmt.Sprintf("%q 변수가 아직 선언되지 않았습니다", ref)))
			}
		}

		// @var 참조 검증 (response)
		for _, v := range seq.Vars {
			if !declared[v] {
				errs = append(errs, ctx.err("@var", fmt.Sprintf("%q 변수가 아직 선언되지 않았습니다", v)))
			}
		}

		// @result로 변수 선언
		if seq.Result != nil {
			declared[seq.Result.Var] = true
		}
	}

	return errs
}

// validateStaleResponse는 put/delete 이후 갱신 없이 response에서 사용되는 변수를 경고한다.
func validateStaleResponse(sf parser.ServiceFunc) []ValidationError {
	var errs []ValidationError

	// get으로 가져온 변수 → 모델명
	getVars := map[string]string{}
	// put/delete로 변경된 모델
	mutated := map[string]bool{}

	for i, seq := range sf.Sequences {
		switch seq.Type {
		case parser.SeqGet:
			if seq.Result != nil && seq.Model != "" {
				modelName := strings.SplitN(seq.Model, ".", 2)[0]
				getVars[seq.Result.Var] = modelName
				// 재조회: 이 모델의 mutation 플래그 해제
				mutated[modelName] = false
			}
		case parser.SeqPut, parser.SeqDelete:
			if seq.Model != "" {
				modelName := strings.SplitN(seq.Model, ".", 2)[0]
				mutated[modelName] = true
			}
		}

		if strings.HasPrefix(seq.Type, "response") {
			ctx := errCtx{sf.FileName, sf.Name, i}
			for _, v := range seq.Vars {
				if modelName, ok := getVars[v]; ok && mutated[modelName] {
					errs = append(errs, ValidationError{
						FileName: ctx.fileName,
						FuncName: ctx.funcName,
						SeqIndex: ctx.seqIndex,
						Tag:      "@var",
						Message:  fmt.Sprintf("%q가 %s 수정 이후 갱신 없이 response에 사용됩니다", v, modelName),
						Level:    "WARNING",
					})
				}
			}
		}
	}

	return errs
}

// paramVarRef는 Param이 변수 참조인 경우 루트 변수명을 반환한다.
// "request" source → 변수 참조 아님 (빈 문자열)
// "project" (source 없음) → "project"
// "project.OwnerEmail" (source 없음) → "project"
// "\"리터럴\"" → 변수 참조 아님
func paramVarRef(p parser.Param) string {
	// source가 "request"이면 외부 입력 → 변수 참조 아님
	if p.Source == "request" {
		return ""
	}
	// source가 있으면 그게 변수 참조
	if p.Source != "" {
		return rootVar(p.Source)
	}
	// source 없고 Name이 리터럴이면 무시
	if strings.HasPrefix(p.Name, `"`) {
		return ""
	}
	// source 없으면 Name이 변수 참조
	return rootVar(p.Name)
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
