package validator

import "fmt"

// ValidationError는 검증 에러 하나를 나타낸다.
type ValidationError struct {
	FileName string // 원본 파일명
	FuncName string // 함수명
	SeqIndex int    // sequence 인덱스
	Tag      string // 관련 태그 (e.g. "@model", "@action")
	Message  string // 에러 메시지
	Level    string // "ERROR" 또는 "WARNING" (빈 문자열이면 ERROR)
}

func (e ValidationError) Error() string {
	level := e.Level
	if level == "" {
		level = "ERROR"
	}
	return fmt.Sprintf("%s: %s:%s:seq[%d] %s — %s", level, e.FileName, e.FuncName, e.SeqIndex, e.Tag, e.Message)
}

// IsWarning은 이 에러가 경고인지 반환한다.
func (e ValidationError) IsWarning() bool {
	return e.Level == "WARNING"
}
