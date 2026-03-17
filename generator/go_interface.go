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

// --- Model 인터페이스 파생 ---

type modelUsage struct {
	ModelName  string
	MethodName string
	Inputs     map[string]string
	Result     *parser.Result
	FuncName   string
}

func collectModelUsages(funcs []parser.ServiceFunc) []modelUsage {
	var usages []modelUsage
	for _, sf := range funcs {
		for _, seq := range sf.Sequences {
			if seq.Model == "" || seq.Type == parser.SeqCall || seq.Package != "" {
				continue // @call과 패키지 접두사 모델은 models_gen.go에 포함하지 않음
			}
			parts := strings.SplitN(seq.Model, ".", 2)
			if len(parts) < 2 {
				continue
			}
			usages = append(usages, modelUsage{
				ModelName:  parts[0],
				MethodName: parts[1],
				Inputs:     seq.Inputs,
				Result:     seq.Result,
				FuncName:   sf.Name,
			})
		}
	}
	return usages
}

type derivedInterface struct {
	Name    string
	Methods []derivedMethod
}

type derivedMethod struct {
	Name         string
	Params       []derivedParam
	HasQueryOpts bool
	ReturnType   string
}

type derivedParam struct {
	Name   string
	GoType string
}

func deriveInterfaces(usages []modelUsage, st *validator.SymbolTable) []derivedInterface {
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

	var interfaces []derivedInterface
	sortedModels := sortedKeys(modelNames)

	for _, modelName := range sortedModels {
		ms, ok := st.Models[modelName]
		if !ok {
			continue
		}

		iface := derivedInterface{Name: modelName + "Model"}

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

			var inputKeys []string
			if len(mi.Params) > 0 {
				// sqlc/interface 파라미터 순서대로
				used := make(map[string]bool)
				for _, p := range mi.Params {
					if _, ok := usage.Inputs[p]; ok {
						inputKeys = append(inputKeys, p)
						used[p] = true
					}
				}
				for k := range usage.Inputs {
					if !used[k] {
						inputKeys = append(inputKeys, k)
					}
				}
			} else {
				for k := range usage.Inputs {
					inputKeys = append(inputKeys, k)
				}
				sort.Strings(inputKeys)
			}

			for _, k := range inputKeys {
				val := usage.Inputs[k]
				if val == "query" {
					dm.HasQueryOpts = true
					continue
				}
				dp := derivedParam{
					Name:   strcase.ToGoCamel(k),
					GoType: resolveInputParamType(val, usage.ModelName, st),
				}
				if dp.Name != "" {
					dm.Params = append(dm.Params, dp)
				}
			}

			dm.ReturnType = deriveReturnType(mi, usage, dm.HasQueryOpts)
			iface.Methods = append(iface.Methods, dm)
		}

		if len(iface.Methods) > 0 {
			interfaces = append(interfaces, iface)
		}
	}

	return interfaces
}

func resolveArgParamName(a parser.Arg) string {
	if a.Literal != "" {
		return strcase.ToGoCamel(a.Literal) // 리터럴은 값 자체를 이름으로 사용
	}
	if a.Source == "request" || a.Source == "currentUser" {
		return strcase.ToGoCamel(a.Field)
	}
	if a.Source != "" {
		return a.Source + strcase.ToGoPascal(a.Field)
	}
	return strcase.ToGoCamel(a.Field)
}

func resolveArgParamType(a parser.Arg, modelName string, st *validator.SymbolTable) string {
	if a.Literal != "" {
		return "string"
	}

	// source.Field → source 테이블의 field 컬럼 조회
	if a.Source != "" && a.Source != "request" && a.Source != "currentUser" {
		refTable := toSnakeCase(a.Source) + "s"
		refCol := toSnakeCase(a.Field)
		if table, ok := st.DDLTables[refTable]; ok {
			if goType, ok := table.Columns[refCol]; ok {
				return goType
			}
		}
	}

	snakeName := toSnakeCase(a.Field)

	// 해당 모델 테이블
	tableName := toSnakeCase(modelName) + "s"
	if table, ok := st.DDLTables[tableName]; ok {
		if goType, ok := table.Columns[snakeName]; ok {
			return goType
		}
	}

	// {Model}ID 패턴
	if strings.HasSuffix(a.Field, "ID") {
		refModel := a.Field[:len(a.Field)-2]
		refTable := toSnakeCase(refModel) + "s"
		if table, ok := st.DDLTables[refTable]; ok {
			if goType, ok := table.Columns["id"]; ok {
				return goType
			}
		}
	}

	// 전체 순회
	for _, table := range st.DDLTables {
		if goType, ok := table.Columns[snakeName]; ok {
			return goType
		}
	}

	return "string"
}

