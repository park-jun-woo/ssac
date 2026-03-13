package generator

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/geul-org/ssac/parser"
	"github.com/geul-org/ssac/validator"
	"github.com/ettle/strcase"
)

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

func buildTemplateData(seq parser.Sequence, errDeclared *bool, declaredVars map[string]bool, resultTypes map[string]string, st *validator.SymbolTable, funcName string, useTx bool) templateData {
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
			} else if errCode := lookupCallErrStatus(st, seq.Model); errCode != 0 {
				d.ErrStatus = httpStatusConst(errCode)
			} else {
				d.ErrStatus = "http.StatusInternalServerError"
			}
		} else {
			modelRef := "h." + strcase.ToGoPascal(parts[0]) + "Model"
			if useTx {
				modelRef += ".WithTx(tx)"
			}
			d.ModelCall = modelRef + "." + parts[1]
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

// --- helpers ---

func needsCurrentUser(seqs []parser.Sequence) bool {
	for _, seq := range seqs {
		// @auth는 항상 currentUser.ID, currentUser.Role을 사용
		if seq.Type == parser.SeqAuth {
			return true
		}
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

func generateQueryOptsCode(funcName string, st *validator.SymbolTable) string {
	if st == nil {
		return "\topts := model.ParseQueryOpts(c, model.QueryOptsConfig{})\n"
	}

	op, hasOp := st.Operations[funcName]
	if !hasOp {
		return "\topts := model.ParseQueryOpts(c, model.QueryOptsConfig{})\n"
	}

	var buf bytes.Buffer
	buf.WriteString("\topts := model.ParseQueryOpts(c, model.QueryOptsConfig{\n")

	if op.XPagination != nil {
		fmt.Fprintf(&buf, "\t\tPagination: &model.PaginationConfig{Style: %q, DefaultLimit: %d, MaxLimit: %d},\n",
			op.XPagination.Style, op.XPagination.DefaultLimit, op.XPagination.MaxLimit)
	}
	if op.XSort != nil {
		buf.WriteString("\t\tSort: &model.SortConfig{")
		if len(op.XSort.Allowed) > 0 {
			buf.WriteString("Allowed: []string{")
			for i, col := range op.XSort.Allowed {
				if i > 0 {
					buf.WriteString(", ")
				}
				fmt.Fprintf(&buf, "%q", col)
			}
			buf.WriteString("}")
		}
		if op.XSort.Default != "" {
			fmt.Fprintf(&buf, ", Default: %q", op.XSort.Default)
		}
		if op.XSort.Direction != "" {
			fmt.Fprintf(&buf, ", Direction: %q", op.XSort.Direction)
		}
		buf.WriteString("},\n")
	}
	if op.XFilter != nil && len(op.XFilter.Allowed) > 0 {
		buf.WriteString("\t\tFilter: &model.FilterConfig{Allowed: []string{")
		for i, col := range op.XFilter.Allowed {
			if i > 0 {
				buf.WriteString(", ")
			}
			fmt.Fprintf(&buf, "%q", col)
		}
		buf.WriteString("}},\n")
	}

	buf.WriteString("\t})\n")
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

func hasWriteSequence(seqs []parser.Sequence) bool {
	for _, seq := range seqs {
		switch seq.Type {
		case parser.SeqPost, parser.SeqPut, parser.SeqDelete:
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

// filterUsedImports는 생성된 코드 본문에서 실제 참조되는 패키지만 남긴다.
func filterUsedImports(imports []string, body string) []string {
	var used []string
	for _, imp := range imports {
		pkg := imp
		if idx := strings.LastIndex(imp, "/"); idx >= 0 {
			pkg = imp[idx+1:]
		}
		if strings.Contains(body, pkg+".") {
			used = append(used, imp)
		}
	}
	return used
}

// lookupCallErrStatus는 SymbolTable에서 @call 대상 함수의 @error 어노테이션 값을 조회한다.
func lookupCallErrStatus(st *validator.SymbolTable, model string) int {
	if st == nil {
		return 0
	}
	parts := strings.SplitN(model, ".", 2)
	if len(parts) < 2 {
		return 0
	}
	pkgName, funcName := parts[0], parts[1]
	for modelKey, ms := range st.Models {
		if !strings.HasPrefix(modelKey, pkgName+".") {
			continue
		}
		if mi, ok := ms.Methods[funcName]; ok {
			return mi.ErrStatus
		}
	}
	return 0
}
