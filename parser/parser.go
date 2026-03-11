package parser

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strings"
)

// ParseDir은 디렉토리 내 모든 .ssac 파일을 재귀 탐색하여 []ServiceFunc를 반환한다.
func ParseDir(dir string) ([]ServiceFunc, error) {
	var funcs []ServiceFunc
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(d.Name(), ".ssac") {
			return err
		}
		sfs, err := ParseFile(path)
		if err != nil {
			return fmt.Errorf("%s 파싱 실패: %w", path, err)
		}
		rel, _ := filepath.Rel(dir, path)
		if filepath.Dir(rel) == "." {
			return fmt.Errorf("%s — service/ 직접에 SSaC 파일을 둘 수 없습니다. 도메인 서브 폴더를 사용하세요 (예: service/auth/%s)", d.Name(), d.Name())
		}
		for i := range sfs {
			parts := strings.Split(filepath.Dir(rel), string(filepath.Separator))
			sfs[i].Domain = parts[0]
			funcs = append(funcs, sfs[i])
		}
		return nil
	})
	return funcs, err
}

// ParseFile은 단일 .ssac 파일을 파싱하여 []ServiceFunc를 반환한다.
func ParseFile(path string) ([]ServiceFunc, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("Go 파싱 실패: %w", err)
	}

	imports := collectImports(f)
	var funcs []ServiceFunc

	for _, decl := range f.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}

		var comments []*ast.Comment
		fnPos := fn.Pos()
		for _, cg := range f.Comments {
			if cg.End() < fnPos {
				comments = append(comments, cg.List...)
			}
		}

		sequences, err := parseComments(comments)
		if err != nil {
			return nil, fmt.Errorf("%s:%s — %w", filepath.Base(path), fn.Name.Name, err)
		}
		if len(sequences) == 0 {
			continue
		}

		funcs = append(funcs, ServiceFunc{
			Name:      fn.Name.Name,
			FileName:  filepath.Base(path),
			Imports:   imports,
			Sequences: sequences,
		})
		// 다음 함수를 위해 comments를 리셋하지 않아도 됨 — cg.End() < fnPos 체크가 함수별로 필터링
	}
	return funcs, nil
}

// parseComments는 주석 리스트에서 v2 시퀀스를 추출한다.
func parseComments(comments []*ast.Comment) ([]Sequence, error) {
	var sequences []Sequence
	var responseLines []string
	inResponse := false
	responseSuppressWarn := false

	for _, c := range comments {
		line := strings.TrimPrefix(c.Text, "//")
		line = strings.TrimSpace(line)

		if inResponse {
			if line == "}" {
				inResponse = false
				seq := Sequence{
					Type:         SeqResponse,
					Fields:       parseResponseFields(responseLines),
					SuppressWarn: responseSuppressWarn,
				}
				sequences = append(sequences, seq)
				responseLines = nil
				continue
			}
			responseLines = append(responseLines, line)
			continue
		}

		if !strings.HasPrefix(line, "@") {
			continue
		}

		seq, isResponseStart, err := parseLine(line)
		if err != nil {
			return nil, err
		}
		if isResponseStart {
			inResponse = true
			responseSuppressWarn = strings.HasPrefix(line, "@response!")
			responseLines = nil
			continue
		}
		if seq != nil {
			sequences = append(sequences, *seq)
		}
	}
	return sequences, nil
}

