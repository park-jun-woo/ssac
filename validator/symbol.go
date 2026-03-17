package validator

import (
	"bufio"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/ettle/strcase"
	ssacparser "github.com/park-jun-woo/ssac/parser"
	"gopkg.in/yaml.v3"
)

// SymbolTable은 외부 SSOT에서 수집한 심볼 정보다.
type SymbolTable struct {
	Models     map[string]ModelSymbol     // "User" → {Methods: {"FindByID": ...}}
	Operations map[string]OperationSymbol // "Login" → {RequestFields, PathParams, ...}
	Funcs      map[string]bool            // "calculateRefund" → true
	DDLTables  map[string]DDLTable        // "users" → {Columns: {"id": "int64", ...}}
	DTOs map[string]bool // "Token" → true (DDL 테이블 없는 순수 DTO)
}

// ModelSymbol은 모델의 메서드 목록이다.
type ModelSymbol struct {
	Methods map[string]MethodInfo
}

// HasMethod는 메서드 존재 여부를 반환한다.
func (ms ModelSymbol) HasMethod(name string) bool {
	_, ok := ms.Methods[name]
	return ok
}

// MethodInfo는 모델 메서드의 상세 정보다.
type MethodInfo struct {
	Cardinality string            // "one", "many", "exec"
	Params      []string          // interface 파라미터명 (context.Context 제외, 패키지 모델용)
	ParamTypes  map[string]string // 파라미터명 → Go 타입 (e.g. "amount" → "int"). @call Request struct 필드용
	ErrStatus   int               // @error 어노테이션 값 (0이면 미지정)
}

// DDLTable은 DDL에서 파싱한 테이블 컬럼 정보다.
type DDLTable struct {
	Columns     map[string]string // snake_case 컬럼명 → Go 타입
	ColumnOrder []string          // DDL 정의 순서 보존
	ForeignKeys []ForeignKey      // FK 관계 목록
	Indexes     []Index           // 인덱스 목록
	PrimaryKey  []string          // PK 컬럼명 목록 (e.g. ["id"])
}

// ForeignKey는 외래 키 관계다.
type ForeignKey struct {
	Column    string // 이 테이블의 컬럼 (e.g. "user_id")
	RefTable  string // 참조 테이블 (e.g. "users")
	RefColumn string // 참조 컬럼 (e.g. "id")
}

// Index는 테이블 인덱스다.
type Index struct {
	Name     string   // 인덱스 이름 (e.g. "idx_reservations_room_time")
	Columns  []string // 인덱스 컬럼 목록
	IsUnique bool     // UNIQUE INDEX 또는 UNIQUE 제약
}

// OperationSymbol은 API 엔드포인트의 request/response 필드 목록이다.
type OperationSymbol struct {
	RequestFields map[string]bool
	PathParams    []PathParam // path parameter (순서 보존)
	XPagination    *XPagination
	XSort          *XSort
	XFilter        *XFilter
	XInclude       *XInclude
}

// PathParam은 OpenAPI path parameter다.
type PathParam struct {
	Name   string // 원본 이름 (e.g. "CourseID")
	GoType string // Go 타입 (e.g. "int64")
}

// HasQueryOpts는 x- 확장이 하나라도 있는지 반환한다.
func (op OperationSymbol) HasQueryOpts() bool {
	return op.XPagination != nil || op.XSort != nil || op.XFilter != nil || op.XInclude != nil
}

// XPagination은 x-pagination 확장이다.
type XPagination struct {
	Style        string `yaml:"style"`
	DefaultLimit int    `yaml:"defaultLimit"`
	MaxLimit     int    `yaml:"maxLimit"`
}

// XSort는 x-sort 확장이다.
type XSort struct {
	Allowed   []string `yaml:"allowed"`
	Default   string   `yaml:"default"`
	Direction string   `yaml:"direction"`
}

// XFilter는 x-filter 확장이다.
type XFilter struct {
	Allowed []string `yaml:"allowed"`
}

// XInclude는 x-include 확장이다.
type XInclude struct {
	Allowed []string `yaml:"allowed"`
}

