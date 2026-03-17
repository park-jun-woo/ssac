package generator

import (
	"bytes"
	"fmt"
	"go/format"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/park-jun-woo/ssac/parser"
	"github.com/park-jun-woo/ssac/validator"
	"github.com/ettle/strcase"
)

// GoTarget은 Go 언어용 코드 생성기다.
type GoTarget struct{}

// FileExtension은 Go 파일 확장자를 반환한다.
func (g *GoTarget) FileExtension() string { return ".go" }

// GenerateFunc는 단일 ServiceFunc의 Go 코드를 생성한다.
func (g *GoTarget) GenerateFunc(sf parser.ServiceFunc, st *validator.SymbolTable) ([]byte, error) {
	if sf.Subscribe != nil {
		return g.generateSubscribeFunc(sf, st)
	}
	return g.generateHTTPFunc(sf, st)
}

// GenerateModelInterfaces는 심볼 테이블과 SSaC spec을 교차하여 Model interface를 생성한다.
func (g *GoTarget) GenerateModelInterfaces(funcs []parser.ServiceFunc, st *validator.SymbolTable, outDir string) error {
	modelDir := filepath.Join(outDir, "model")
	if err := os.MkdirAll(modelDir, 0755); err != nil {
		return fmt.Errorf("model 디렉토리 생성 실패: %w", err)
	}

	usages := collectModelUsages(funcs)
	interfaces := deriveInterfaces(usages, st)
	if len(interfaces) == 0 {
		return nil
	}

	code := renderInterfaces(interfaces, hasQueryOpts(st))
	formatted, err := format.Source(code)
	if err != nil {
		return fmt.Errorf("models_gen.go gofmt 실패: %w\n--- raw ---\n%s", err, string(code))
	}

	path := filepath.Join(modelDir, "models_gen.go")
	return os.WriteFile(path, formatted, 0644)
}

// GenerateHandlerStruct는 도메인별 Handler struct를 생성한다.
func (g *GoTarget) GenerateHandlerStruct(funcs []parser.ServiceFunc, st *validator.SymbolTable, outDir string) error {
	domainModels := collectDomainModels(funcs)
	for domain, models := range domainModels {
		if len(models) == 0 {
			continue
		}

		var buf bytes.Buffer
		pkgName := "service"
		if domain != "" {
			pkgName = domain
		}
		buf.WriteString("package " + pkgName + "\n\n")
		buf.WriteString("import (\n")
		buf.WriteString("\t\"database/sql\"\n\n")
		buf.WriteString("\t\"model\"\n")
		buf.WriteString(")\n\n")
		buf.WriteString("type Handler struct {\n")
		buf.WriteString("\tDB *sql.DB\n")
		for _, m := range models {
			pascalName := strcase.ToGoPascal(m)
			fmt.Fprintf(&buf, "\t%sModel model.%sModel\n", pascalName, pascalName)
		}
		buf.WriteString("}\n")

		formatted, err := format.Source(buf.Bytes())
		if err != nil {
			return fmt.Errorf("handler.go gofmt 실패: %w\n--- raw ---\n%s", err, buf.String())
		}

		outPath := outDir
		if domain != "" {
			outPath = filepath.Join(outDir, domain)
			os.MkdirAll(outPath, 0755)
		}
		path := filepath.Join(outPath, "handler.go")
		if err := os.WriteFile(path, formatted, 0644); err != nil {
			return fmt.Errorf("handler.go 파일 쓰기 실패: %w", err)
		}
	}
	return nil
}

// collectDomainModels는 도메인별로 사용되는 모델 이름을 수집한다.
func collectDomainModels(funcs []parser.ServiceFunc) map[string][]string {
	domainSet := map[string]map[string]bool{}
	for _, sf := range funcs {
		domain := sf.Domain
		if domainSet[domain] == nil {
			domainSet[domain] = map[string]bool{}
		}
		for _, seq := range sf.Sequences {
			if seq.Model == "" || seq.Type == parser.SeqCall {
				continue
			}
			parts := strings.SplitN(seq.Model, ".", 2)
			if len(parts) >= 1 {
				domainSet[domain][parts[0]] = true
			}
		}
	}
	result := map[string][]string{}
	for domain, models := range domainSet {
		keys := make([]string, 0, len(models))
		for k := range models {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		result[domain] = keys
	}
	return result
}
