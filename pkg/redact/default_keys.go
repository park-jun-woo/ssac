//ff:type feature=pkg-redact type=data
//ff:what DefaultKeys — 자주 쓰이는 민감 필드 키 집합
package redact

// DefaultKeys is the baseline map of sensitive attribute keys used by
// ReplaceAttr. Generated logger init copies this map and extends it with
// DDL `-- @sensitive` column names.
//
// Keys are all lowercase; ReplaceAttr lowercases the incoming attr.Key
// before lookup.
var DefaultKeys = map[string]bool{
	"password":      true,
	"password_hash": true,
	"passwordhash":  true,
	"secret":        true,
	"token":         true,
	"access_token":  true,
	"refresh_token": true,
	"api_key":       true,
	"apikey":        true,
	"ssn":           true,
	"credit_card":   true,
	"cvv":           true,
	"authorization": true,
}