// LoadSymbolTable은 프로젝트 디렉토리에서 심볼 테이블을 구성한다.
// 디렉토리 구조:
//
//	<root>/db/queries/*.sql  — sqlc 쿼리 (모델+메서드)
//	<root>/api/openapi.yaml  — OpenAPI spec (request/response)
//	<root>/model/*.go        — Go interface, func
func LoadSymbolTable(root string) (*SymbolTable, error) {
	st := &SymbolTable{
		Models:     make(map[string]ModelSymbol),
		Operations: make(map[string]OperationSymbol),
		Funcs:      make(map[string]bool),
		DDLTables:  make(map[string]DDLTable),
		DTOs:       make(map[string]bool),
	}

	if err := st.loadDDL(filepath.Join(root, "db")); err != nil {
		return nil, fmt.Errorf("DDL 로드 실패: %w", err)
	}
	if err := st.loadSqlcQueries(filepath.Join(root, "db", "queries")); err != nil {
		return nil, fmt.Errorf("sqlc 쿼리 로드 실패: %w", err)
	}
	if err := st.loadOpenAPI(filepath.Join(root, "api", "openapi.yaml")); err != nil {
		return nil, fmt.Errorf("OpenAPI 로드 실패: %w", err)
	}
	if err := st.loadGoInterfaces(filepath.Join(root, "model")); err != nil {
		return nil, fmt.Errorf("Go interface 로드 실패: %w", err)
	}

	return st, nil
}

// loadSqlcQueries는 queries/*.sql에서 모델과 메서드를 추출한다.
// 파일명: users.sql → 모델 "User" (단수화 + PascalCase)
// 주석: -- name: FindByID :one → 메서드 "FindByID"
func (st *SymbolTable) loadSqlcQueries(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}

		modelName := sqlFileToModel(entry.Name())
		ms := ModelSymbol{Methods: make(map[string]MethodInfo)}

		f, err := os.Open(filepath.Join(dir, entry.Name()))
		if err != nil {
			return err
		}

		scanner := bufio.NewScanner(f)
		var currentMethod string
		var currentCardinality string
		var currentSQL strings.Builder

		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			// -- name: FindByID :one  또는  -- name: CourseFindByID :one
			if strings.HasPrefix(line, "-- name:") {
				// 이전 메서드의 SQL 처리
				if currentMethod != "" {
					params := extractSqlcParams(currentSQL.String())
					ms.Methods[currentMethod] = MethodInfo{
						Cardinality: currentCardinality,
						Params:      params,
					}
				}
				// 새 메서드 시작
				parts := strings.Fields(line)
				if len(parts) >= 4 {
					currentMethod = stripModelPrefix(parts[2], modelName)
					currentCardinality = strings.TrimPrefix(parts[3], ":")
				} else if len(parts) >= 3 {
					currentMethod = stripModelPrefix(parts[2], modelName)
					currentCardinality = ""
				} else {
					currentMethod = ""
					currentCardinality = ""
				}
				currentSQL.Reset()
			} else if currentMethod != "" {
				currentSQL.WriteString(line + " ")
			}
		}
		// 마지막 메서드 처리
		if currentMethod != "" {
			params := extractSqlcParams(currentSQL.String())
			ms.Methods[currentMethod] = MethodInfo{
				Cardinality: currentCardinality,
				Params:      params,
			}
		}
		f.Close()

		if len(ms.Methods) > 0 {
			st.Models[modelName] = ms
		}
	}
	return nil
}

// extractSqlcParams는 SQL 본문에서 $N ↔ 컬럼명 매핑을 추출하여 $1, $2, ... 순서의 PascalCase 파라미터명을 반환한다.
func extractSqlcParams(sql string) []string {
	if !strings.Contains(sql, "$") {
		return nil
	}

	upper := strings.ToUpper(sql)
	if strings.Contains(upper, "INSERT") {
		if params := extractInsertParams(sql); len(params) > 0 {
			return params
		}
	}
	return extractWhereSetParams(sql)
}

// extractInsertParams는 INSERT INTO table (col1, col2) VALUES ($1, $2) 패턴에서 컬럼 순서를 추출한다.
func extractInsertParams(sql string) []string {
	// 첫 번째 괄호 쌍 = 컬럼 목록
	parenStart := strings.IndexByte(sql, '(')
	if parenStart < 0 {
		return nil
	}
	parenEnd := strings.IndexByte(sql[parenStart:], ')')
	if parenEnd < 0 {
		return nil
	}
	colStr := sql[parenStart+1 : parenStart+parenEnd]
	cols := strings.Split(colStr, ",")

	var params []string
	for _, col := range cols {
		col = strings.TrimSpace(col)
		if col != "" {
			params = append(params, strcase.ToGoPascal(col))
		}
	}
	return params
}

