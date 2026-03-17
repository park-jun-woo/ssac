package generator

import (
	"bytes"
	"fmt"
	"go/format"

	"github.com/park-jun-woo/ssac/parser"
	"github.com/park-jun-woo/ssac/validator"
)

// generateHTTPFunc는 HTTP 핸들러 함수를 생성한다.
func (g *GoTarget) generateHTTPFunc(sf parser.ServiceFunc, st *validator.SymbolTable) ([]byte, error) {
	// 분석
	pathParams := getPathParams(sf.Name, st)
	pathParamSet := map[string]bool{}
	for _, pp := range pathParams {
		pathParamSet[pp.Name] = true
	}

	requestParams := collectRequestParams(sf.Sequences, st, pathParamSet)
	needsCU := needsCurrentUser(sf.Sequences)
	needsQO := needsQueryOpts(sf, st)

	pkgName := "service"
	if sf.Domain != "" {
		pkgName = sf.Domain
	}

	// 1. 함수 본문 생성
	var bodyBuf bytes.Buffer

	fmt.Fprintf(&bodyBuf, "func (h *Handler) %s(c *gin.Context) {\n", sf.Name)

	for _, pp := range pathParams {
		bodyBuf.WriteString(generatePathParamCode(pp))
	}
	if len(pathParams) > 0 {
		bodyBuf.WriteString("\n")
	}

	if needsCU {
		var cuBuf bytes.Buffer
		goTemplates.ExecuteTemplate(&cuBuf, "currentUser", nil)
		bodyBuf.Write(cuBuf.Bytes())
		bodyBuf.WriteString("\n")
	}

	for _, rp := range requestParams {
		bodyBuf.WriteString(rp.extractCode)
	}
	if len(requestParams) > 0 {
		bodyBuf.WriteString("\n")
	}

	if needsQO {
		bodyBuf.WriteString(generateQueryOptsCode(sf.Name, st))
		bodyBuf.WriteString("\n")
	}

	useTx := hasWriteSequence(sf.Sequences)
	if useTx {
		bodyBuf.WriteString("\ttx, err := h.DB.BeginTx(c.Request.Context(), nil)\n")
		bodyBuf.WriteString("\tif err != nil {\n")
		bodyBuf.WriteString("\t\tc.JSON(http.StatusInternalServerError, gin.H{\"error\": \"transaction failed\"})\n")
		bodyBuf.WriteString("\t\treturn\n")
		bodyBuf.WriteString("\t}\n")
		bodyBuf.WriteString("\tdefer tx.Rollback()\n\n")
	}

	resultTypes := map[string]string{}
	for _, seq := range sf.Sequences {
		if seq.Result != nil {
			resultTypes[seq.Result.Var] = seq.Result.Type
		}
	}

	errDeclared := hasConversionErr(requestParams)
	if useTx {
		errDeclared = true
	}
	declaredVars := map[string]bool{}
	funcHasTotal := false
	usedVars := collectUsedVars(sf.Sequences)
	committed := false
	for i, seq := range sf.Sequences {
		if useTx && seq.Type == parser.SeqResponse && !committed {
			bodyBuf.WriteString("\tif err = tx.Commit(); err != nil {\n")
			bodyBuf.WriteString("\t\tc.JSON(http.StatusInternalServerError, gin.H{\"error\": \"commit failed\"})\n")
			bodyBuf.WriteString("\t\treturn\n")
			bodyBuf.WriteString("\t}\n\n")
			committed = true
		}
		data := buildTemplateData(seq, &errDeclared, declaredVars, resultTypes, st, sf.Name, useTx)
		if data.HasTotal {
			funcHasTotal = true
		}
		if seq.Type == parser.SeqResponse {
			data.HasTotal = funcHasTotal
		}
		if seq.Result != nil && !usedVars[seq.Result.Var] {
			data.Unused = true
			if data.ErrDeclared {
				data.ReAssign = true
			}
		}

		tmplName := templateName(seq)
		var seqBuf bytes.Buffer
		if err := goTemplates.ExecuteTemplate(&seqBuf, tmplName, data); err != nil {
			return nil, fmt.Errorf("sequence[%d] %s 템플릿 실행 실패: %w", i, seq.Type, err)
		}
		bodyBuf.Write(seqBuf.Bytes())
		bodyBuf.WriteString("\n")
	}

	if useTx && !committed {
		bodyBuf.WriteString("\tif err = tx.Commit(); err != nil {\n")
		bodyBuf.WriteString("\t\tc.JSON(http.StatusInternalServerError, gin.H{\"error\": \"commit failed\"})\n")
		bodyBuf.WriteString("\t\treturn\n")
		bodyBuf.WriteString("\t}\n\n")
	}

	bodyBuf.WriteString("}\n")

	// 2. 후보 import 수집 → 본문 기준 필터링
	imports := collectImports(sf, requestParams, pathParams, needsCU, needsQO)
	imports = filterUsedImports(imports, bodyBuf.String())

	// 3. 최종 조립
	var buf bytes.Buffer
	buf.WriteString("package " + pkgName + "\n\n")
	if len(imports) > 0 {
		buf.WriteString("import (\n")
		for _, imp := range imports {
			fmt.Fprintf(&buf, "\t%q\n", imp)
		}
		buf.WriteString(")\n\n")
	}
	buf.Write(bodyBuf.Bytes())

	formatted, err := format.Source(buf.Bytes())
	if err != nil {
		return buf.Bytes(), fmt.Errorf("gofmt 실패: %w\n--- raw ---\n%s", err, buf.String())
	}
	return formatted, nil
}

