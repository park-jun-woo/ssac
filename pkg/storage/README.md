# pkg/storage

S3 호환 스토리지(AWS S3, MinIO 등)에 대한 Presigned URL / Upload / Delete 유틸 패키지.

## 개요

`pkg/storage`는 `@call storage.PresignURL({...})`, `@call storage.UploadFile({...})`, `@call storage.DeleteFile({...})` 시퀀스로 호출되는 유틸 패키지다. `pkg/file`과 달리 전역 `FileModel`을 주입받지 않고, 매 호출마다 요청 파라미터(`Bucket/Endpoint/Region`)로 S3 클라이언트를 새로 생성한다. 주 용도는 presigned GET URL 발급이며, 보조적으로 바이트 업로드/삭제도 제공한다.

`Endpoint`가 비어 있으면 AWS 기본 엔드포인트를 사용하고, MinIO 등 커스텀 엔드포인트를 주면 `UsePathStyle=true`로 동작한다. 인증은 AWS SDK 기본 체인(`config.LoadDefaultConfig`)을 따른다.

## 모델/인터페이스

내부용 클라이언트 팩토리만 존재한다 (공개 인터페이스 없음).

| 함수 | 설명 |
|---|---|
| `newS3Client(ctx, endpoint, region)` | `*s3.Client` 생성. `endpoint != ""`일 때 `BaseEndpoint` 및 `UsePathStyle=true` 적용 |

## 구현체

S3 단일 백엔드. 각 공개 함수가 내부에서 `newS3Client(ctx, req.Endpoint, req.Region)`을 호출한다.

- 자격증명: `config.LoadDefaultConfig(ctx, ...)` — 환경 변수 / IAM 역할 / `~/.aws/credentials` 등 SDK 기본 체인
- Region: `req.Region`으로 명시 주입 필수
- Endpoint:
  - 빈 문자열 → AWS S3 (virtual-hosted style URL: `https://<bucket>.s3.<region>.amazonaws.com/<key>`)
  - 커스텀 값 → path style (`<endpoint>/<bucket>/<key>`) — MinIO, LocalStack, Cloudflare R2 등

별도 `Init()`이 없으므로 각 호출이 독립적이며, 호출마다 클라이언트 생성 오버헤드가 발생한다.

## 공개 API

### `PresignURL(ctx, PresignURLRequest) (PresignURLResponse, error)`

서명된 GET URL을 발급한다.

| 필드 | 타입 | 설명 |
|---|---|---|
| Req.Bucket | string | 버킷명 |
| Req.Key | string | 객체 키 |
| Req.ExpiresIn | int | 만료 시간(초). `<=0`이면 기본 3600 |
| Req.Endpoint | string | 커스텀 엔드포인트(선택) |
| Req.Region | string | AWS 리전 |
| Resp.URL | string | presigned URL |

에러: 클라이언트 생성 실패, `PresignGetObject` 실패.

### `UploadFile(ctx, UploadFileRequest) (UploadFileResponse, error)`

바이트 데이터를 S3에 업로드한다.

| 필드 | 타입 | 설명 |
|---|---|---|
| Req.Bucket | string | 버킷명 |
| Req.Key | string | 객체 키 |
| Req.Data | []byte | 업로드 바이트 |
| Req.ContentType | string | MIME 타입 (`image/png` 등) |
| Req.Endpoint | string | 커스텀 엔드포인트(선택) |
| Req.Region | string | AWS 리전 |
| Resp.URL | string | 업로드된 객체 URL |

에러: 클라이언트 생성 실패, `PutObject` 실패.

### `DeleteFile(ctx, DeleteFileRequest) (DeleteFileResponse, error)`

S3 객체를 삭제한다.

| 필드 | 타입 | 설명 |
|---|---|---|
| Req.Bucket | string | 버킷명 |
| Req.Key | string | 객체 키 |
| Req.Endpoint | string | 커스텀 엔드포인트(선택) |
| Req.Region | string | AWS 리전 |
| Resp | (empty) | 본문 없음 |

에러: 클라이언트 생성 실패, `DeleteObject` 실패.

## 사용 예시

### SSaC 시퀀스

```go
// @call storage.PresignURLResponse signed = storage.PresignURL({
//     Bucket: "uploads",
//     Key: request.Key,
//     ExpiresIn: "900",
//     Region: "ap-northeast-2"
// })
// @response { url: signed.URL }

// @call storage.UploadFileResponse up = storage.UploadFile({
//     Bucket: "uploads",
//     Key: request.Key,
//     Data: request.Data,
//     ContentType: "image/png",
//     Region: "ap-northeast-2"
// })
// @response { url: up.URL }

// @call storage.DeleteFile({Bucket: "uploads", Key: request.Key, Region: "ap-northeast-2"})
```

### Go 직접 호출

```go
ctx := context.Background()

resp, err := storage.PresignURL(ctx, storage.PresignURLRequest{
    Bucket:    "uploads",
    Key:       "avatars/u1.png",
    ExpiresIn: 900,
    Region:    "ap-northeast-2",
})
if err != nil { log.Fatal(err) }
fmt.Println(resp.URL)

_, _ = storage.UploadFile(ctx, storage.UploadFileRequest{
    Bucket:      "uploads",
    Key:         "avatars/u1.png",
    Data:        pngBytes,
    ContentType: "image/png",
    Region:      "ap-northeast-2",
})
```
