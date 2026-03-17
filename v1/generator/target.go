package generator

import (
	"github.com/park-jun-woo/ssac/parser"
	"github.com/park-jun-woo/ssac/validator"
)

// Target은 특정 언어의 코드 생성기가 구현해야 하는 인터페이스이다.
type Target interface {
	// GenerateFunc는 하나의 서비스 함수를 타겟 언어 소스 코드로 변환한다.
	GenerateFunc(sf parser.ServiceFunc, st *validator.SymbolTable) ([]byte, error)

	// GenerateModelInterfaces는 서비스 함수에서 사용하는 모델 인터페이스를 생성한다.
	GenerateModelInterfaces(funcs []parser.ServiceFunc, st *validator.SymbolTable, outDir string) error

	// FileExtension은 생성 파일의 확장자를 반환한다. (예: ".go", ".java", ".ts")
	FileExtension() string
}

// DefaultTarget은 Go Target을 반환한다.
func DefaultTarget() Target {
	return &GoTarget{}
}

// 컴파일 타임 인터페이스 구현 확인
var _ Target = (*GoTarget)(nil)