var sqlParamRe = regexp.MustCompile(`(\w+)\s*[=<>!]+\s*\$(\d+)`)

// extractWhereSetParams는 WHERE/SET 절에서 col = $N, col > $N 패턴을 추출하여 $N 순서대로 반환한다.
func extractWhereSetParams(sql string) []string {
	matches := sqlParamRe.FindAllStringSubmatch(sql, -1)
	if len(matches) == 0 {
		return nil
	}

	type paramEntry struct {
		pos  int
		name string
	}
	var entries []paramEntry
	for _, m := range matches {
		pos, err := strconv.Atoi(m[2])
		if err != nil {
			continue
		}
		entries = append(entries, paramEntry{pos: pos, name: m[1]})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].pos < entries[j].pos })

	var params []string
	for _, e := range entries {
		params = append(params, strcase.ToGoPascal(e.name))
	}
	return params
}

// stripModelPrefix는 쿼리 이름에서 모델명 접두사를 제거한다.
// "CourseFindByID" + "Course" → "FindByID", "FindByID" + "Course" → "FindByID"
func stripModelPrefix(queryName, modelName string) string {
	if strings.HasPrefix(queryName, modelName) {
		stripped := queryName[len(modelName):]
		if len(stripped) > 0 && stripped[0] >= 'A' && stripped[0] <= 'Z' {
			return stripped
		}
	}
	return queryName
}

// sqlFileToModel은 "reservations.sql" → "Reservation" 변환한다.
func sqlFileToModel(filename string) string {
	name := strings.TrimSuffix(filename, ".sql")
	// 단수화: 간단한 규칙 (es → 제거, s → 제거)
	if strings.HasSuffix(name, "ies") {
		name = name[:len(name)-3] + "y"
	} else if strings.HasSuffix(name, "sses") || strings.HasSuffix(name, "xes") {
		name = name[:len(name)-2]
	} else if strings.HasSuffix(name, "s") {
		name = name[:len(name)-1]
	}
	// PascalCase
	return strings.ToUpper(name[:1]) + name[1:]
}

// loadOpenAPI는 openapi.yaml에서 operationId별 request/response 필드를 추출한다.
func (st *SymbolTable) loadOpenAPI(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	var spec openAPISpec
	if err := yaml.Unmarshal(data, &spec); err != nil {
		return fmt.Errorf("YAML 파싱 실패: %w", err)
	}

	schemas := spec.Components.Schemas

	for _, pathItem := range spec.Paths {
		for _, op := range pathItem.operations() {
			if op.OperationID == "" {
				continue
			}

			opSym := OperationSymbol{
				RequestFields: make(map[string]bool),
				XPagination:   op.XPagination,
				XSort:          op.XSort,
				XFilter:        op.XFilter,
				XInclude:       op.XInclude,
			}

			// path/query parameters
			for _, param := range op.Parameters {
				opSym.RequestFields[param.Name] = true
				if param.In == "path" {
					opSym.PathParams = append(opSym.PathParams, PathParam{
						Name:   param.Name,
						GoType: oaTypeToGo(param.Schema.Type, param.Schema.Format),
					})
				}
			}

			// request body fields
			if op.RequestBody != nil {
				if content, ok := op.RequestBody.Content["application/json"]; ok {
					fields := collectSchemaFields(content.Schema, schemas)
					for _, f := range fields {
						opSym.RequestFields[f] = true
					}
				}
			}

			st.Operations[op.OperationID] = opSym
		}
	}

	return nil
}

