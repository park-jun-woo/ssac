package generator

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/park-jun-woo/ssac/parser"
	"github.com/park-jun-woo/ssac/validator"
)

// Generate는 []ServiceFunc를 받아 outDir에 Go 파일을 생성한다.
func Generate(funcs []parser.ServiceFunc, outDir string, st *validator.SymbolTable) error {
	return GenerateWith(DefaultTarget(), funcs, outDir, st)
}

// GenerateFunc는 단일 ServiceFunc의 Go 코드를 생성한다.
func GenerateFunc(sf parser.ServiceFunc, st *validator.SymbolTable) ([]byte, error) {
	return DefaultTarget().GenerateFunc(sf, st)
}

// GenerateModelInterfaces는 심볼 테이블과 SSaC spec을 교차하여 Model interface를 생성한다.
func GenerateModelInterfaces(funcs []parser.ServiceFunc, st *validator.SymbolTable, outDir string) error {
	return DefaultTarget().GenerateModelInterfaces(funcs, st, outDir)
}

// GenerateHandlerStruct는 도메인별 Handler struct를 생성한다.
func GenerateHandlerStruct(funcs []parser.ServiceFunc, st *validator.SymbolTable, outDir string) error {
	return DefaultTarget().GenerateHandlerStruct(funcs, st, outDir)
}

// GenerateWith는 지정된 Target으로 코드를 생성한다.
func GenerateWith(t Target, funcs []parser.ServiceFunc, outDir string, st *validator.SymbolTable) error {
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return fmt.Errorf("출력 디렉토리 생성 실패: %w", err)
	}

	for _, sf := range funcs {
		code, err := t.GenerateFunc(sf, st)
		if err != nil {
			return fmt.Errorf("%s 코드 생성 실패: %w", sf.Name, err)
		}

		ext := t.FileExtension()
		outName := strings.TrimSuffix(sf.FileName, ".ssac") + ext
		outPath := outDir
		if sf.Domain != "" {
			outPath = filepath.Join(outDir, sf.Domain)
			os.MkdirAll(outPath, 0755)
		}
		path := filepath.Join(outPath, outName)
		if err := os.WriteFile(path, code, 0644); err != nil {
			return fmt.Errorf("%s 파일 쓰기 실패: %w", path, err)
		}
	}
	return nil
}

// commonInitialisms는 Go 컨벤션에서 대소문자를 통일하는 공통 이니셜리즘이다.
// https://github.com/golang/lint/blob/master/lint.go#L770
var commonInitialisms = map[string]bool{
	"ACL": true, "API": true, "ASCII": true, "CPU": true, "CSS": true,
	"DNS": true, "EOF": true, "HTML": true, "HTTP": true, "HTTPS": true,
	"ID": true, "IP": true, "JSON": true, "QPS": true, "RAM": true,
	"RPC": true, "SLA": true, "SMTP": true, "SQL": true, "SSH": true,
	"TCP": true, "TLS": true, "TTL": true, "UDP": true, "UI": true,
	"UID": true, "UUID": true, "URI": true, "URL": true, "XML": true,
}

// lcFirst는 Go 컨벤션에 맞게 첫 "단어"를 소문자로 변환한다.
// "ID" → "id", "CourseID" → "courseID", "HTTPClient" → "httpClient"
func lcFirst(s string) string {
	if s == "" {
		return s
	}
	// 선행 대문자 연속 개수
	upper := 0
	for i := 0; i < len(s); i++ {
		if s[i] >= 'A' && s[i] <= 'Z' {
			upper++
		} else {
			break
		}
	}
	if upper == 0 {
		return s
	}
	if upper == 1 {
		return strings.ToLower(s[:1]) + s[1:]
	}
	// 전부 대문자: "ID" → "id", "URL" → "url"
	if upper == len(s) {
		return strings.ToLower(s)
	}
	// 마지막 대문자는 다음 단어 시작: "IDParser" → "idParser", "HTTPClient" → "httpClient"
	return strings.ToLower(s[:upper-1]) + s[upper-1:]
}

// ucFirst는 Go 컨벤션에 맞게 첫 글자를 대문자로 변환한다.
// 이니셜리즘이면 전부 대문자: "id" → "ID", "url" → "URL"
func ucFirst(s string) string {
	if s == "" {
		return s
	}
	if commonInitialisms[strings.ToUpper(s)] {
		return strings.ToUpper(s)
	}
	return strings.ToUpper(s[:1]) + s[1:]
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
