package parser

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
)

// ParseDir은 디렉토리 내 모든 .go 파일을 파싱하여 []ServiceFunc를 반환한다.
func ParseDir(dir string) ([]ServiceFunc, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("디렉토리 읽기 실패: %w", err)
	}

	var funcs []ServiceFunc
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") {
			continue
		}
		sf, err := ParseFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			return nil, fmt.Errorf("%s 파싱 실패: %w", entry.Name(), err)
		}
		if sf != nil {
			funcs = append(funcs, *sf)
		}
	}
	return funcs, nil
}

// ParseFile은 단일 .go 파일을 파싱하여 ServiceFunc를 반환한다.
// sequence 주석이 없으면 nil을 반환한다.
func ParseFile(path string) (*ServiceFunc, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("Go 파싱 실패: %w", err)
	}

	for _, decl := range f.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}

		// 함수 앞의 모든 주석 그룹을 수집 (빈 줄로 분리되어도 포함)
		var comments []*ast.Comment
		fnPos := fn.Pos()
		for _, cg := range f.Comments {
			if cg.End() < fnPos {
				comments = append(comments, cg.List...)
			}
		}

		sequences := parseCommentList(comments)
		if len(sequences) == 0 {
			continue
		}

		return &ServiceFunc{
			Name:      fn.Name.Name,
			FileName:  filepath.Base(path),
			Sequences: sequences,
		}, nil
	}
	return nil, nil
}

// parseCommentList는 주석 리스트에서 sequence 블록을 추출한다.
func parseCommentList(comments []*ast.Comment) []Sequence {
	var sequences []Sequence
	var current *Sequence

	for _, comment := range comments {
		line := strings.TrimPrefix(comment.Text, "//")
		line = strings.TrimSpace(line)

		if line == "" {
			continue
		}

		if !strings.HasPrefix(line, "@") {
			continue
		}

		tag, value := parseTag(line)

		if tag == "sequence" {
			if current != nil {
				sequences = append(sequences, *current)
			}
			seqType := parseSequenceType(value)
			current = &Sequence{Type: seqType}
			// guard nil/exists: 3번째 단어가 대상 변수
			if (seqType == SeqGuardNil || seqType == SeqGuardExists) {
				current.Target = parseGuardTarget(value)
			}
			continue
		}

		if current == nil {
			continue
		}

		switch tag {
		case "model":
			current.Model = value
		case "param":
			current.Params = append(current.Params, parseParam(value))
		case "result":
			current.Result = parseResult(value)
		case "message":
			current.Message = trimQuotes(value)
		case "var":
			current.Vars = append(current.Vars, value)
		case "action":
			current.Action = value
		case "resource":
			current.Resource = value
		case "id":
			current.ID = value
		case "component":
			current.Component = value
		case "func":
			current.Func = value
		}
	}

	if current != nil {
		sequences = append(sequences, *current)
	}

	return sequences
}

// parseTag는 "@tag value" 형식의 라인에서 태그와 값을 분리한다.
func parseTag(line string) (tag, value string) {
	line = strings.TrimPrefix(line, "@")
	parts := strings.SplitN(line, " ", 2)
	tag = parts[0]
	if len(parts) > 1 {
		value = strings.TrimSpace(parts[1])
	}
	return
}

// parseSequenceType은 "guard nil project" 같은 값에서 sequence 타입을 추출한다.
// "guard nil"과 "guard exists"는 두 단어 타입이며, 뒤의 대상은 Type에 포함하지 않는다.
// "response json"처럼 서브타입이 있는 경우도 함께 반환한다.
func parseSequenceType(value string) string {
	parts := strings.Fields(value)
	if len(parts) == 0 {
		return value
	}

	// guard nil / guard exists: 두 단어 타입
	if parts[0] == "guard" && len(parts) >= 2 {
		candidate := parts[0] + " " + parts[1]
		if ValidSequenceTypes[candidate] {
			return candidate
		}
	}

	// response json 등: 서브타입 포함
	if parts[0] == "response" && len(parts) >= 2 {
		return parts[0] + " " + parts[1]
	}

	return parts[0]
}

// parseParam은 "@param Name source" 형식을 파싱한다.
// 따옴표로 감싼 리터럴은 하나의 Name으로 취급한다.
func parseParam(value string) Param {
	// 따옴표로 시작하면 닫는 따옴표까지가 Name
	if strings.HasPrefix(value, `"`) {
		end := strings.Index(value[1:], `"`)
		if end >= 0 {
			return Param{Name: value[:end+2]} // 따옴표 포함
		}
	}

	parts := strings.Fields(value)
	p := Param{Name: parts[0]}
	if len(parts) > 1 {
		p.Source = parts[1]
	}
	return p
}

// parseResult는 "@result var Type" 형식을 파싱한다.
func parseResult(value string) *Result {
	parts := strings.Fields(value)
	if len(parts) < 2 {
		return &Result{Var: parts[0]}
	}
	return &Result{Var: parts[0], Type: parts[1]}
}

// parseGuardTarget은 "guard nil project"에서 대상 변수 "project"를 추출한다.
func parseGuardTarget(value string) string {
	parts := strings.Fields(value)
	if len(parts) >= 3 {
		return parts[2]
	}
	return ""
}

// trimQuotes는 양쪽 따옴표를 제거한다.
func trimQuotes(s string) string {
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		return s[1 : len(s)-1]
	}
	return s
}