// parseLine은 한 줄을 파싱하여 Sequence를 반환한다.
// @response { 의 경우 (nil, true, nil)를 반환하여 멀티라인 모드 시작을 알린다.
func parseLine(line string) (*Sequence, bool, error) {
	if strings.HasPrefix(line, "@response") {
		tag := "@response"
		suppressWarn := false
		if strings.HasPrefix(line, "@response!") {
			tag = "@response!"
			suppressWarn = true
		}
		trimmed := strings.TrimSpace(strings.TrimPrefix(line, tag))
		if trimmed == "{" {
			return nil, true, nil
		}
		// @response 간단쓰기: @response varName
		if trimmed != "" && trimmed != "{" {
			return &Sequence{
				Type:         SeqResponse,
				Target:       trimmed,
				SuppressWarn: suppressWarn,
			}, false, nil
		}
	}

	// @type! — ! 접미사 감지
	suppressWarn := false
	if idx := strings.IndexByte(line, '!'); idx > 0 {
		spaceIdx := strings.IndexByte(line, ' ')
		if spaceIdx < 0 || idx < spaceIdx {
			line = line[:idx] + line[idx+1:]
			suppressWarn = true
		}
	}

	var seq *Sequence
	var err error
	switch {
	case strings.HasPrefix(line, "@get "):
		seq, err = parseCRUD(SeqGet, line[5:], true)
	case strings.HasPrefix(line, "@post "):
		seq, err = parseCRUD(SeqPost, line[6:], true)
	case strings.HasPrefix(line, "@put "):
		seq, err = parseCRUD(SeqPut, line[5:], false)
	case strings.HasPrefix(line, "@delete "):
		seq, err = parseCRUD(SeqDelete, line[8:], false)
	case strings.HasPrefix(line, "@empty "):
		seq = parseGuard(SeqEmpty, line[7:])
	case strings.HasPrefix(line, "@exists "):
		seq = parseGuard(SeqExists, line[8:])
	case strings.HasPrefix(line, "@state "):
		seq, err = parseState(line[7:])
	case strings.HasPrefix(line, "@auth "):
		seq, err = parseAuth(line[6:])
	case strings.HasPrefix(line, "@call "):
		seq, err = parseCall(line[6:])
	default:
		return nil, false, nil
	}

	if err != nil {
		return nil, false, err
	}
	if seq != nil && suppressWarn {
		seq.SuppressWarn = true
	}
	return seq, false, nil
}

// parseCRUD는 @get/@post/@put/@delete를 파싱한다.
// hasResult=true: Type var = Model.Method({Key: val, ...})
// hasResult=false: Model.Method({Key: val, ...})
func parseCRUD(seqType, rest string, hasResult bool) (*Sequence, error) {
	rest = strings.TrimSpace(rest)
	seq := &Sequence{Type: seqType}

	if hasResult {
		// Type var = Model.Method({Key: val, ...})
		eqIdx := strings.Index(rest, "=")
		if eqIdx < 0 {
			return nil, nil
		}
		lhs := strings.TrimSpace(rest[:eqIdx])
		rhs := strings.TrimSpace(rest[eqIdx+1:])

		result := parseResult(lhs)
		if result == nil {
			return nil, nil
		}
		seq.Result = result

		model, inputs, err := parseCallExprInputs(rhs)
		if err != nil {
			return nil, err
		}
		seq.Package, seq.Model = splitPackagePrefix(model)
		seq.Inputs = inputs
	} else {
		// Model.Method({Key: val, ...})
		model, inputs, err := parseCallExprInputs(rest)
		if err != nil {
			return nil, err
		}
		seq.Package, seq.Model = splitPackagePrefix(model)
		seq.Inputs = inputs
	}

	return seq, nil
}

// parseGuard는 @empty/@exists를 파싱한다.
// target "message"
func parseGuard(seqType, rest string) *Sequence {
	rest = strings.TrimSpace(rest)
	target, msg := splitTargetMessage(rest)
	return &Sequence{
		Type:    seqType,
		Target:  target,
		Message: msg,
	}
}

// parseState는 @state를 파싱한다.
// diagramID {inputs} "transition" "message"
func parseState(rest string) (*Sequence, error) {
	rest = strings.TrimSpace(rest)

	// diagramID
	spaceIdx := strings.IndexByte(rest, ' ')
	if spaceIdx < 0 {
		return nil, nil
	}
	diagramID := rest[:spaceIdx]
	rest = strings.TrimSpace(rest[spaceIdx+1:])

	// {inputs}
	inputs, rest, err := extractInputs(rest)
	if err != nil {
		return nil, err
	}

	// "transition" "message"
	transition, msg := parseTwoQuoted(rest)

	return &Sequence{
		Type:       SeqState,
		DiagramID:  diagramID,
		Inputs:     inputs,
		Transition: transition,
		Message:    msg,
	}, nil
}

// parseAuth는 @auth를 파싱한다.
// "action" "resource" {inputs} "message"
func parseAuth(rest string) (*Sequence, error) {
	rest = strings.TrimSpace(rest)

	// "action"
	action, rest := extractQuoted(rest)
	rest = strings.TrimSpace(rest)

	// "resource"
	resource, rest := extractQuoted(rest)
	rest = strings.TrimSpace(rest)

	// {inputs}
	inputs, rest, err := extractInputs(rest)
	if err != nil {
		return nil, err
	}

	// "message"
	msg, _ := extractQuoted(strings.TrimSpace(rest))

	return &Sequence{
		Type:     SeqAuth,
		Action:   action,
		Resource: resource,
		Inputs:   inputs,
		Message:  msg,
	}, nil
}

