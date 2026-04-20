# pkg/config

환경 변수 접근을 위한 얇은 래퍼 — `os.Getenv` 기반.

## 개요

`pkg/config`는 인프라 설정값(DB DSN, 외부 API 키 등)을 환경 변수에서 읽는 두 개의
단순 함수만 제공한다: `Get`(비어있으면 `""`)과 `MustGet`(비어있으면 panic).
내부적으로 `os.Getenv`를 그대로 호출할 뿐이며, 파일 파싱/타입 변환/디폴트값
머지 기능은 일부러 포함하지 않았다.

## 설계 철학

SSaC DSL의 예약 소스 중 `config.*`는 **입력으로 허용되지 않는다** (validator ERROR).
인프라 설정은 비즈니스 로직의 시퀀스 파라미터가 아니라 "프로세스가 시작될 때
주변 환경에서 한 번 읽어야 하는 값"이기 때문이다. 따라서 생성된 함수 본체나
부팅 훅에서 `config.MustGet("DB_DSN")` 혹은 `os.Getenv` 를 직접 호출한다.

이 패키지는 SSaC `@call` 시퀀스에서 쓰기 위한 게 아니라 **수기로 작성하는 init/
main 코드**에서 쓰이는 유틸이다. Request/Response 구조체도 없다.

## 공개 API

| 함수 | 시그니처 | 동작 |
|---|---|---|
| `Get` | `Get(key string) string` | 값 반환. 미설정이면 `""` (에러 없음) |
| `MustGet` | `MustGet(key string) string` | 값 반환. 미설정/빈 문자열이면 `panic("required env var not set: " + key)` |

에러 조건:
- `Get`: 에러를 반환하지 않는다. 미설정과 빈 문자열을 구분하지 않는다.
- `MustGet`: 값이 `""`일 때 panic. 복구 가능한 에러가 아니라 부팅 시 Fail-Fast 용도로 설계됐다.

테스트 (`test_*_test.go`)가 네 가지 경로를 모두 확인한다: `Get` 성공/빈 값,
`MustGet` 성공/panic.

## 사용 예시

부팅 시 필수값 검증:

```go
func main() {
    dsn    := config.MustGet("DB_DSN")        // 없으면 panic
    port   := config.Get("PORT")              // 없으면 ""
    if port == "" { port = "8080" }           // 호출 측에서 디폴트 처리

    db, _ := sql.Open("postgres", dsn)
    r := gin.Default()
    _ = r.Run(":" + port)
}
```

SSaC 생성 함수 내부에서 직접:

```go
// 생성 코드 내 @call 이전/이후에 수기로 붙이는 초기화 블록
apiKey := config.MustGet("STRIPE_API_KEY")
client := stripe.New(apiKey)
```

주의: SSaC DSL에서 `config.STRIPE_API_KEY` 같은 입력 참조는 validator가 ERROR로 처리한다.
설정값이 필요한 시퀀스는 `@call pkg.Func({...})` 호출 전후로 직접 환경 변수를 읽어
실제 값(또는 클라이언트 객체)으로 치환한 뒤 시퀀스에 넘기는 구조로 작성한다.
