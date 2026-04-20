# pkg/image

이미지 바이트를 입력받아 OG 이미지 및 썸네일 PNG로 변환하는 유틸 패키지.

## 개요

`pkg/image`는 SSaC 서비스 시퀀스에서 `@call image.OgImage({...})` 또는 `@call image.Thumbnail({...})`로 호출되는 유틸 패키지다. 입력은 `[]byte` 형태의 원본 이미지, 출력은 고정 규격으로 리사이즈/크롭된 PNG `[]byte`다. 순수 함수로 상태가 없으며, 외부 저장소 의존성도 없다(저장은 `pkg/file` / `pkg/storage`가 담당).

이미지 처리는 [`github.com/disintegration/imaging`](https://github.com/disintegration/imaging) 라이브러리를 사용하며, Fill 방식(중앙 기준 크롭+스케일) + Lanczos 필터로 비율을 유지한 채 목표 크기를 채운다.

## 모델/인터페이스

공개 인터페이스 없음. 각 함수가 요청/응답 구조체만 사용한다.

| 타입 | 설명 |
|---|---|
| `OgImageRequest` / `OgImageResponse` | OG 이미지(1200x630) 입출력 |
| `ThumbnailRequest` / `ThumbnailResponse` | 썸네일(200x200) 입출력 |

## 구현체

단일 구현 — 함수형 API. 내부 흐름:

1. `imaging.Decode(bytes.NewReader(req.Data))` — PNG/JPEG/GIF 등 자동 포맷 감지
2. `imaging.Fill(src, W, H, imaging.Center, imaging.Lanczos)` — 중앙 기준 크롭/리사이즈
3. `imaging.Encode(&buf, img, imaging.PNG)` — 항상 PNG로 출력

별도 설정이나 생성자가 없고, `context.Context`도 받지 않는다(순수 계산).

의존성: `github.com/disintegration/imaging` (MIT).

## 공개 API

### `OgImage(OgImageRequest) (OgImageResponse, error)`

이미지를 **1200x630** OG 규격으로 크롭/리사이즈한 뒤 PNG로 인코딩한다.

| 필드 | 타입 | 설명 |
|---|---|---|
| Req.Data | []byte | 원본 이미지(JPEG/PNG/GIF 등) |
| Resp.Data | []byte | 변환된 PNG 바이트 |

에러:
- `imaging.Decode` 실패 — 지원하지 않는 포맷 또는 깨진 이미지
- `imaging.Encode` 실패 — 인코딩 I/O 오류

### `Thumbnail(ThumbnailRequest) (ThumbnailResponse, error)`

이미지를 **200x200** 정사각형 썸네일로 크롭/리사이즈한 뒤 PNG로 인코딩한다.

| 필드 | 타입 | 설명 |
|---|---|---|
| Req.Data | []byte | 원본 이미지 |
| Resp.Data | []byte | 변환된 PNG 바이트 |

에러: OgImage와 동일(디코드/인코드 실패).

## 사용 예시

### SSaC 시퀀스

```go
// OG 이미지 생성 후 S3 업로드
// @call image.OgImageResponse og = image.OgImage({Data: request.Data})
// @call storage.UploadFileResponse up = storage.UploadFile({
//     Bucket: "og",
//     Key: request.Key,
//     Data: og.Data,
//     ContentType: "image/png",
//     Region: "ap-northeast-2"
// })
// @response { url: up.URL }

// 썸네일 생성 후 로컬 저장
// @call image.ThumbnailResponse thumb = image.Thumbnail({Data: request.Data})
// @call file.Upload({Key: request.Key, Body: thumb.Data})
```

### Go 직접 호출

```go
raw, _ := os.ReadFile("photo.jpg")

og, err := image.OgImage(image.OgImageRequest{Data: raw})
if err != nil { log.Fatal(err) }
_ = os.WriteFile("og.png", og.Data, 0o644)

thumb, err := image.Thumbnail(image.ThumbnailRequest{Data: raw})
if err != nil { log.Fatal(err) }
_ = os.WriteFile("thumb.png", thumb.Data, 0o644)
```