// loadGoInterfaces는 model/*.go에서 interface(component)와 func을 추출한다.
func (st *SymbolTable) loadGoInterfaces(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	fset := token.NewFileSet()
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") {
			continue
		}

		f, err := parser.ParseFile(fset, filepath.Join(dir, entry.Name()), nil, parser.ParseComments)
		if err != nil {
			return fmt.Errorf("%s 파싱 실패: %w", entry.Name(), err)
		}

		for _, decl := range f.Decls {
			gd, ok := decl.(*ast.GenDecl)
			if !ok {
				continue
			}

			// GenDecl 또는 TypeSpec의 Doc에서 @dto 감지
			hasDtoTag := false
			if gd.Doc != nil {
				for _, c := range gd.Doc.List {
					if strings.Contains(c.Text, "@dto") {
						hasDtoTag = true
						break
					}
				}
			}

			for _, spec := range gd.Specs {
				ts, ok := spec.(*ast.TypeSpec)
				if !ok {
					continue
				}

				// TypeSpec 자체의 Doc도 확인
				if !hasDtoTag && ts.Doc != nil {
					for _, c := range ts.Doc.List {
						if strings.Contains(c.Text, "@dto") {
							hasDtoTag = true
							break
						}
					}
				}

				// @dto 태그가 있으면 DTO로 등록
				if hasDtoTag {
					st.DTOs[ts.Name.Name] = true
					hasDtoTag = false // 다음 spec을 위해 리셋
				}

				// interface → Models에 등록
				if _, ok := ts.Type.(*ast.InterfaceType); ok {
					// interface의 메서드도 Models에 등록
					ms := ModelSymbol{Methods: make(map[string]MethodInfo)}
					iface := ts.Type.(*ast.InterfaceType)
					for _, method := range iface.Methods.List {
						if len(method.Names) > 0 {
							ms.Methods[method.Names[0].Name] = MethodInfo{}
						}
					}
					if len(ms.Methods) > 0 {
						st.Models[ts.Name.Name] = ms
					}
				}
			}

			// 패키지 레벨 func → Funcs로 등록
			fd, ok := decl.(*ast.FuncDecl)
			if ok && fd.Recv == nil {
				st.Funcs[fd.Name.Name] = true
			}
		}

		// ast.Decls에서 FuncDecl은 GenDecl과 별개로 순회
		for _, decl := range f.Decls {
			fd, ok := decl.(*ast.FuncDecl)
			if ok && fd.Recv == nil {
				st.Funcs[fd.Name.Name] = true
			}
		}
	}

	return nil
}

// LoadPackageInterfaces는 서비스 파일의 import 경로에서 패키지 접두사 모델의 Go interface를 파싱한다.
// 패키지명 → import 경로 매핑 후, 해당 경로에서 interface를 찾아 st.Models["pkg.Model"]에 등록.
func (st *SymbolTable) LoadPackageInterfaces(funcs []ssacparser.ServiceFunc, projectRoot string) {
	// 1. 모든 서비스 파일에서 패키지 접두사 모델 수집
	pkgModels := map[string]bool{} // "session" → true
	for _, sf := range funcs {
		for _, seq := range sf.Sequences {
			if seq.Package != "" {
				pkgModels[seq.Package] = true
			}
		}
	}
	if len(pkgModels) == 0 {
		return
	}

	// 2. 서비스 파일 import에서 패키지명 → import 경로 매핑
	pkgPaths := map[string]string{} // "session" → "myapp/session"
	for _, sf := range funcs {
		for _, imp := range sf.Imports {
			// import 경로의 마지막 segment가 패키지명
			segments := strings.Split(imp, "/")
			pkgName := segments[len(segments)-1]
			if pkgModels[pkgName] {
				pkgPaths[pkgName] = imp
			}
		}
	}

	// 3. 각 패키지 경로에서 Go interface 파싱
	for pkgName, impPath := range pkgPaths {
		// projectRoot 기준으로 경로 탐색 (상대 경로)
		dir := filepath.Join(projectRoot, impPath)
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			continue
		}
		st.loadPackageGoInterfaces(pkgName, dir)
	}
}