// parseCall은 @call을 파싱한다.
// Type var = pkg.Func({Key: val, ...}) 또는 pkg.Func({Key: val, ...})
func parseCall(rest string) (*Sequence, error) {
	rest = strings.TrimSpace(rest)
	seq := &Sequence{Type: SeqCall}

	// = 가 있고, 그 전에 ( 가 없으면 result 있는 형태
	eqIdx := strings.Index(rest, "=")
	parenIdx := strings.Index(rest, "(")
	if eqIdx > 0 && (parenIdx < 0 || eqIdx < parenIdx) {
		lhs := strings.TrimSpace(rest[:eqIdx])
		rhs := strings.TrimSpace(rest[eqIdx+1:])

		result := parseResult(lhs)
		if result == nil {
			return nil, nil
		}
		seq.Result = result

		model, inputs, err := parseCallExprInputs(rhs)
		if err != nil {
			return nil, err
		}
		seq.Model = model
		seq.Inputs = inputs
	} else {
		model, inputs, err := parseCallExprInputs(rest)
		if err != nil {
			return nil, err
		}
		seq.Model = model
		seq.Inputs = inputs
	}

	return seq, nil
}

// parseCallExprInputs는 "pkg.Func({Key: val, ...})"를 파싱한다.
func parseCallExprInputs(expr string) (string, map[string]string, error) {
	expr = strings.TrimSpace(expr)
	parenIdx := strings.Index(expr, "(")
	if parenIdx < 0 {
		return expr, nil, nil
	}
	model := expr[:parenIdx]
	inner := expr[parenIdx+1:]
	inner = strings.TrimSuffix(strings.TrimSpace(inner), ")")
	inner = strings.TrimSpace(inner)
	if inner == "" {
		return model, nil, nil
	}
	inputs, err := parseInputs(inner)
	return model, inputs, err
}

// parseResult는 "Type var" 또는 "[]Type var"를 파싱한다.
func parseResult(lhs string) *Result {
	lhs = strings.TrimSpace(lhs)
	parts := strings.Fields(lhs)
	if len(parts) != 2 {
		return nil
	}
	typeName := parts[0]
	r := &Result{Var: parts[1]}

	// Page[Gig] → Wrapper="Page", Type="Gig"
	// Cursor[Gig] → Wrapper="Cursor", Type="Gig"
	if bracketIdx := strings.IndexByte(typeName, '['); bracketIdx > 0 {
		if strings.HasSuffix(typeName, "]") {
			r.Wrapper = typeName[:bracketIdx]
			r.Type = typeName[bracketIdx+1 : len(typeName)-1]
			return r
		}
	}

	r.Type = typeName
	return r
}

// parseCallExpr는 "Model.Method(args)" 또는 "pkg.Func(args)"를 파싱한다.
func parseCallExpr(expr string) (string, []Arg) {
	expr = strings.TrimSpace(expr)
	parenIdx := strings.Index(expr, "(")
	if parenIdx < 0 {
		return expr, nil
	}
	model := expr[:parenIdx]
	argsStr := expr[parenIdx+1:]
	argsStr = strings.TrimSuffix(strings.TrimSpace(argsStr), ")")
	argsStr = strings.TrimSpace(argsStr)
	if argsStr == "" {
		return model, nil
	}
	return model, parseArgs(argsStr)
}

// parseArgs는 쉼표 분리 인자를 파싱한다.
func parseArgs(s string) []Arg {
	parts := strings.Split(s, ",")
	var args []Arg
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		args = append(args, parseArg(p))
	}
	return args
}

// parseArg는 단일 인자를 파싱한다.
func parseArg(s string) Arg {
	s = strings.TrimSpace(s)
	// "literal"
	if strings.HasPrefix(s, `"`) && strings.HasSuffix(s, `"`) {
		return Arg{Literal: s[1 : len(s)-1]}
	}
	// source.Field
	dotIdx := strings.IndexByte(s, '.')
	if dotIdx > 0 {
		return Arg{Source: s[:dotIdx], Field: s[dotIdx+1:]}
	}
	// bare variable (shouldn't happen in valid syntax, but handle gracefully)
	return Arg{Source: s}
}

