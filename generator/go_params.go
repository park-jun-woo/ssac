package generator

import (
	"bytes"
	"fmt"
	"sort"
	"strings"

	"github.com/park-jun-woo/ssac/parser"
	"github.com/park-jun-woo/ssac/validator"
	"github.com/ettle/strcase"
)

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
				seen["github.com/park-jun-woo/fullend/pkg/pagination"] = true
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
	if hasNonStringPathParam {
		seen["strconv"] = true
	}

	if needsCU || needsQO {
		seen["model"] = true
	}
	if hasWriteSequence(sf.Sequences) {
		seen["database/sql"] = true
	}

	var imports []string
	order := []string{"database/sql", "net/http", "strconv", "time"}
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
	if hasWriteSequence(sf.Sequences) {
		seen["database/sql"] = true
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