// loadPackageGoInterfaces는 디렉토리에서 Go interface를 파싱하여 "pkg.Model" 키로 등록한다.
// 또한 {Method}Request struct를 파싱하여 ParamTypes에 필드 타입을 저장한다.
func (st *SymbolTable) loadPackageGoInterfaces(pkgName, dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}

	fset := token.NewFileSet()
	// 1차: Request struct 수집
	requestStructs := map[string]map[string]string{} // "VerifyPasswordRequest" → {"Email": "string", "Password": "string"}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") {
			continue
		}
		f, err := parser.ParseFile(fset, filepath.Join(dir, entry.Name()), nil, parser.ParseComments)
		if err != nil {
			continue
		}
		for _, decl := range f.Decls {
			gd, ok := decl.(*ast.GenDecl)
			if !ok {
				continue
			}
			for _, spec := range gd.Specs {
				ts, ok := spec.(*ast.TypeSpec)
				if !ok {
					continue
				}
				if !strings.HasSuffix(ts.Name.Name, "Request") {
					continue
				}
				st2, ok := ts.Type.(*ast.StructType)
				if !ok {
					continue
				}
				fields := map[string]string{}
				for _, field := range st2.Fields.List {
					typeName := exprToGoType(field.Type)
					for _, name := range field.Names {
						fields[name.Name] = typeName
					}
				}
				if len(fields) > 0 {
					requestStructs[ts.Name.Name] = fields
				}
			}
		}
	}

	// 2차: interface 파싱
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") {
			continue
		}

		f, err := parser.ParseFile(fset, filepath.Join(dir, entry.Name()), nil, parser.ParseComments)
		if err != nil {
			continue
		}

		for _, decl := range f.Decls {
			gd, ok := decl.(*ast.GenDecl)
			if !ok {
				continue
			}
			for _, spec := range gd.Specs {
				ts, ok := spec.(*ast.TypeSpec)
				if !ok {
					continue
				}
				iface, ok := ts.Type.(*ast.InterfaceType)
				if !ok {
					continue
				}
				ms := ModelSymbol{Methods: make(map[string]MethodInfo)}
				for _, method := range iface.Methods.List {
					if len(method.Names) > 0 {
						methodName := method.Names[0].Name
						var params []string
						if ft, ok := method.Type.(*ast.FuncType); ok && ft.Params != nil {
							for _, param := range ft.Params.List {
								if isContextType(param.Type) {
									continue
								}
								for _, name := range param.Names {
									params = append(params, name.Name)
								}
							}
						}
						mi := MethodInfo{Params: params}
						// Request struct 매칭: {MethodName}Request
						reqStructName := methodName + "Request"
						if fields, ok := requestStructs[reqStructName]; ok {
							mi.ParamTypes = fields
						}
						ms.Methods[methodName] = mi
					}
				}
				if len(ms.Methods) > 0 {
					// "Model" suffix 제거: "SessionModel" → "Session"
					modelName := ts.Name.Name
					if strings.HasSuffix(modelName, "Model") {
						modelName = modelName[:len(modelName)-5]
					}
					key := pkgName + "." + modelName
					st.Models[key] = ms
				}
			}
		}
	}

	// 3차: standalone function 파싱 (@call 대상)
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") {
			continue
		}
		f, err := parser.ParseFile(fset, filepath.Join(dir, entry.Name()), nil, parser.ParseComments)
		if err != nil {
			continue
		}
		for _, decl := range f.Decls {
			fd, ok := decl.(*ast.FuncDecl)
			if !ok || fd.Recv != nil {
				continue
			}
			funcName := fd.Name.Name
			reqStructName := funcName + "Request"
			if _, ok := requestStructs[reqStructName]; !ok {
				continue
			}

			// @error 어노테이션 파싱
			errStatus := 0
			if fd.Doc != nil {
				for _, comment := range fd.Doc.List {
					text := strings.TrimSpace(strings.TrimPrefix(comment.Text, "//"))
					if strings.HasPrefix(text, "@error ") {
						if code, err := strconv.Atoi(strings.TrimSpace(text[7:])); err == nil {
							errStatus = code
						}
					}
				}
			}

			modelKey := pkgName + "._func"
			ms, exists := st.Models[modelKey]
			if !exists {
				ms = ModelSymbol{Methods: make(map[string]MethodInfo)}
			}
			mi := MethodInfo{
				ParamTypes: requestStructs[reqStructName],
				ErrStatus:  errStatus,
			}
			ms.Methods[funcName] = mi
			st.Models[modelKey] = ms
		}
	}
}

// exprToGoType는 ast.Expr를 Go 타입 문자열로 변환한다.
func exprToGoType(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.SelectorExpr:
		if x, ok := t.X.(*ast.Ident); ok {
			return x.Name + "." + t.Sel.Name
		}
	case *ast.StarExpr:
		return "*" + exprToGoType(t.X)
	case *ast.ArrayType:
		return "[]" + exprToGoType(t.Elt)
	}
	return "interface{}"
}

// isContextType는 ast.Expr이 context.Context 타입인지 확인한다.
func isContextType(expr ast.Expr) bool {
	sel, ok := expr.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	ident, ok := sel.X.(*ast.Ident)
	if !ok {
		return false
	}
	return ident.Name == "context" && sel.Sel.Name == "Context"
}

