package generator

import (
	"bytes"
	"fmt"
	"go/format"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/geul-org/ssac/parser"
	"github.com/geul-org/ssac/validator"
)

// GenerateModelInterfaces는 심볼 테이블과 SSaC spec을 교차하여 Model interface를 생성한다.
func GenerateModelInterfaces(funcs []parser.ServiceFunc, st *validator.SymbolTable, outDir string) error {
	modelDir := filepath.Join(outDir, "model")
	if err := os.MkdirAll(modelDir, 0755); err != nil {
		return fmt.Errorf("model 디렉토리 생성 실패: %w", err)
	}

	// SSaC spec에서 사용된 모델+메서드 수집
	usages := collectModelUsages(funcs)

	// 사용된 모델만 interface 생성
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

// modelUsage는 SSaC spec에서 사용된 모델 메서드 호출 정보다.
type modelUsage struct {
	ModelName  string
	MethodName string
	Params     []parser.Param
	Result     *parser.Result
	FuncName   string // 어떤 서비스 함수에서 사용됐는지
}

// collectModelUsages는 모든 서비스 함수에서 @model 사용을 수집한다.
func collectModelUsages(funcs []parser.ServiceFunc) []modelUsage {
	var usages []modelUsage
	for _, sf := range funcs {
		for _, seq := range sf.Sequences {
			if seq.Model == "" {
				continue
			}
			parts := strings.SplitN(seq.Model, ".", 2)
			if len(parts) < 2 {
				continue
			}
			usages = append(usages, modelUsage{
				ModelName:  parts[0],
				MethodName: parts[1],
				Params:     seq.Params,
				Result:     seq.Result,
				FuncName:   sf.Name,
			})
		}
	}
	return usages
}

// derivedInterface는 파생된 interface 정의다.
type derivedInterface struct {
	Name    string
	Methods []derivedMethod
}

// derivedMethod는 파생된 메서드 시그니처다.
type derivedMethod struct {
	Name        string
	Params      []derivedParam
	HasQueryOpts bool
	ReturnType  string // "*User, error", "[]Reservation, int, error", "error"
}

// derivedParam은 메서드 파라미터다.
type derivedParam struct {
	Name   string
	GoType string
}

// deriveInterfaces는 심볼 테이블과 SSaC 사용 패턴을 교차하여 interface를 도출한다.
func deriveInterfaces(usages []modelUsage, st *validator.SymbolTable) []derivedInterface {
	// 모델별로 메서드 그룹화
	type methodKey struct {
		model, method string
	}
	methodMap := map[methodKey]modelUsage{}
	modelNames := map[string]bool{}

	for _, u := range usages {
		key := methodKey{u.ModelName, u.MethodName}
		if _, exists := methodMap[key]; !exists {
			methodMap[key] = u
			modelNames[u.ModelName] = true
		}
	}

	// SSaC에서 사용된 메서드만 interface에 포함
	var interfaces []derivedInterface
	sortedModels := sortedKeys(modelNames)

	for _, modelName := range sortedModels {
		ms, ok := st.Models[modelName]
		if !ok {
			continue
		}

		iface := derivedInterface{Name: modelName + "Model"}

		// SSaC에서 사용된 메서드만 순회
		var usedMethods []string
		for _, u := range usages {
			if u.ModelName == modelName {
				found := false
				for _, m := range usedMethods {
					if m == u.MethodName {
						found = true
						break
					}
				}
				if !found {
					usedMethods = append(usedMethods, u.MethodName)
				}
			}
		}
		sort.Strings(usedMethods)

		for _, methodName := range usedMethods {
			mi, methodExists := ms.Methods[methodName]
			if !methodExists {
				mi = validator.MethodInfo{}
			}
			key := methodKey{modelName, methodName}
			usage := methodMap[key]

			dm := derivedMethod{Name: methodName}

			// 파라미터 결정
			for _, p := range usage.Params {
				dp := derivedParam{
					Name:   resolveParamName(p),
					GoType: resolveParamType(p, st),
				}
				dm.Params = append(dm.Params, dp)
			}

			// QueryOpts: 이 메서드를 사용하는 함수의 operation에 x- 확장이 있는지 확인
			if op, ok := st.Operations[usage.FuncName]; ok && op.HasQueryOpts() {
				dm.HasQueryOpts = true
			}

			// 반환 타입 결정 (카디널리티 기반)
			dm.ReturnType = deriveReturnType(mi, usage, dm.HasQueryOpts)

			iface.Methods = append(iface.Methods, dm)
		}

		if len(iface.Methods) > 0 {
			interfaces = append(interfaces, iface)
		}
	}

	return interfaces
}

// resolveParamName은 Param에서 Go 파라미터명을 도출한다.
func resolveParamName(p parser.Param) string {
	name := p.Name
	// 리터럴은 시그니처에서 제외
	if strings.HasPrefix(name, `"`) {
		return ""
	}
	// dot notation (e.g. user.ID) → 시그니처에서 제외
	if strings.Contains(name, ".") {
		return ""
	}
	return lcFirst(name)
}

// resolveParamType은 DDL 컬럼 타입에서 Go 타입을 결정한다.
func resolveParamType(p parser.Param, st *validator.SymbolTable) string {
	// 리터럴 → string
	if strings.HasPrefix(p.Name, `"`) {
		return "string"
	}

	// DDL에서 타입 추론: PascalCase → snake_case
	snakeName := toSnakeCase(p.Name)
	for _, table := range st.DDLTables {
		if goType, ok := table.Columns[snakeName]; ok {
			return goType
		}
	}

	return "string"
}

// deriveReturnType은 카디널리티에서 반환 타입을 결정한다.
func deriveReturnType(mi validator.MethodInfo, usage modelUsage, hasQueryOpts bool) string {
	switch mi.Cardinality {
	case "exec":
		return "error"
	case "many":
		typeName := "interface{}"
		if usage.Result != nil {
			typeName = usage.Result.Type
			if strings.HasPrefix(typeName, "[]") {
				typeName = typeName[2:]
			}
		}
		if hasQueryOpts {
			return fmt.Sprintf("([]%s, int, error)", typeName)
		}
		return fmt.Sprintf("([]%s, error)", typeName)
	default: // "one" 또는 빈 문자열
		typeName := "interface{}"
		if usage.Result != nil {
			typeName = usage.Result.Type
		}
		return fmt.Sprintf("(*%s, error)", typeName)
	}
}

// renderInterfaces는 interface 정의를 Go 코드로 렌더링한다.
func renderInterfaces(interfaces []derivedInterface, needQueryOpts bool) []byte {
	var buf bytes.Buffer
	buf.WriteString("package model\n\n")

	// time.Time이 사용되면 import 추가
	if needsTimeImport(interfaces) {
		buf.WriteString("import \"time\"\n\n")
	}

	for _, iface := range interfaces {
		fmt.Fprintf(&buf, "type %s interface {\n", iface.Name)
		for _, m := range iface.Methods {
			params := renderParams(m.Params)
			if m.HasQueryOpts {
				if params != "" {
					params += ", "
				}
				params += "opts QueryOpts"
			}
			fmt.Fprintf(&buf, "\t%s(%s) %s\n", m.Name, params, m.ReturnType)
		}
		buf.WriteString("}\n\n")
	}

	if needQueryOpts {
		buf.WriteString(`type QueryOpts struct {
	Limit    int
	Offset   int
	Cursor   string
	SortCol  string
	SortDir  string
	Filters  map[string]string
	Includes []string
}
`)
	}

	return buf.Bytes()
}

// renderParams는 파라미터 목록을 Go 시그니처 문자열로 렌더링한다.
func renderParams(params []derivedParam) string {
	var parts []string
	for _, p := range params {
		if p.Name == "" {
			// 리터럴 파라미터 → 시그니처에서 제외
			continue
		}
		parts = append(parts, p.Name+" "+p.GoType)
	}
	return strings.Join(parts, ", ")
}

// hasQueryOpts는 어떤 operation이라도 x- 확장을 가지는지 확인한다.
func hasQueryOpts(st *validator.SymbolTable) bool {
	for _, op := range st.Operations {
		if op.HasQueryOpts() {
			return true
		}
	}
	return false
}

// toSnakeCase는 PascalCase/camelCase를 snake_case로 변환한다.
func toSnakeCase(s string) string {
	var result []byte
	for i, c := range s {
		if c >= 'A' && c <= 'Z' {
			if i > 0 {
				result = append(result, '_')
			}
			result = append(result, byte(c)+32)
		} else {
			result = append(result, byte(c))
		}
	}
	return string(result)
}

func needsTimeImport(interfaces []derivedInterface) bool {
	for _, iface := range interfaces {
		for _, m := range iface.Methods {
			for _, p := range m.Params {
				if p.GoType == "time.Time" {
					return true
				}
			}
		}
	}
	return false
}

func sortedKeys(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func sortedMethodKeys(m map[string]validator.MethodInfo) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