// generateSubscribeFunc는 큐 구독 핸들러 함수를 생성한다.
func (g *GoTarget) generateSubscribeFunc(sf parser.ServiceFunc, st *validator.SymbolTable) ([]byte, error) {
	pkgName := "service"
	if sf.Domain != "" {
		pkgName = sf.Domain
	}

	// 1. 함수 본문 생성
	var bodyBuf bytes.Buffer

	msgType := sf.Subscribe.MessageType
	fmt.Fprintf(&bodyBuf, "func (h *Handler) %s(ctx context.Context, message %s) error {\n", sf.Name, msgType)

	resultTypes := map[string]string{}
	for _, seq := range sf.Sequences {
		if seq.Result != nil {
			resultTypes[seq.Result.Var] = seq.Result.Type
		}
	}

	useTx := hasWriteSequence(sf.Sequences)
	if useTx {
		bodyBuf.WriteString("\ttx, err := h.DB.BeginTx(ctx, nil)\n")
		bodyBuf.WriteString("\tif err != nil {\n")
		bodyBuf.WriteString("\t\treturn fmt.Errorf(\"transaction failed: %w\", err)\n")
		bodyBuf.WriteString("\t}\n")
		bodyBuf.WriteString("\tdefer tx.Rollback()\n\n")
	}

	errDeclared := useTx
	declaredVars := map[string]bool{}
	usedVars := collectUsedVars(sf.Sequences)
	for i, seq := range sf.Sequences {
		data := buildTemplateData(seq, &errDeclared, declaredVars, resultTypes, st, sf.Name, useTx)
		if seq.Result != nil && !usedVars[seq.Result.Var] {
			data.Unused = true
			if data.ErrDeclared {
				data.ReAssign = true
			}
		}
		tmplName := subscribeTemplateName(seq)
		var seqBuf bytes.Buffer
		if err := goTemplates.ExecuteTemplate(&seqBuf, tmplName, data); err != nil {
			return nil, fmt.Errorf("sequence[%d] %s 템플릿 실행 실패: %w", i, seq.Type, err)
		}
		bodyBuf.Write(seqBuf.Bytes())
		bodyBuf.WriteString("\n")
	}

	if useTx {
		bodyBuf.WriteString("\tif err = tx.Commit(); err != nil {\n")
		bodyBuf.WriteString("\t\treturn fmt.Errorf(\"commit failed: %w\", err)\n")
		bodyBuf.WriteString("\t}\n\n")
	}

	bodyBuf.WriteString("\treturn nil\n")
	bodyBuf.WriteString("}\n")

	// 2. 후보 import 수집 → 본문 기준 필터링
	imports := collectSubscribeImports(sf)
	imports = filterUsedImports(imports, bodyBuf.String())

	// 3. 최종 조립
	var buf bytes.Buffer
	buf.WriteString("package " + pkgName + "\n\n")
	if len(imports) > 0 {
		buf.WriteString("import (\n")
		for _, imp := range imports {
			fmt.Fprintf(&buf, "\t%q\n", imp)
		}
		buf.WriteString(")\n\n")
	}
	buf.Write(bodyBuf.Bytes())

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

// templateName은 HTTP 함수 내 시퀀스의 템플릿 이름을 반환한다.
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