// --- OpenAPI YAML 구조체 ---

type openAPISpec struct {
	Paths      map[string]openAPIPathItem `yaml:"paths"`
	Components openAPIComponents          `yaml:"components"`
}

type openAPIComponents struct {
	Schemas map[string]openAPISchema `yaml:"schemas"`
}

type openAPISchema struct {
	Type       string                   `yaml:"type"`
	Format     string                   `yaml:"format"`
	Properties map[string]openAPISchema `yaml:"properties"`
	Ref        string                   `yaml:"$ref"`
}

type openAPIPathItem struct {
	Get    *openAPIOperation `yaml:"get"`
	Post   *openAPIOperation `yaml:"post"`
	Put    *openAPIOperation `yaml:"put"`
	Delete *openAPIOperation `yaml:"delete"`
}

func (p openAPIPathItem) operations() []*openAPIOperation {
	var ops []*openAPIOperation
	for _, op := range []*openAPIOperation{p.Get, p.Post, p.Put, p.Delete} {
		if op != nil {
			ops = append(ops, op)
		}
	}
	return ops
}

type openAPIOperation struct {
	OperationID string                     `yaml:"operationId"`
	Parameters  []openAPIParameter         `yaml:"parameters"`
	RequestBody *openAPIRequestBody        `yaml:"requestBody"`
	Responses   map[string]openAPIResponse `yaml:"responses"`
	XPagination *XPagination               `yaml:"x-pagination"`
	XSort       *XSort                     `yaml:"x-sort"`
	XFilter     *XFilter                   `yaml:"x-filter"`
	XInclude    *XInclude                  `yaml:"x-include"`
}

type openAPIParameter struct {
	Name   string          `yaml:"name"`
	In     string          `yaml:"in"`
	Schema openAPISchema   `yaml:"schema"`
}

type openAPIRequestBody struct {
	Content map[string]openAPIMediaType `yaml:"content"`
}

type openAPIResponse struct {
	Content map[string]openAPIMediaType `yaml:"content"`
}

type openAPIMediaType struct {
	Schema openAPISchema `yaml:"schema"`
}

// loadDDL은 db/ 디렉토리의 DDL .sql 파일에서 CREATE TABLE 문의 컬럼 타입을 추출한다.
func (st *SymbolTable) loadDDL(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}

		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			return err
		}

		parseDDLTables(string(data), st.DDLTables)
	}
	return nil
}