// parseResponseFields는 @response 블록 내부 라인을 파싱한다.
func parseResponseFields(lines []string) map[string]string {
	fields := make(map[string]string)
	for _, line := range lines {
		line = strings.TrimSpace(line)
		line = strings.TrimSuffix(line, ",")
		colonIdx := strings.IndexByte(line, ':')
		if colonIdx < 0 {
			continue
		}
		key := strings.TrimSpace(line[:colonIdx])
		val := strings.TrimSpace(line[colonIdx+1:])
		if key != "" && val != "" {
			fields[key] = val
		}
	}
	return fields
}

// parseInputs는 {key: value, ...} 형식을 파싱한다.
func parseInputs(s string) (map[string]string, error) {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "{")
	s = strings.TrimSuffix(s, "}")
	s = strings.TrimSpace(s)
	if s == "" {
		return map[string]string{}, nil
	}
	result := make(map[string]string)
	for _, pair := range strings.Split(s, ",") {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}
		colonIdx := strings.IndexByte(pair, ':')
		if colonIdx < 0 {
			return nil, fmt.Errorf("%q는 유효하지 않은 입력 형식입니다. \"{Key: value}\" 형식을 사용하세요", pair)
		}
		key := strings.TrimSpace(pair[:colonIdx])
		val := strings.TrimSpace(pair[colonIdx+1:])
		if key != "" && val != "" {
			result[key] = val
		}
	}
	return result, nil
}

// extractInputs는 문자열에서 {…} 블록을 추출하고 나머지를 반환한다.
func extractInputs(s string) (map[string]string, string, error) {
	openIdx := strings.IndexByte(s, '{')
	if openIdx < 0 {
		return map[string]string{}, s, nil
	}
	closeIdx := strings.IndexByte(s, '}')
	if closeIdx < 0 {
		return map[string]string{}, s, nil
	}
	inputStr := s[openIdx : closeIdx+1]
	rest := strings.TrimSpace(s[closeIdx+1:])
	inputs, err := parseInputs(inputStr)
	return inputs, rest, err
}

// extractQuoted는 문자열 앞의 "quoted" 값을 추출하고 나머지를 반환한다.
func extractQuoted(s string) (string, string) {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, `"`) {
		return "", s
	}
	endIdx := strings.IndexByte(s[1:], '"')
	if endIdx < 0 {
		return "", s
	}
	return s[1 : endIdx+1], strings.TrimSpace(s[endIdx+2:])
}

// parseTwoQuoted는 "first" "second"를 파싱한다.
func parseTwoQuoted(s string) (string, string) {
	s = strings.TrimSpace(s)
	first, rest := extractQuoted(s)
	second, _ := extractQuoted(rest)
	return first, second
}

// splitTargetMessage는 "target "message""를 분리한다.
func splitTargetMessage(s string) (string, string) {
	quoteIdx := strings.IndexByte(s, '"')
	if quoteIdx < 0 {
		return strings.TrimSpace(s), ""
	}
	target := strings.TrimSpace(s[:quoteIdx])
	msg, _ := extractQuoted(s[quoteIdx:])
	return target, msg
}

// splitPackagePrefix는 "session.Session.Get" → ("session", "Session.Get")로 분리한다.
// "Course.FindByID" → ("", "Course.FindByID") — 2-part는 패키지 없음.
// @call은 이미 pkg.Func 형식이므로 이 함수를 사용하지 않는다.
func splitPackagePrefix(model string) (pkg, rest string) {
	// dot 개수: 1개 → 기존 Model.Method, 2개 이상 → pkg.Model.Method
	firstDot := strings.IndexByte(model, '.')
	if firstDot < 0 {
		return "", model
	}
	secondDot := strings.IndexByte(model[firstDot+1:], '.')
	if secondDot < 0 {
		// "Model.Method" — no package prefix
		return "", model
	}
	// "pkg.Model.Method" — first part is package (lowercase check)
	pkg = model[:firstDot]
	if len(pkg) > 0 && pkg[0] >= 'a' && pkg[0] <= 'z' {
		return pkg, model[firstDot+1:]
	}
	// If first part starts with uppercase, it's not a package prefix
	return "", model
}

// collectImports는 AST에서 import 경로를 수집한다.
func collectImports(f *ast.File) []string {
	var imports []string
	for _, imp := range f.Imports {
		path := strings.Trim(imp.Path.Value, `"`)
		if path == "net/http" {
			continue
		}
		imports = append(imports, path)
	}
	return imports
}
