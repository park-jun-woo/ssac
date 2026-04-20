# pkg/file

파일/객체 저장소에 대한 Upload/Download/Delete 유틸 패키지. Local 디스크와 AWS S3 두 백엔드를 `FileModel` 인터페이스로 추상화한다.

## 개요

`pkg/file`은 SSaC가 생성한 서비스 코드가 `@call file.Upload({...})`, `@call file.Download({...})`, `@call file.Delete({...})` 시퀀스로 호출하는 유틸 패키지다. 백엔드는 `FileModel` 인터페이스로 분리되어 있으며, 애플리케이션 부트스트랩에서 `file.Init(model)`을 호출해 전역 `defaultModel`을 주입한다. 공개 API(`Upload`/`Download`/`Delete`)는 이 `defaultModel`을 위임 호출한다.

현재 구현은 로컬 파일시스템(`localFile`)과 AWS S3(`s3File`) 두 가지이며, 업로드 시 중첩 디렉토리는 자동으로 생성된다.

## 모델/인터페이스

| 타입 | 설명 |
|---|---|
| `FileModel` | 저장소 계약 인터페이스 — `Upload/Download/Delete(ctx, key, ...)` |
| `localFile` | 로컬 디스크 구현체 (`root` 경로 기준) |
| `s3File` | AWS S3 구현체 (`client`, `bucket`) |

`FileModel` 메서드:

| 메서드 | 시그니처 |
|---|---|
| Upload | `Upload(ctx context.Context, key string, body io.Reader) error` |
| Download | `Download(ctx context.Context, key string) (io.ReadCloser, error)` |
| Delete | `Delete(ctx context.Context, key string) error` |

## 구현체

### Local (`NewLocalFile`)

```go
func NewLocalFile(root string) FileModel
```

- 루트 디렉토리 `root` 기준으로 `filepath.Join(root, key)` 경로에 파일을 저장한다.
- Upload 시 `os.MkdirAll(dir, 0o755)`로 중첩 디렉토리를 자동 생성한다 (`a/b/c/d.txt` 허용).
- Download는 `os.Open`, Delete는 `os.Remove`를 사용한다.

### S3 (`NewS3File`)

```go
func NewS3File(client *s3.Client, bucket string) FileModel
```

- `github.com/aws/aws-sdk-go-v2/service/s3` 클라이언트와 버킷명을 주입받는다.
- Upload: `PutObject`, Download: `GetObject`(Body 반환), Delete: `DeleteObject`.
- S3 클라이언트 생성은 호출측 책임 (e.g. `pkg/storage` 참고 또는 `config.LoadDefaultConfig`).

### 주입

```go
file.Init(file.NewLocalFile("/var/uploads"))
// 또는
file.Init(file.NewS3File(s3Client, "my-bucket"))
```

## 공개 API

### `Upload(ctx, UploadRequest) (UploadResponse, error)`

| 필드 | 타입 | 설명 |
|---|---|---|
| Req.Key | string | 저장소 내 키/경로 |
| Req.Body | string | 업로드할 바이트(문자열로 전달) |
| Resp.Key | string | 업로드된 키 (에코) |

에러: `defaultModel.Upload` 실패 시 전파 (디렉토리 생성 실패, S3 PutObject 실패 등).

### `Download(ctx, DownloadRequest) (DownloadResponse, error)`

| 필드 | 타입 | 설명 |
|---|---|---|
| Req.Key | string | 저장소 내 키 |
| Resp.Body | string | 다운로드된 바이트(문자열) |

에러: 파일/객체 없음(404 매핑), I/O 실패. 주석에 `@error 404` 표기.

### `Delete(ctx, DeleteRequest) (DeleteResponse, error)`

| 필드 | 타입 | 설명 |
|---|---|---|
| Req.Key | string | 삭제할 키 |
| Resp | (empty) | 본문 없음 |

에러: 저장소 Delete 실패 시 전파.

## 사용 예시

### SSaC 시퀀스

```go
// @call file.UploadResponse up = file.Upload({Key: "avatars/u1.png", Body: request.Body})
// @response { key: up.Key }

// @call file.DownloadResponse dl = file.Download({Key: request.Key})
// @response { body: dl.Body }

// @delete file.Delete({Key: request.Key})
```

### Go 직접 호출

```go
func main() {
    file.Init(file.NewLocalFile("/var/uploads"))

    ctx := context.Background()
    _, err := file.Upload(ctx, file.UploadRequest{Key: "a/b/c.txt", Body: "hello"})
    if err != nil { log.Fatal(err) }

    out, _ := file.Download(ctx, file.DownloadRequest{Key: "a/b/c.txt"})
    fmt.Println(out.Body)

    _, _ = file.Delete(ctx, file.DeleteRequest{Key: "a/b/c.txt"})
}
```