// parseDDLTables는 CREATE TABLE 문에서 컬럼명, 타입, FK, 인덱스를 추출한다.
func parseDDLTables(content string, tables map[string]DDLTable) {
	lines := strings.Split(content, "\n")
	var currentTable string

	for _, line := range lines {
		line = strings.TrimSpace(line)
		upper := strings.ToUpper(line)

		// CREATE INDEX idx_name ON tablename (col1, col2);
		if strings.HasPrefix(upper, "CREATE INDEX") || strings.HasPrefix(upper, "CREATE UNIQUE INDEX") {
			parseCreateIndex(line, tables)
			continue
		}

		// CREATE TABLE tablename (
		if strings.HasPrefix(upper, "CREATE TABLE") {
			parts := strings.Fields(line)
			for i, p := range parts {
				pu := strings.ToUpper(p)
				if pu == "TABLE" && i+1 < len(parts) {
					currentTable = strings.Trim(parts[i+1], "( ")
					tables[currentTable] = DDLTable{Columns: make(map[string]string)}
					break
				}
			}
			continue
		}

		if currentTable == "" {
			continue
		}

		// 테이블 정의 종료
		if strings.HasPrefix(line, ")") {
			currentTable = ""
			continue
		}

		// 독립 FOREIGN KEY: CONSTRAINT fk_name FOREIGN KEY (col) REFERENCES table(col)
		if strings.HasPrefix(upper, "CONSTRAINT") || strings.HasPrefix(upper, "FOREIGN") {
			if fk, ok := parseConstraintFK(line); ok {
				if t, exists := tables[currentTable]; exists {
					t.ForeignKeys = append(t.ForeignKeys, fk)
					tables[currentTable] = t
				}
			}
			continue
		}

		// PRIMARY KEY → PK 컬럼 추출
		if strings.HasPrefix(upper, "PRIMARY") {
			if t, ok := tables[currentTable]; ok {
				t.PrimaryKey = extractParenColumns(line)
				tables[currentTable] = t
			}
			continue
		}

		// UNIQUE 제약 (독립 라인) → unique index 추가
		if strings.HasPrefix(upper, "UNIQUE") {
			if t, ok := tables[currentTable]; ok {
				cols := extractParenColumns(line)
				if len(cols) > 0 {
					t.Indexes = append(t.Indexes, Index{Name: "unique_" + strings.Join(cols, "_"), Columns: cols, IsUnique: true})
					tables[currentTable] = t
				}
			}
			continue
		}

		// CHECK → skip
		if strings.HasPrefix(upper, "CHECK") || line == "" {
			continue
		}

		// 컬럼 라인: column_name TYPE ...
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}

		colName := parts[0]
		colType := strings.ToUpper(parts[1])
		colType = strings.TrimSuffix(colType, ",")

		goType := pgTypeToGo(colType)
		if t, ok := tables[currentTable]; ok {
			t.Columns[colName] = goType
			t.ColumnOrder = append(t.ColumnOrder, colName)

			// 인라인 PRIMARY KEY
			if strings.Contains(upper, "PRIMARY KEY") {
				t.PrimaryKey = append(t.PrimaryKey, colName)
			}

			// 인라인 UNIQUE
			if strings.Contains(upper, "UNIQUE") && !strings.Contains(upper, "PRIMARY") {
				t.Indexes = append(t.Indexes, Index{Name: colName + "_unique", Columns: []string{colName}, IsUnique: true})
			}

			// 인라인 FK: column_name TYPE ... REFERENCES table(col)
			if fk, ok := parseInlineFK(colName, parts); ok {
				t.ForeignKeys = append(t.ForeignKeys, fk)
			}
			tables[currentTable] = t
		}
	}
}

// parseInlineFK는 컬럼 정의에서 인라인 REFERENCES를 파싱한다.
// e.g. "user_id BIGINT NOT NULL REFERENCES users(id)"
func parseInlineFK(colName string, parts []string) (ForeignKey, bool) {
	for i, p := range parts {
		if strings.ToUpper(p) == "REFERENCES" && i+1 < len(parts) {
			ref := parts[i+1]
			ref = strings.TrimSuffix(ref, ",")
			refTable, refCol := parseRef(ref)
			if refTable != "" {
				return ForeignKey{Column: colName, RefTable: refTable, RefColumn: refCol}, true
			}
		}
	}
	return ForeignKey{}, false
}

// parseConstraintFK는 독립 FOREIGN KEY 절을 파싱한다.
// e.g. "CONSTRAINT fk_user FOREIGN KEY (user_id) REFERENCES users(id)"
// e.g. "FOREIGN KEY (user_id) REFERENCES users(id)"
func parseConstraintFK(line string) (ForeignKey, bool) {
	upper := strings.ToUpper(line)
	fkIdx := strings.Index(upper, "FOREIGN KEY")
	refIdx := strings.Index(upper, "REFERENCES")
	if fkIdx < 0 || refIdx < 0 {
		return ForeignKey{}, false
	}

	// FOREIGN KEY (col) 부분에서 컬럼 추출
	between := line[fkIdx+len("FOREIGN KEY") : refIdx]
	col := extractParenContent(between)
	if col == "" {
		return ForeignKey{}, false
	}

	// REFERENCES table(col) 부분
	after := strings.TrimSpace(line[refIdx+len("REFERENCES"):])
	after = strings.TrimSuffix(after, ",")
	refTable, refCol := parseRef(after)
	if refTable == "" {
		return ForeignKey{}, false
	}

	return ForeignKey{Column: col, RefTable: refTable, RefColumn: refCol}, true
}

