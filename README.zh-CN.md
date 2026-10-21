# FlashData Go

[English](README.md) · [FlashData 官网](https://flashdata.dev) · [API 文档](https://flashdata.dev/docs) · [Go API 参考](https://pkg.go.dev/github.com/flashdatadev/flashdata-go@v1.0.0) · [价格](https://flashdata.dev/pricing) · [v1.0.0 正式版](https://github.com/flashdatadev/flashdata-go/releases/tag/v1.0.0)

[FlashData](https://flashdata.dev) 的 Go SDK，通过统一 API 获取结构化 Google 搜索与 YouTube 数据。支持搜索、图片、新闻、购物、视频和地点，以及 YouTube 元数据、转录、字幕、频道、搜索建议和许可标签；媒体下载可直接交付到您自己的云存储。适用于 Go 后端团队、搜索与 AI 应用、研究人员和数据流水线，[FlashData CLI](https://github.com/flashdatadev/flashdata-cli) 已基于此 SDK 构建。

**版本 1.0.0。** 可通过 Go 安装此标签版本，或从 [v1.0.0 正式版](https://github.com/flashdatadev/flashdata-go/releases/tag/v1.0.0)下载模块归档及校验和。本项目没有部署配套的后端身份补丁。 需要 Go 1.25 或更高版本。现有 24 个开发者 API 操作和 15 个数据源与 Python、TypeScript SDK 对齐；新增密钥身份验证与代理接入能力，详见[契约状态](docs/api-contract.md)。

## 安装

在您自己的 Go 应用模块中执行：

```bash
go get github.com/flashdatadev/flashdata-go@v1.0.0
```

提交生成的 `go.mod` 与 `go.sum`，即可锁定 SDK v1.0.0 及其已验证校验和。Go 直接从公开源码标签安装，不依赖 GitHub Release 是否发布。详见 [v1.0.0 版本说明](docs/release-notes-1.0.0.md)。

SDK v1.0.0 已可通过 [Go 公共模块代理](https://proxy.golang.org/github.com/flashdatadev/flashdata-go/@v/list)下载，自动生成的 API 参考及示例见 [pkg.go.dev](https://pkg.go.dev/github.com/flashdatadev/flashdata-go@v1.0.0)。安装无需单独注册包仓库账户。

运行仓库示例或从源码开发 SDK：

```bash
git clone --branch v1.0.0 --depth 1 https://github.com/flashdatadev/flashdata-go.git
cd flashdata-go
go mod download
```

SDK 需要 Go 1.25 或更高版本。本地开发可使用 `go.work` 工作区；正常安装不需要相邻的 SDK 仓库。

## API 密钥与首次查询

从 [FlashData 账户](https://flashdata.dev) 获取密钥，并显式传给 `flashdata.New`。SDK 本身不会读取环境变量，也不会自动加载 `.env`。不要把密钥提交到版本控制。

```bash
export FLASHDATA_API_KEY="YOUR_API_KEY"
go run ./examples/google_search
```

PowerShell：

```powershell
$env:FLASHDATA_API_KEY="YOUR_API_KEY"
go run ./examples/google_search
```

```go
package main

import (
	"context"
	"encoding/json"
	flashdata "github.com/flashdatadev/flashdata-go"
	"log"
	"os"
	"time"
)

func main() {
	client, err := flashdata.New(flashdata.Options{APIKey: os.Getenv("FLASHDATA_API_KEY"), DataOrigin: os.Getenv("FLASHDATA_DATA_ORIGIN"), ManagementOrigin: os.Getenv("FLASHDATA_MANAGEMENT_ORIGIN")})
	if err != nil {
		log.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	result, err := client.Google.Search(ctx, flashdata.GoogleSearchInput{Query: "machine learning", Params: &flashdata.GoogleSearchParams{Num: flashdata.Ptr(int64(10)), Autocorrect: flashdata.Ptr(false)}})
	if err != nil {
		log.Fatal(err)
	}
	if err = json.NewEncoder(os.Stdout).Encode(result); err != nil {
		log.Fatal(err)
	}
}
```

可选参数使用指针。`flashdata.Ptr(false)` 会明确发送 false，`nil` 表示不设置并使用服务端默认行为。`Transcript` 与 `Captions` 保留各自的数据源；示例明确传入 `auto:true` 和 `auto:false`。下载仅支持后台模式，并且必须配置客户存储。

原始结果以 `json.RawMessage` 保留，包括新增字段和大整数。可以使用 `flashdata.DecodeResults[flashdata.GoogleSearchPayload](result)` 读取可选类型提示，也可解码为自定义结构体。提供方字段可能缺失或为 null。`DecodeResults[map[string]any]` 使用 `json.Number`，避免大整数被四舍五入。

## 后台任务、批量提交与分页

运行 `go run ./examples/background`，提交一个 YouTube 元数据任务、保存 `background.job.json`、等待完成并获取结果。示例会在提交前独占创建恢复文件，拒绝覆盖已有文件；只有明确创建新任务时才使用 `-record another.job.json`。通过 `go run ./examples/background -job job_YOUR_ID` 恢复已有任务，不会重新提交。`*Async` 方法返回 `*AsyncSubmission`，实时方法返回 `*DataQueryResponse`。后台查询可通过 `Query.CallbackURL` 设置回调，仅接受公开 HTTPS、443 端口地址。

`go run ./examples/batch` 提交 Google/YouTube 混合批次。`go run ./examples/pagination` 获取**分别计费的两页**搜索结果。批次大小及准入受当前账户限制约束。连接正式 FlashData 服务时，这些查询按正常规则计费；公开发现和账户读取免费。

SDK 不会自动重试提交。`SubmissionUnknownError` 表示请求可能已经被服务端接受；请保留状态，核实后再决定是否重试。返回的任务 ID 是恢复凭据。取消或超时仅停止本地等待，已经接受的服务端任务会继续执行。单次请求超时与整个轮询超时分别处理。

## 完整批量下载任务

Bulk 支持元数据、转录、字幕、许可标签和下载。流程为：创建 → 上传 ID 文件 → 启动 → 等待 → 流式读取回执。下载媒体直接写入客户自己的存储桶；NDJSON 包含每个条目的交付状态，不包含媒体二进制内容。

创建 `ids.txt`：

```text
dQw4w9WgXcQ
jNQXAC9IVRw
```

创建仅自己可访问的 `storage.json`，将所有占位值替换为真实且有权限使用的存储配置：

```json
{
  "storage_type": "s3",
  "storage_url": "your-bucket-name",
  "region": "us-east-1",
  "access_key": "YOUR_STORAGE_ACCESS_KEY",
  "secret_key": "YOUR_STORAGE_SECRET_KEY",
  "object_prefix": "flashdata/audio"
}
```

支持 S3、S3 兼容存储、OSS、GCS、Azure、COS、BOS 和 TOS。提供方凭据、写入权限、端点及媒体格式组合最终由服务端验证。GCS 使用 `service_account`，Azure 使用 `account_name` 和具备写权限的 `sas_token`，详见[存储与恢复](docs/bulk.md)。不要提交该文件。

完整 SDK 调用顺序如下，代码与英文版一致：

```go
package main

import (
    "context"
    "encoding/json"
    "log"
    "os"
    "time"

    flashdata "github.com/flashdatadev/flashdata-go"
)

func main() {
    client, err := flashdata.New(flashdata.Options{APIKey: os.Getenv("FLASHDATA_API_KEY"), DataOrigin: os.Getenv("FLASHDATA_DATA_ORIGIN"), ManagementOrigin: os.Getenv("FLASHDATA_MANAGEMENT_ORIGIN")})
    if err != nil { log.Fatal(err) }
    ctx, cancel := context.WithTimeout(context.Background(), 65*time.Minute)
    defer cancel()

    // storage.json contains your real customer-owned destination configuration.
    storageJSON, err := os.ReadFile("storage.json")
    if err != nil { log.Fatal("could not read storage.json") }
    var storage flashdata.YouTubeDownloadParamsStorage
    if err = json.Unmarshal(storageJSON, &storage); err != nil { log.Fatal("invalid storage JSON") }

    // Reserve both paths before creating work; never overwrite a saved job.
    file, err := os.OpenFile("download-results.jsonl", os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
    if err != nil { log.Fatal("choose a new result output path") }
    defer file.Close()
    recovery, err := os.OpenFile("download.job.json", os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
    if err != nil { log.Fatal("recover the saved job or choose a new recovery path") }
    defer recovery.Close()

    // Each line of ids.txt is one 11-character YouTube video ID.
    // Validate the input and output paths before creating a real job; the
    // runnable bulk example below performs these preflight checks.
    created, err := client.Bulk.Create(ctx, flashdata.CreateBulkJobRequest{
        Source: flashdata.SourceYouTubeDownload,
        Options: flashdata.YouTubeDownloadParams{
            DownloadType: flashdata.Ptr("audio"), OutputFormat: flashdata.Ptr("mp3"),
            Storage: storage,
        },
    })
    if err != nil { log.Fatal(err) }
    id := created.Job.ID
    log.Printf("Accepted %s; save this ID before continuing", id)
    if err = json.NewEncoder(recovery).Encode(map[string]string{"id": id}); err == nil {
        err = recovery.Sync()
    }
    if err != nil {
        log.Fatalf("accepted %s; could not persist recovery record", id)
    }
    if err = client.Bulk.UploadFile(ctx, created, "ids.txt"); err != nil { log.Fatal(err) }
    if _, err = client.Bulk.Start(ctx, id); err != nil { log.Fatal(err) }
    if _, err = client.Bulk.Wait(ctx, id, flashdata.PollOptions{Timeout: time.Hour}); err != nil { log.Fatal(err) }

    encoder := json.NewEncoder(file)
    failed := 0
    for row, err := range client.Bulk.Rows(ctx, id) {
        if err != nil { log.Fatalf("partial output retained for %s: %v", id, err) }
        if err = encoder.Encode(row); err != nil { log.Fatal("could not write results") }
        if string(row["status"]) != `"completed"` { failed++ }
    }
    if err = file.Sync(); err != nil { log.Fatal("could not flush results") }
    if failed != 0 { log.Fatalf("%d failed/invalid item receipts retained; inspect before retrying", failed) }
    log.Printf("Saved receipts for %s; media files are in your storage bucket", id)
}
```

带输入/输出预检查和任务恢复的可运行版本：

```bash
go run ./examples/bulk -source youtube_download -ids ids.txt -storage storage.json -out receipts.jsonl
# 恢复已启动的任务，使用新的输出文件名：
go run ./examples/bulk -job bulk_YOUR_ID -out recovered.jsonl
# 其他数据源使用相同流程：
go run ./examples/bulk -source youtube_metadata -ids ids.txt -out metadata.jsonl
```

示例只把已接受的任务 ID 保存到 `OUTPUT.job.json`，在创建任务前独占预留恢复文件，在上传前同步保存 ID，拒绝覆盖已有导出或恢复文件，并上传已经验证的输入快照。在报告 failed/invalid 条目前先保存回执；即使整个任务 completed，也必须逐条检查状态。`awaiting_upload` 需要原始签名上传并显式启动；paused 需要使用 `Bulk.Update` 修复暂停原因后显式恢复。分别使用 `-job ID -start` 或 `-job ID -resume`，不要自动创建替代任务。签名上传过期后不能通过重复创建来“恢复”，请检查任务状态或联系支持。

输入文件为 1 字节至 2 GiB，每行一个视频 ID。上传链接有效期 15 分钟；内部结果保留 24 小时，客户存储遵循自己的桶策略。默认响应及解压后单个分片上限为 64 MiB，单行上限为 16 MiB。`Rows` 和 `IterRows` 流式读取，提前退出循环会关闭流；`Part` 返回压缩字节。较大导出请明确调整限制。

示例仅在内存中保留签名上传能力，因此单独的 ID 文件无法恢复进程在上传前中断的情况。需要这种恢复能力时，应用必须先将完整创建响应持久化到安全存储，再进行上传；CLI 已实现相应日志。请求结果不确定时留下的空恢复文件必须先核对，不能直接创建替代任务。ID 文件不含凭据；存储配置 JSON 必须放在操作系统保护的目录中，单独设置 `0600` 不能构成安全的 Windows 凭据保险库。

## 接入流程与可信身份

已有账户可直接显式传入 `Options.APIKey`。可选代理接入使用协议 2、随机证明及现有浏览器认领页面。该能力受功能开关控制，未启用的部署可能返回 404；公开能力发现不会创建试用。

`go run ./examples/onboarding` 读取接入能力。[完整接入指南](docs/onboarding.md) 说明持久化注册、浏览器批准、密钥交接恢复、全新进程验证及 ACK。SDK 提供全部八个机器侧操作；浏览器确认、批准、撤销继续由已验证的 FlashData 浏览器会话处理。

`Account.Identity` 返回服务端核实的所有权与 `recurringAllowed`。对应后端实现是**等待部署的本地补丁**。身份端点不可用时应拒绝启用周期性任务，不要根据密钥前缀、套餐、本地标记或缓存结果推断权限。SDK 不保存凭据、不打开浏览器、不安排周期性查询。

## API 覆盖

| Capability | Go API |
| --- | --- |
| Google Search, Images, News, Shopping, Videos, Places | `client.Google.Search/Images/News/Shopping/Videos/Places` |
| YouTube Search, extended Search, Metadata, Transcript, Captions | `client.YouTube.Search/SearchExtended/Metadata/Transcript/Captions` |
| License labels, Channel, Suggestions, Download | `client.YouTube.LicenseCheck/Channel/Suggestions/Download` |
| Realtime (14 sources), background and batch (all 15) | `Queries.Realtime/Submit/Batch`; `*Async` source helpers |
| Standard jobs | `Jobs.Get/Wait/Result` |
| Bulk jobs, uploads, lifecycle, listing, results | `Bulk.Create/Upload/UploadFile/Start/Get/List/IterJobs/Update/Pause/Resume/Cancel/Wait/Manifest/Part/IterRows/Rows` |
| Key access, verified identity, balance, prices, limits, usage | `Account.Access/Identity/Balance/Pricing/Limits/Usage` |
| Public discovery | `Public.Sources/Plans/Health/Status` |
| Onboarding protocol 2 | `Onboarding.Capabilities/Register/TemporaryStatus/Identity/Status/CreateClaim/Handoff/Acknowledge` |

参见 [API 参考](docs/api-reference.md)、[数据源参考](docs/sources.md)、[SDK 对齐说明](docs/parity.md)及[可运行示例目录](examples/README.md)。

## 错误、超时与 HTTP

使用 `errors.As` 识别 `APIError`、`ValidationError`、`SubmissionUnknownError`、`PollingTimeoutError`、`PollingInterruptedError`、`JobPausedError`、`JobFailedError` 和 `ProtocolError`；使用 `errors.Is` 检查上下文取消/超时及 `ErrResponseTooLarge`。如果格式异常（包括字段类型错误）的响应中仍有有效任务 ID，`SubmissionUnknownError.JobIDs` 会保留这些 ID 供核对，但不代表整个请求已成功接受。`APIError` 包含脱敏消息与代码、HTTP 状态、请求 ID 和 Retry-After。重试策略必须由调用方明确决定，不能盲目重试提交或付费实时查询。

Client 可复用，支持并发。每个操作接受 `context.Context`。默认单次请求超时 120 秒；普通任务轮询 5 分钟、间隔 2 秒；Bulk 轮询 1 小时、间隔 5 秒；上传请求预算为 15 分钟。调用方上下文及自定义 `HTTPClient.Timeout` 可以缩短这些时间。可注入 HTTP 客户端或传输层进行代理和监控；SDK 使用副本并禁用 Cookie 及自动重定向。不要让自定义传输层为签名存储请求额外添加凭据。

正式端点为 `https://data.flashdata.dev` 与 `https://api.flashdata.dev`，管理方法自动追加 `/v2`。必须使用 HTTPS，仅环回测试允许 HTTP。结果存储来源通过已认证的账户元数据发现，严格匹配 origin，访问存储时不携带密钥或 Cookie。显式设置 `ResultOrigin` 代表调用方自行指定信任来源。

存储错误会对完整签名 URL 及其中单独回显的凭据进行脱敏。Client、接入凭据、存储配置和签名上传模型的常规 Go 格式化输出也会脱敏；JSON 序列化会保留请求所需的真实凭据，不能直接记录到日志中。

## 开发与发布

```bash
go mod download
go test ./...
go vet ./...
# 支持 race detector 且已安装 C 工具链时：
go test -race ./...
# 可选：更新生成类型，Python 3.10+ 仅为开发依赖：
python scripts/generate.py
gofmt -w types_generated.go sources_generated.go
```

测试使用本地模拟 HTTP 服务和脱敏数据，不调用真实密钥、不产生付费请求、不注册试用、不写入真实云存储；同时离线编译运行示例和两份 README 程序。本地还检查 Go 模块打包与干净消费项目。[发布说明](docs/release.md) 明确后续发布边界，[复查改进](docs/review.md)和[验证记录](docs/validation.md) 列出实际执行的检查。

[贡献指南](CONTRIBUTING.md) · [安全说明](SECURITY.md) · [更新记录](CHANGELOG.md) · [MIT 许可证](LICENSE)

[联合发布检查](docs/release-readiness.md) · [第三方声明](THIRD_PARTY_NOTICES.md)
