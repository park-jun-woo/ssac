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
func printValidationResults(errs []validator.ValidationError) bool {
	hasError := false
	for _, e := range errs {
		fmt.Fprintf(os.Stderr, "%s\n", e)
		if !e.IsWarning() {
			hasError = true
		}
	}
	return hasError
}

// resolveServiceDir는 프로젝트 루트와 서비스 디렉토리를 결정한다.
func resolveServiceDir(dir string) (serviceDir, projectRoot string) {
	serviceDir = filepath.Join(dir, "service")
	projectRoot = dir
	if _, err := os.Stat(serviceDir); os.IsNotExist(err) {
		serviceDir = dir
		projectRoot = ""
	}
	return
}

func runParse() {
	dir := "specs/backend/service"
	if len(os.Args) > 2 {
		dir = os.Args[2]
	}

	serviceDir, _ := resolveServiceDir(dir)

	funcs, err := parser.ParseDir(serviceDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	for _, f := range funcs {
		if f.Domain != "" {
			fmt.Printf("=== %s (%s/%s) ===\n", f.Name, f.Domain, f.FileName)
		} else {
			fmt.Printf("=== %s (%s) ===\n", f.Name, f.FileName)
		}
		for i, s := range f.Sequences {
			fmt.Printf("  [%d] @%s", i, s.Type)
			if s.Model != "" {
				fmt.Printf(" %s", s.Model)
			}
			if s.Result != nil {
				fmt.Printf(" → %s %s", s.Result.Var, s.Result.Type)
			}
			if len(s.Args) > 0 {
				fmt.Printf(" args=%d", len(s.Args))
			}
			if s.Target != "" {
				fmt.Printf(" target=%s", s.Target)
			}
			if s.Action != "" {
				fmt.Printf(" action=%s resource=%s", s.Action, s.Resource)
			}
			if s.DiagramID != "" {
				fmt.Printf(" diagram=%s transition=%s", s.DiagramID, s.Transition)
			}
			if len(s.Fields) > 0 {
				fmt.Printf(" fields=%d", len(s.Fields))
			}
			if s.Message != "" {
				fmt.Printf(" msg=%q", s.Message)
			}
			fmt.Println()
		}
	}
}

func runValidate() {
	dir := "specs/backend/service"
	if len(os.Args) > 2 {
		dir = os.Args[2]
	}

	serviceDir, projectRoot := resolveServiceDir(dir)

	funcs, err := parser.ParseDir(serviceDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "parse error: %v\n", err)
		os.Exit(1)
	}

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

	serviceDir, projectRoot := resolveServiceDir(inDir)

	funcs, err := parser.ParseDir(serviceDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "parse error: %v\n", err)
		os.Exit(1)
	}

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

	if st != nil {
		if err := generator.GenerateModelInterfaces(funcs, st, outDir); err != nil {
			fmt.Fprintf(os.Stderr, "model interface generate error: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("generated model interfaces in %s/model\n", outDir)
	}

	fmt.Printf("generated %d files in %s\n", len(funcs), outDir)
}
