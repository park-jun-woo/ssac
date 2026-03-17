package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/park-jun-woo/ssac/generator"
	"github.com/park-jun-woo/ssac/parser"
	"github.com/park-jun-woo/ssac/validator"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: ssac <command>")
		fmt.Fprintln(os.Stderr, "commands: parse, validate, gen")
		os.Exit(1)
	}

	switch os.Args[1] {
	case "parse":
		runParse()
	case "validate":
		runValidate()
	case "gen":
		runGen()
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", os.Args[1])
		os.Exit(1)
	}
}

// printValidationResults는 검증 결과를 출력하고, 에러 유무를 반환한다.
// WARNING은 출력하되 에러로 간주하지 않는다.
func printValidationResults(errs []validator.ValidationError) bool {
	hasError := false
	for _, e := range errs {
		if e.IsWarning() {
			fmt.Fprintf(os.Stderr, "%s\n", e)
		} else {
			fmt.Fprintf(os.Stderr, "%s\n", e)
			hasError = true
		}
	}
	return hasError
}

func runValidate() {
	dir := "specs/backend/service"
	if len(os.Args) > 2 {
		dir = os.Args[2]
	}

	// dir/service/ 가 있으면 프로젝트 루트, 없으면 service 디렉토리 직접 지정
	serviceDir := filepath.Join(dir, "service")
	projectRoot := dir
	if _, err := os.Stat(serviceDir); os.IsNotExist(err) {
		serviceDir = dir
		projectRoot = ""
	}

	funcs, err := parser.ParseDir(serviceDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "parse error: %v\n", err)
		os.Exit(1)
	}

	// 프로젝트 루트가 있고 외부 SSOT가 있으면 심볼 테이블 교차 검증
	if projectRoot != "" {
		st, stErr := validator.LoadSymbolTable(projectRoot)
		if stErr == nil {
			errs := validator.ValidateWithSymbols(funcs, st)
			if hasError := printValidationResults(errs); hasError {
				os.Exit(1)
			}
			fmt.Println("validation passed (with symbol table)")
			return
		}
	}

	// 외부 SSOT 없으면 내부 검증만
	errs := validator.Validate(funcs)
	if hasError := printValidationResults(errs); hasError {
		os.Exit(1)
	}
	fmt.Println("validation passed")
}

func runGen() {
	if len(os.Args) < 4 {
		fmt.Fprintln(os.Stderr, "usage: ssac gen <service-dir> <out-dir>")
		os.Exit(1)
	}
	inDir := os.Args[2]
	outDir := os.Args[3]

	// inDir/service/ 가 있으면 프로젝트 루트로 간주
	serviceDir := filepath.Join(inDir, "service")
	projectRoot := inDir
	if _, err := os.Stat(serviceDir); os.IsNotExist(err) {
		serviceDir = inDir
		projectRoot = ""
	}

	funcs, err := parser.ParseDir(serviceDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "parse error: %v\n", err)
		os.Exit(1)
	}

	// validate before generate (외부 SSOT 있으면 교차 검증)
	var st *validator.SymbolTable
	var validationErrs []validator.ValidationError
	if projectRoot != "" {
		loaded, stErr := validator.LoadSymbolTable(projectRoot)
		if stErr == nil {
			st = loaded
			validationErrs = validator.ValidateWithSymbols(funcs, st)
		}
	}
	if validationErrs == nil {
		validationErrs = validator.Validate(funcs)
	}
	if hasError := printValidationResults(validationErrs); hasError {
		fmt.Fprintln(os.Stderr, "validation failed, code generation aborted")
		os.Exit(1)
	}

	if err := generator.Generate(funcs, outDir, st); err != nil {
		fmt.Fprintf(os.Stderr, "generate error: %v\n", err)
		os.Exit(1)
	}

	// Model interface 파생 생성 (심볼 테이블이 있을 때만)
	if st != nil {
		if err := generator.GenerateModelInterfaces(funcs, st, outDir); err != nil {
			fmt.Fprintf(os.Stderr, "model interface generate error: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("generated model interfaces in %s/model\n", outDir)
	}

	fmt.Printf("generated %d files in %s\n", len(funcs), outDir)
}

func runParse() {
	dir := "specs/backend/service"
	if len(os.Args) > 2 {
		dir = os.Args[2]
	}

	funcs, err := parser.ParseDir(dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	for _, f := range funcs {
		fmt.Printf("=== %s (%s) ===\n", f.Name, f.FileName)
		for i, s := range f.Sequences {
			fmt.Printf("  [%d] %s", i, s.Type)
			if s.Model != "" {
				fmt.Printf(" | model=%s", s.Model)
			}
			if s.Result != nil {
				fmt.Printf(" | result=%s %s", s.Result.Var, s.Result.Type)
			}
			if s.Message != "" {
				fmt.Printf(" | message=%q", s.Message)
			}
			fmt.Println()
		}
	}
}
