package generator

import (
	"fmt"
	"sort"
	"strings"

	"github.com/geul-org/ssac/parser"
	"github.com/geul-org/ssac/validator"
	"github.com/ettle/strcase"
)

// --- Args → Go code ---

func buildArgsCode(args []parser.Arg) string {
	var parts []string
	for _, a := range args {
		parts = append(parts, argToCode(a))
	}
	return strings.Join(parts, ", ")
}

func argToCode(a parser.Arg) string {
	if a.Literal != "" {
		return `"` + a.Literal + `"`
	}
	if a.Source == "query" {
		return "opts"
	}
	if a.Source == "request" {
		return strcase.ToGoCamel(a.Field)
	}
	if a.Source == "currentUser" {
		return a.Source + "." + a.Field
	}
	if a.Source != "" {
		if a.Field == "" {
			return a.Source
		}
		return a.Source + "." + a.Field
	}
	return a.Field
}

// buildInputFieldsFromMap은 map[string]string을 Go struct 리터럴 필드로 변환한다.
func buildInputFieldsFromMap(inputs map[string]string) string {
	keys := make([]string, 0, len(inputs))
	for k := range inputs {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var fields []string
	for _, k := range keys {
		fields = append(fields, strcase.ToGoPascal(k)+": "+inputValueToCode(inputs[k]))
	}
	return strings.Join(fields, ", ")
}

// inputValueToCode는 inputs 값에 argToCode와 동일한 예약 소스 변환을 적용한다.
func inputValueToCode(val string) string {
	if val == "query" {
		return "opts"
	}
	if strings.HasPrefix(val, "request.") {
		return strcase.ToGoCamel(val[len("request."):])
	}
	// currentUser.Field, 일반 변수 → 그대로
	return val
}

// buildArgsCodeFromInputs는 Inputs map의 value만 추출하여 positional 함수 인자로 변환한다.
// paramOrder가 있으면 그 순서로 배치하고, 없으면 알파벳순 fallback.
func buildArgsCodeFromInputs(inputs map[string]string, paramOrder []string) string {
	if len(inputs) == 0 {
		return ""
	}

	var keys []string
	if len(paramOrder) > 0 {
		used := make(map[string]bool)
		for _, p := range paramOrder {
			if _, ok := inputs[p]; ok {
				keys = append(keys, p)
				used[p] = true
			}
		}
		// paramOrder에 없는 키 (query 등) → 마지막에 추가
		var extra []string
		for k := range inputs {
			if !used[k] {
				extra = append(extra, k)
			}
		}
		sort.Strings(extra)
		keys = append(keys, extra...)
	} else {
		// fallback: 알파벳순, query는 마지막
		var queryKey string
		for k := range inputs {
			if inputs[k] == "query" {
				queryKey = k
			} else {
				keys = append(keys, k)
			}
		}
		sort.Strings(keys)
		if queryKey != "" {
			keys = append(keys, queryKey)
		}
	}

	var parts []string
	for _, k := range keys {
		parts = append(parts, inputValueToCode(inputs[k]))
	}
	return strings.Join(parts, ", ")
}

// lookupParamOrder는 심볼 테이블에서 모델 메서드의 파라미터 순서를 조회한다.
func lookupParamOrder(model string, st *validator.SymbolTable) []string {
	parts := strings.SplitN(model, ".", 2)
	if len(parts) < 2 {
		return nil
	}
	ms, ok := st.Models[parts[0]]
	if !ok {
		return nil
	}
	mi, ok := ms.Methods[parts[1]]
	if !ok {
		return nil
	}
	return mi.Params
}

// buildPublishPayload는 publish의 Inputs를 map[string]any 리터럴 필드로 변환한다.
func buildPublishPayload(inputs map[string]string) string {
	keys := make([]string, 0, len(inputs))
	for k := range inputs {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var fields []string
	for _, k := range keys {
		fields = append(fields, fmt.Sprintf("\t\t%q: %s,", strcase.ToGoPascal(k), inputValueToCode(inputs[k])))
	}
	return strings.Join(fields, "\n")
}

// buildPublishOptions는 publish의 Options를 Go 코드로 변환한다.
func buildPublishOptions(options map[string]string) string {
	if len(options) == 0 {
		return ""
	}
	var parts []string
	keys := make([]string, 0, len(options))
	for k := range options {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		switch k {
		case "delay":
			parts = append(parts, fmt.Sprintf("queue.WithDelay(%s)", options[k]))
		case "priority":
			parts = append(parts, fmt.Sprintf("queue.WithPriority(%q)", options[k]))
		}
	}
	if len(parts) == 0 {
		return ""
	}
	return ", " + strings.Join(parts, ", ")
}

// hasQueryInput은 Inputs map에 query 예약 소스가 있는지 확인한다.
func hasQueryInput(inputs map[string]string) bool {
	for _, v := range inputs {
		if v == "query" {
			return true
		}
	}
	return false
}