// parseCreateIndex는 CREATE INDEX 문을 파싱한다.
// e.g. "CREATE INDEX idx_name ON tablename (col1, col2);"
func parseCreateIndex(line string, tables map[string]DDLTable) {
	upper := strings.ToUpper(line)
	onIdx := strings.Index(upper, " ON ")
	if onIdx < 0 {
		return
	}

	// 인덱스 이름: CREATE [UNIQUE] INDEX idx_name ON ...
	parts := strings.Fields(line[:onIdx])
	idxName := ""
	for i, p := range parts {
		if strings.ToUpper(p) == "INDEX" && i+1 < len(parts) {
			idxName = parts[i+1]
			break
		}
	}

	// ON tablename (col1, col2)
	after := strings.TrimSpace(line[onIdx+4:])
	afterParts := strings.SplitN(after, "(", 2)
	if len(afterParts) < 2 {
		return
	}

	tableName := strings.TrimSpace(afterParts[0])
	colsPart := strings.TrimSuffix(strings.TrimSpace(afterParts[1]), ");")
	colsPart = strings.TrimSuffix(colsPart, ")")

	var cols []string
	for _, c := range strings.Split(colsPart, ",") {
		c = strings.TrimSpace(c)
		if c != "" {
			cols = append(cols, c)
		}
	}

	isUnique := strings.Contains(strings.ToUpper(line), "UNIQUE")
	if t, ok := tables[tableName]; ok && len(cols) > 0 {
		t.Indexes = append(t.Indexes, Index{Name: idxName, Columns: cols, IsUnique: isUnique})
		tables[tableName] = t
	}
}

// extractParenColumns는 "PRIMARY KEY (col1, col2)" 등에서 괄호 안 컬럼을 추출한다.
func extractParenColumns(line string) []string {
	parenIdx := strings.Index(line, "(")
	if parenIdx < 0 {
		return nil
	}
	inner := line[parenIdx+1:]
	inner = strings.TrimSuffix(strings.TrimSpace(inner), ",")
	inner = strings.TrimSuffix(inner, ");")
	inner = strings.TrimSuffix(inner, ")")
	var cols []string
	for _, c := range strings.Split(inner, ",") {
		c = strings.TrimSpace(c)
		if c != "" {
			cols = append(cols, c)
		}
	}
	return cols
}

// parseRef는 "users(id)" → ("users", "id") を파싱한다.
func parseRef(s string) (table, col string) {
	s = strings.TrimSpace(s)
	parenIdx := strings.Index(s, "(")
	if parenIdx < 0 {
		return s, ""
	}
	table = s[:parenIdx]
	col = strings.TrimSuffix(s[parenIdx+1:], ")")
	col = strings.TrimSuffix(col, ",")
	return table, col
}

// extractParenContent는 "(content)" 에서 content를 추출한다.
func extractParenContent(s string) string {
	open := strings.Index(s, "(")
	close := strings.Index(s, ")")
	if open < 0 || close < 0 || close <= open {
		return ""
	}
	return strings.TrimSpace(s[open+1 : close])
}

// pgTypeToGo는 PostgreSQL 타입을 Go 타입으로 매핑한다.
func pgTypeToGo(pgType string) string {
	switch pgType {
	case "BIGINT", "BIGSERIAL", "INTEGER", "SERIAL", "INT", "SMALLINT":
		return "int64"
	case "VARCHAR", "TEXT", "UUID", "CHAR":
		return "string"
	case "BOOLEAN", "BOOL":
		return "bool"
	case "TIMESTAMPTZ", "TIMESTAMP", "DATE":
		return "time.Time"
	case "NUMERIC", "DECIMAL", "REAL", "FLOAT", "DOUBLE":
		return "float64"
	default:
		// VARCHAR(255) 같은 경우
		if strings.HasPrefix(pgType, "VARCHAR") || strings.HasPrefix(pgType, "CHAR") {
			return "string"
		}
		return "string"
	}
}

// oaTypeToGo는 OpenAPI type+format을 Go 타입으로 변환한다.
func oaTypeToGo(oaType, format string) string {
	switch oaType {
	case "integer":
		if format == "int64" {
			return "int64"
		}
		return "int"
	case "number":
		return "float64"
	case "boolean":
		return "bool"
	default: // string, string+uuid 등
		return "string"
	}
}

// collectSchemaFields는 인라인 properties와 $ref 모두에서 필드를 수집한다.
func collectSchemaFields(schema openAPISchema, schemas map[string]openAPISchema) []string {
	var fields []string

	// 인라인 properties
	for k := range schema.Properties {
		fields = append(fields, k)
	}

	// $ref 해결
	if schema.Ref != "" {
		name := schema.Ref[strings.LastIndex(schema.Ref, "/")+1:]
		if resolved, ok := schemas[name]; ok {
			for k := range resolved.Properties {
				fields = append(fields, k)
			}
		}
	}

	return fields
}