// resolveInputParamType는 Inputs value에서 Go 타입을 추론한다.
// value 형식: "request.Field", "source.Field", "\"literal\"", "currentUser.Field"
func resolveInputParamType(val string, modelName string, st *validator.SymbolTable) string {
	// 리터럴
	if strings.HasPrefix(val, `"`) {
		return "string"
	}

	dotIdx := strings.IndexByte(val, '.')
	if dotIdx < 0 {
		return "string"
	}
	source := val[:dotIdx]
	field := val[dotIdx+1:]

	// source.Field → source 테이블의 field 컬럼 조회
	if source != "request" && source != "currentUser" {
		refTable := toSnakeCase(source) + "s"
		refCol := toSnakeCase(field)
		if table, ok := st.DDLTables[refTable]; ok {
			if goType, ok := table.Columns[refCol]; ok {
				return goType
			}
		}
	}

	snakeName := toSnakeCase(field)

	// 해당 모델 테이블
	tableName := toSnakeCase(modelName) + "s"
	if table, ok := st.DDLTables[tableName]; ok {
		if goType, ok := table.Columns[snakeName]; ok {
			return goType
		}
	}

	// {Model}ID 패턴
	if strings.HasSuffix(field, "ID") {
		refModel := field[:len(field)-2]
		refTable := toSnakeCase(refModel) + "s"
		if table, ok := st.DDLTables[refTable]; ok {
			if goType, ok := table.Columns["id"]; ok {
				return goType
			}
		}
	}

	// 전체 순회
	for _, table := range st.DDLTables {
		if goType, ok := table.Columns[snakeName]; ok {
			return goType
		}
	}

	return "string"
}

func deriveReturnType(mi validator.MethodInfo, usage modelUsage, hasQueryOpts bool) string {
	// Wrapper 타입 (Page[T], Cursor[T])
	if usage.Result != nil && usage.Result.Wrapper != "" {
		return fmt.Sprintf("(*pagination.%s[%s], error)", usage.Result.Wrapper, usage.Result.Type)
	}

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
	default:
		typeName := "interface{}"
		if usage.Result != nil {
			typeName = usage.Result.Type
		}
		return fmt.Sprintf("(*%s, error)", typeName)
	}
}

func renderInterfaces(interfaces []derivedInterface, needQueryOpts bool) []byte {
	var buf bytes.Buffer
	buf.WriteString("package model\n\n")

	needTime := needsTimeImport(interfaces)
	needPagination := needsPaginationImport(interfaces)
	buf.WriteString("import (\n")
	buf.WriteString("\t\"database/sql\"\n")
	if needTime {
		buf.WriteString("\t\"time\"\n")
	}
	if needPagination {
		buf.WriteString("\n\t\"github.com/park-jun-woo/fullend/pkg/pagination\"\n")
	}
	buf.WriteString(")\n\n")

	for _, iface := range interfaces {
		fmt.Fprintf(&buf, "type %s interface {\n", iface.Name)
		fmt.Fprintf(&buf, "\tWithTx(tx *sql.Tx) %s\n", iface.Name)
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
	Limit   int
	Offset  int
	Cursor  string
	SortCol string
	SortDir string
	Filters map[string]string
}
`)
	}

	return buf.Bytes()
}

func renderParams(params []derivedParam) string {
	var parts []string
	for _, p := range params {
		if p.Name == "" {
			continue
		}
		parts = append(parts, p.Name+" "+p.GoType)
	}
	return strings.Join(parts, ", ")
}

func hasQueryOpts(st *validator.SymbolTable) bool {
	for _, op := range st.Operations {
		if op.HasQueryOpts() {
			return true
		}
	}
	return false
}

func needsPaginationImport(interfaces []derivedInterface) bool {
	for _, iface := range interfaces {
		for _, m := range iface.Methods {
			if strings.Contains(m.ReturnType, "pagination.") {
				return true
			}
		}
	}
	return false
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
