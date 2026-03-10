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

// ParseDir은 디렉토리 내 모든 .go 파일을 재귀 탐색하여 []ServiceFunc를 반환한다.
func ParseDir(dir string) ([]ServiceFunc, error) {
	var funcs []ServiceFunc
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(d.Name(), ".go") {
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

// ParseFile은 단일 .go 파일을 파싱하여 []ServiceFunc를 반환한다.
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

		sequences := parseComments(comments)
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
func parseComments(comments []*ast.Comment) []Sequence {
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

		seq, isResponseStart := parseLine(line)
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
	return sequences
}

// parseLine은 한 줄을 파싱하여 Sequence를 반환한다.
// @response { 의 경우 (nil, true)를 반환하여 멀티라인 모드 시작을 알린다.
func parseLine(line string) (*Sequence, bool) {
	if strings.HasPrefix(line, "@response") {
		tag := "@response"
		if strings.HasPrefix(line, "@response!") {
			tag = "@response!"
		}
		trimmed := strings.TrimSpace(strings.TrimPrefix(line, tag))
		if trimmed == "{" {
			return nil, true
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
	switch {
	case strings.HasPrefix(line, "@get "):
		seq = parseCRUD(SeqGet, line[5:], true)
	case strings.HasPrefix(line, "@post "):
		seq = parseCRUD(SeqPost, line[6:], true)
	case strings.HasPrefix(line, "@put "):
		seq = parseCRUD(SeqPut, line[5:], false)
	case strings.HasPrefix(line, "@delete "):
		seq = parseCRUD(SeqDelete, line[8:], false)
	case strings.HasPrefix(line, "@empty "):
		seq = parseGuard(SeqEmpty, line[7:])
	case strings.HasPrefix(line, "@exists "):
		seq = parseGuard(SeqExists, line[8:])
	case strings.HasPrefix(line, "@state "):
		seq = parseState(line[7:])
	case strings.HasPrefix(line, "@auth "):
		seq = parseAuth(line[6:])
	case strings.HasPrefix(line, "@call "):
		seq = parseCall(line[6:])
	default:
		return nil, false
	}

	if seq != nil && suppressWarn {
		seq.SuppressWarn = true
	}
	return seq, false
}

// parseCRUD는 @get/@post/@put/@delete를 파싱한다.
// hasResult=true: Type var = Model.Method({Key: val, ...})
// hasResult=false: Model.Method({Key: val, ...})
func parseCRUD(seqType, rest string, hasResult bool) *Sequence {
	rest = strings.TrimSpace(rest)
	seq := &Sequence{Type: seqType}

	if hasResult {
		// Type var = Model.Method({Key: val, ...})
		eqIdx := strings.Index(rest, "=")
		if eqIdx < 0 {
			return nil
		}
		lhs := strings.TrimSpace(rest[:eqIdx])
		rhs := strings.TrimSpace(rest[eqIdx+1:])

		result := parseResult(lhs)
		if result == nil {
			return nil
		}
		seq.Result = result

		model, inputs := parseCallExprInputs(rhs)
		seq.Model = model
		seq.Inputs = inputs
	} else {
		// Model.Method({Key: val, ...})
		model, inputs := parseCallExprInputs(rest)
		seq.Model = model
		seq.Inputs = inputs
	}

	return seq
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
func parseState(rest string) *Sequence {
	rest = strings.TrimSpace(rest)

	// diagramID
	spaceIdx := strings.IndexByte(rest, ' ')
	if spaceIdx < 0 {
		return nil
	}
	diagramID := rest[:spaceIdx]
	rest = strings.TrimSpace(rest[spaceIdx+1:])

	// {inputs}
	inputs, rest := extractInputs(rest)

	// "transition" "message"
	transition, msg := parseTwoQuoted(rest)

	return &Sequence{
		Type:       SeqState,
		DiagramID:  diagramID,
		Inputs:     inputs,
		Transition: transition,
		Message:    msg,
	}
}

// parseAuth는 @auth를 파싱한다.
// "action" "resource" {inputs} "message"
func parseAuth(rest string) *Sequence {
	rest = strings.TrimSpace(rest)

	// "action"
	action, rest := extractQuoted(rest)
	rest = strings.TrimSpace(rest)

	// "resource"
	resource, rest := extractQuoted(rest)
	rest = strings.TrimSpace(rest)

	// {inputs}
	inputs, rest := extractInputs(rest)

	// "message"
	msg, _ := extractQuoted(strings.TrimSpace(rest))

	return &Sequence{
		Type:     SeqAuth,
		Action:   action,
		Resource: resource,
		Inputs:   inputs,
		Message:  msg,
	}
}

// parseCall은 @call을 파싱한다.
// Type var = pkg.Func({Key: val, ...}) 또는 pkg.Func({Key: val, ...})
func parseCall(rest string) *Sequence {
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
			return nil
		}
		seq.Result = result

		model, inputs := parseCallExprInputs(rhs)
		seq.Model = model
		seq.Inputs = inputs
	} else {
		model, inputs := parseCallExprInputs(rest)
		seq.Model = model
		seq.Inputs = inputs
	}

	return seq
}

// parseCallExprInputs는 "pkg.Func({Key: val, ...})"를 파싱한다.
func parseCallExprInputs(expr string) (string, map[string]string) {
	expr = strings.TrimSpace(expr)
	parenIdx := strings.Index(expr, "(")
	if parenIdx < 0 {
		return expr, nil
	}
	model := expr[:parenIdx]
	inner := expr[parenIdx+1:]
	inner = strings.TrimSuffix(strings.TrimSpace(inner), ")")
	inner = strings.TrimSpace(inner)
	if inner == "" {
		return model, nil
	}
	return model, parseInputs(inner)
}

// parseResult는 "Type var" 또는 "[]Type var"를 파싱한다.
func parseResult(lhs string) *Result {
	lhs = strings.TrimSpace(lhs)
	parts := strings.Fields(lhs)
	if len(parts) != 2 {
		return nil
	}
	return &Result{
		Type: parts[0],
		Var:  parts[1],
	}
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
func parseInputs(s string) map[string]string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "{")
	s = strings.TrimSuffix(s, "}")
	s = strings.TrimSpace(s)
	if s == "" {
		return map[string]string{}
	}
	result := make(map[string]string)
	for _, pair := range strings.Split(s, ",") {
		pair = strings.TrimSpace(pair)
		colonIdx := strings.IndexByte(pair, ':')
		if colonIdx < 0 {
			continue
		}
		key := strings.TrimSpace(pair[:colonIdx])
		val := strings.TrimSpace(pair[colonIdx+1:])
		if key != "" && val != "" {
			result[key] = val
		}
	}
	return result
}

// extractInputs는 문자열에서 {…} 블록을 추출하고 나머지를 반환한다.
func extractInputs(s string) (map[string]string, string) {
	openIdx := strings.IndexByte(s, '{')
	if openIdx < 0 {
		return map[string]string{}, s
	}
	closeIdx := strings.IndexByte(s, '}')
	if closeIdx < 0 {
		return map[string]string{}, s
	}
	inputStr := s[openIdx : closeIdx+1]
	rest := strings.TrimSpace(s[closeIdx+1:])
	return parseInputs(inputStr), rest
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
