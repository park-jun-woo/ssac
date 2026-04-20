# pkg/text

텍스트 변환/가공 유틸리티 (slug 생성, HTML 새니타이즈, 유니코드 안전 truncate).

## 개요

SSaC `@call text.Func({...})` 시퀀스로 호출되는 텍스트 처리 패키지. 세 가지 함수를 제공한다: URL-safe slug 변환(`GenerateSlug`), XSS 방지용 HTML 새니타이즈(`SanitizeHTML`), 유니코드 안전 말줄임 처리(`TruncateText`). 외부 의존 라이브러리는 `github.com/gosimple/slug`와 `github.com/microcosm-cc/bluemonday`이며, 후자는 UGC(User Generated Content) 정책을 사용한다.

## 공개 API

### `GenerateSlug(req GenerateSlugRequest) (GenerateSlugResponse, error)`

텍스트를 URL-safe slug(소문자 + 하이픈 구분)로 변환한다. `gosimple/slug` 라이브러리를 사용해 다국어 transliteration을 지원한다.

#### GenerateSlugRequest

| 필드 | 타입 | 설명 |
|---|---|---|
| Text | string | 원본 텍스트 |

#### GenerateSlugResponse

| 필드 | 타입 | 설명 |
|---|---|---|
| Slug | string | 변환된 slug |

### `SanitizeHTML(req SanitizeHTMLRequest) (SanitizeHTMLResponse, error)`

`bluemonday.UGCPolicy()`를 적용해 HTML에서 스크립트, 이벤트 핸들러, 위험 속성을 제거한다. 댓글, 게시글 본문 등 사용자 입력 HTML을 저장하기 전 호출한다.

#### SanitizeHTMLRequest

| 필드 | 타입 | 설명 |
|---|---|---|
| HTML | string | 원본 HTML |

#### SanitizeHTMLResponse

| 필드 | 타입 | 설명 |
|---|---|---|
| Sanitized | string | 새니타이즈된 HTML |

### `TruncateText(req TruncateTextRequest) (TruncateTextResponse, error)`

`[]rune` 변환으로 유니코드 문자 단위로 잘라 깨진 문자가 나오지 않는다. 원본이 `MaxLength` 이하면 그대로 반환. `Suffix`가 빈 문자열이면 `"..."`를 사용한다.

#### TruncateTextRequest

| 필드 | 타입 | 설명 |
|---|---|---|
| Text | string | 원본 텍스트 |
| MaxLength | int | 최대 rune 개수 |
| Suffix | string | 말줄임 접미사(기본 `"..."`) |

#### TruncateTextResponse

| 필드 | 타입 | 설명 |
|---|---|---|
| Truncated | string | 자른 결과 |

## 사용 예시

### SSaC 시퀀스

```go
// @call string slug = text.GenerateSlug({Text: request.Title})
// @call string clean = text.SanitizeHTML({HTML: request.Body})
// @call string summary = text.TruncateText({
//   Text: post.Content,
//   MaxLength: 120,
//   Suffix: "…"
// })
```

### Go 직접 호출

```go
import "github.com/park-jun-woo/ssac/pkg/text"

s, _ := text.GenerateSlug(text.GenerateSlugRequest{Text: "Hello World 한글"})
// s.Slug == "hello-world-hangeul"

h, _ := text.SanitizeHTML(text.SanitizeHTMLRequest{
    HTML: `<p onclick="x()">hi</p><script>evil()</script>`,
})
// h.Sanitized == "<p>hi</p>"

t, _ := text.TruncateText(text.TruncateTextRequest{
    Text:      "긴 문장입니다",
    MaxLength: 3,
})
// t.Truncated == "긴 문..."
```
