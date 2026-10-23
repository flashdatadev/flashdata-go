# FlashData Go

[English](README.md) · [FlashData 官网](https://flashdata.dev) · [API 文档](https://flashdata.dev/docs) · [Go 参考](https://pkg.go.dev/github.com/flashdatadev/flashdata-go@v1.1.0) · [价格](https://flashdata.dev/pricing)

**为 Go 应用接入 Google 搜索和 YouTube 数据。** [FlashData](https://flashdata.dev) 通过统一 API 提供结构化搜索结果、视频元数据、已有字幕和云存储媒体交付。使用本 SDK 为 AI 助手提供网页上下文、研究视频，或构建处理大量视频 ID 的数据流水线。

适用于 Go 服务和后台工作进程。需要在终端完成相同工作流时，可以使用 [FlashData CLI](https://github.com/flashdatadev/flashdata-cli)。

## 按使用场景选择工作流

| 想要构建的功能 | 从这里开始 |
| --- | --- |
| 为 AI 助手提供网页上下文和来源链接 | [Google 搜索](#在新项目中完成第一次搜索) |
| 行业研究或新闻监控 | [新闻搜索](#搜索最新新闻) |
| 按主题发现视频或学习资源 | [YouTube 搜索](#按主题查找-youtube-视频) |
| 视频分析和内容目录补全 | [视频元数据](#获取视频元数据) |
| 为视频摘要、搜索或检索获取文本 | [已有字幕文本](#获取字幕用于摘要或分析) |
| 根据一组视频 ID 构建数据集 | [批量元数据导出](#构建视频数据集) |
| 将音频或视频交付到自己的云存储桶 | [批量媒体下载](#完整批量下载任务) |
| 继续中断的批量任务 | [恢复任务](#恢复已启动的任务) |

以下示例负责获取数据；摘要、检索索引、调度和 AI 模型调用由应用实现。

## 安装

1.1.0 提供 JSON、纯文本和 Markdown 响应方法，原有类型化 JSON 方法的默认行为保持不变。

需要 **Go 1.25+**。在应用的 Go 模块中运行：

```bash
go get github.com/flashdatadev/flashdata-go@v1.1.0
```

## 在新项目中完成第一次搜索

创建一个小型应用：

```bash
mkdir flashdata-demo
cd flashdata-demo
go mod init example.com/flashdata-demo
go get github.com/flashdatadev/flashdata-go@v1.1.0
```

将以下内容保存为 `main.go`：

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
	client, err := flashdata.New(flashdata.Options{APIKey: os.Getenv("FLASHDATA_API_KEY")})
	if err != nil {
		log.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	result, err := client.Google.Search(ctx, flashdata.GoogleSearchInput{
		Query:  "battery recycling startups",
		Params: &flashdata.GoogleSearchParams{Num: flashdata.Ptr(int64(5))},
	})
	if err != nil {
		log.Fatal(err)
	}
	if err = json.NewEncoder(os.Stdout).Encode(result); err != nil {
		log.Fatal(err)
	}
}
```

从 [FlashData 账户](https://flashdata.dev) 获取密钥，在终端设置后运行应用：

```bash
export FLASHDATA_API_KEY="YOUR_API_KEY"
go run .
```

Windows PowerShell：

```powershell
$env:FLASHDATA_API_KEY="YOUR_API_KEY"
go run .
```

程序会以 JSON 输出结构化 Google 搜索结果，包括网页标题、链接和摘要。查询和任务会消耗账户额度，参见[价格](https://flashdata.dev/pricing)。

请显式传入 `APIKey`。此处由应用读取 `FLASHDATA_API_KEY`；SDK 本身不会读取环境变量或自动加载 `.env` 文件。

## JSON、纯文本和 Markdown

JSON 仍为默认格式。使用 `Google.SearchWithFormat`、
`YouTube.TranscriptWithFormat` 或 `Jobs.ResultWithFormat`，传入
`ContentOptions{Accept: AcceptText}` 可获取纯文本；增加
`RespondWith: RespondMarkdown` 可选择 Markdown 内容。纯文本读取 `response.Text`，
JSON 读取 `response.JSON`，也可使用 `response.DecodeJSON(&model)` 解码。
类型和常量均属于 `flashdata` 包。

提交、批量提交和任务状态的 `WithFormat` 方法使用 `ResponseOptions` 选择 JSON 或
纯文本。Markdown 仅适用于实时查询和已完成结果。现有类型化方法、轮询和分页仍返回
JSON。参见[响应格式指南](docs/response-formats.md)和
[可运行示例](examples/response_formats/main.go)。

## 常用功能

下面的片段复用 `main.go` 中的 `client` 和 `ctx`。**将 `result, err := ...` 查询以及后面的两个错误处理/输出检查替换为任一片段**，然后重新运行 `go run .`。

### 搜索最新新闻

```go
result, err := client.Google.News(ctx, flashdata.GoogleNewsInput{Query: "battery recycling"})
if err != nil {
	log.Fatal(err)
}
if err = json.NewEncoder(os.Stdout).Encode(result); err != nil {
	log.Fatal(err)
}
```

其他 Google 来源可使用 `client.Google.Images`、`Shopping`、`Videos` 或 `Places`，并传入各自的类型化输入，详见[来源参数](docs/sources.md)。

### 按主题查找 YouTube 视频

```go
result, err := client.YouTube.Search(ctx, flashdata.YouTubeSearchInput{
	Query:  "Go programming tutorial",
	Params: &flashdata.YouTubeSearchParams{MaxResults: flashdata.Ptr(int64(5))},
})
if err != nil {
	log.Fatal(err)
}
if err = json.NewEncoder(os.Stdout).Encode(result); err != nil {
	log.Fatal(err)
}
```

后续调用可以使用结果返回的视频 ID。

### 获取视频元数据

```go
result, err := client.YouTube.Metadata(ctx, flashdata.YouTubeMetadataInput{VideoID: "dQw4w9WgXcQ"})
if err != nil {
	log.Fatal(err)
}
if err = json.NewEncoder(os.Stdout).Encode(result); err != nil {
	log.Fatal(err)
}
```

响应包含视频标题、频道、时长、播放量、点赞数等数据。

### 获取字幕，用于摘要或分析

```go
result, err := client.YouTube.Transcript(ctx, flashdata.YouTubeTranscriptInput{
	VideoID: "dQw4w9WgXcQ",
	Params:  &flashdata.YouTubeTranscriptParams{Language: flashdata.Ptr("en"), Auto: flashdata.Ptr(true)},
})
if err != nil {
	log.Fatal(err)
}
if err = json.NewEncoder(os.Stdout).Encode(result); err != nil {
	log.Fatal(err)
}
```

请将示例视频 ID 替换为实际需要的 ID。字幕能否获取取决于已有的语言轨道；此功能不会将任意音频转写为文字。`client.YouTube.Captions` 请求已有的人工字幕轨道，请为该来源设置 `Auto: flashdata.Ptr(false)`。

结果保留服务商原始 JSON，包括服务后续新增的字段。如需类型化访问，可用 `flashdata.DecodeResults[flashdata.GoogleSearchPayload](result)` 获取 Google 搜索载荷；字段可能缺失或为 null。详见[响应类型](docs/api-reference.md)。

### 读取人工上传的字幕轨道

使用与上面相同的客户端和上下文，请求已有的人工英文字幕：

```go
result, err := client.YouTube.Captions(ctx, flashdata.YouTubeCaptionsInput{
	VideoID: "dQw4w9WgXcQ",
	Params: &flashdata.YouTubeCaptionsParams{Language: flashdata.Ptr("en"), Auto: flashdata.Ptr(false)},
})
if err != nil {
	log.Fatal(err)
}
if err = json.NewEncoder(os.Stdout).Encode(result); err != nil {
	log.Fatal(err)
}
```

## 运行仓库自带的示例

上面的快速开始适用于你自己的应用。如需运行仓库中独立的示例程序，请将 SDK 克隆到另一个目录，并在克隆目录根路径运行命令，继续使用相同的 API 密钥环境变量：

```bash
git clone --depth 1 https://github.com/flashdatadev/flashdata-go.git
cd flashdata-go
go mod download
```

每个[示例](examples/README.md)都是完整程序。例如，`go run ./examples/youtube_metadata` 获取单个视频元数据，`go run ./examples/youtube_transcript` 获取英文字幕。

### 在后台处理单个查询

后台示例会提交元数据任务，在等待之前保存任务 ID，然后获取结果：

```bash
go run ./examples/background
# After an interruption, use the ID saved in background.job.json:
go run ./examples/background -job job_YOUR_ID
```

在自己的服务中，可调用来源的 `*Async` 方法，持久化已受理的任务 ID，再调用 `client.Jobs.Wait` 和 `client.Jobs.Result`。本地超时不会取消已受理的服务端任务。完整实现见[后台任务示例](examples/background/main.go)。混合批次和搜索分页也有[示例](examples/README.md)。

## 构建视频数据集

在 SDK 克隆目录中创建 `ids.txt`，每行填写一个 YouTube 视频 ID：

```text
dQw4w9WgXcQ
jNQXAC9IVRw
```

将这些视频的元数据收集为 JSONL 数据集：

```bash
go run ./examples/bulk -source youtube_metadata -ids ids.txt -out videos.jsonl
```

程序会创建批量任务、上传 ID、启动处理、等待完成，并为每个输入条目写入一行结果。使用 `youtube_transcript`、`youtube_captions` 或 `youtube_trainability` 可构建其他批量数据集。详见[批量选项](docs/bulk.md)。

## 完整批量下载任务

将 `ids.txt` 中所有视频的 MP3 音频交付**到你的云存储桶**。本地 JSONL 输出包含逐视频交付回执，而非音频字节。

**1. 配置存储。** 在源码管理之外的私有目录创建 `storage.json`，将所有占位符替换为自己的 S3 目标和凭据：

```json
{
  "storage_type": "s3",
  "storage_url": "YOUR_BUCKET_NAME",
  "region": "us-east-1",
  "access_key": "YOUR_STORAGE_ACCESS_KEY",
  "secret_key": "YOUR_STORAGE_SECRET_KEY",
  "object_prefix": "flashdata/audio"
}
```

Linux/macOS 上，限制文件权限：

```bash
chmod 600 storage.json
```

Windows 上，请限制目录和文件 ACL，仅允许当前用户、SYSTEM 以及可选的 Administrators 访问。Unix 文件模式本身不能保护 Windows 凭据。S3 兼容存储、OSS、GCS、Azure、COS、BOS 和 TOS 的配置详见[云存储目标选项](docs/bulk.md)。

**2. 启动并导出。** 在 SDK 克隆目录中，将 `-storage` 指向该私有文件（以下命令假定 `storage.json` 位于当前目录）：

```bash
go run ./examples/bulk -source youtube_download -ids ids.txt -storage storage.json -out audio-receipts.jsonl
```

此完整示例会验证输入、预留输出路径、创建任务，将 ID 保存到 `audio-receipts.jsonl.job.json`，然后上传、启动、等待和导出回执。请检查每行状态；失败条目会保留，并使程序以非零状态退出。已有输出文件和恢复记录不会被覆盖。

### 改为交付 MP4 视频

将以下非秘密配置保存为 `video-options.json`：

```json
{"download_type":"audio_video","video_quality":"<=720","output_format":"mp4"}
```

```bash
go run ./examples/bulk -source youtube_download -ids ids.txt -storage storage.json -options video-options.json -out video-receipts.jsonl
```

此命令使用 `main` 分支中的最新示例；上述克隆命令会获取这些示例。
v1.1.0 SDK 同时支持音频和视频选项。

### 恢复已启动的任务

从 `audio-receipts.jsonl.job.json` 或 `video-receipts.jsonl.job.json` 读取 ID，替换 `bulk_YOUR_ID`，并选择一个新输出路径：

```bash
go run ./examples/bulk -job bulk_YOUR_ID -out recovered-receipts.jsonl
```

恢复操作读取原任务，不会创建新任务。如果进程在上传或启动前退出，仅保存 ID 的示例需要额外处理，详见[批量恢复](docs/bulk.md)。[CLI](https://github.com/flashdatadev/flashdata-cli) 为该工作流提供受保护的持久化日志。

<details>
<summary>在自己的应用中直接使用批量 API</summary>

这个独立程序展示创建 → 上传 → 启动 → 等待 → 流式读取。按前述方式准备 `ids.txt`、私有 `storage.json` 以及显式 API 密钥后即可运行。[可运行的批量示例](examples/bulk/main.go) 还提供输入/输出预检查和恢复参数，可作为应用集成的起点。如需在上传前恢复，请将完整创建响应保存在密钥存储中。

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
	client, err := flashdata.New(flashdata.Options{APIKey: os.Getenv("FLASHDATA_API_KEY")})
	if err != nil {
		log.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 65*time.Minute)
	defer cancel()

	// storage.json contains your real customer-owned destination configuration.
	storageJSON, err := os.ReadFile("storage.json")
	if err != nil {
		log.Fatal("could not read storage.json")
	}
	var storage flashdata.Storage
	if err = json.Unmarshal(storageJSON, &storage); err != nil {
		log.Fatal("invalid storage JSON")
	}

	// Reserve both paths before creating work; never overwrite a saved job.
	file, err := os.OpenFile("download-results.jsonl", os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		log.Fatal("choose a new result output path")
	}
	defer file.Close()
	recovery, err := os.OpenFile("download.job.json", os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		log.Fatal("recover the saved job or choose a new recovery path")
	}
	defer recovery.Close()

	// Each line of ids.txt is one 11-character YouTube video ID.
	// Validate the input and output paths before creating a real job; the
	// runnable examples/bulk program performs these preflight checks.
	created, err := client.Bulk.Create(ctx, flashdata.CreateBulkJobRequest{
		Source: flashdata.SourceYouTubeDownload,
		Options: flashdata.YouTubeDownloadParams{
			DownloadType: flashdata.Ptr("audio"), OutputFormat: flashdata.Ptr("mp3"),
			Storage: storage,
		},
	})
	if err != nil {
		log.Fatal(err)
	}
	id := created.Job.ID
	log.Printf("Accepted %s; save this ID before continuing", id)
	if err = json.NewEncoder(recovery).Encode(map[string]string{"id": id}); err == nil {
		err = recovery.Sync()
	}
	if err != nil {
		log.Fatalf("accepted %s; could not persist recovery record", id)
	}
	if err = client.Bulk.UploadFile(ctx, created, "ids.txt"); err != nil {
		log.Fatal(err)
	}
	if _, err = client.Bulk.Start(ctx, id); err != nil {
		log.Fatal(err)
	}
	if _, err = client.Bulk.Wait(ctx, id, flashdata.PollOptions{Timeout: time.Hour}); err != nil {
		log.Fatal(err)
	}

	encoder := json.NewEncoder(file)
	failed := 0
	for row, err := range client.Bulk.Rows(ctx, id) {
		if err != nil {
			log.Fatalf("partial output retained for %s: %v", id, err)
		}
		if err = encoder.Encode(row); err != nil {
			log.Fatal("could not write results")
		}
		if string(row["status"]) != `"completed"` {
			failed++
		}
	}
	if err = file.Sync(); err != nil {
		log.Fatal("could not flush results")
	}
	if failed != 0 {
		log.Fatalf("%d failed/invalid item receipts retained; inspect before retrying", failed)
	}
	log.Printf("Saved receipts for %s; media files are in your storage bucket", id)
}
```

</details>

## 可用 API

| 任务 | SDK 方法 |
| --- | --- |
| Google 搜索、图片、新闻、购物、视频、地点 | `client.Google.Search / Images / News / Shopping / Videos / Places` |
| YouTube 发现、元数据、字幕、人工字幕、频道、建议、许可标签 | `client.YouTube.Search / SearchExtended / Metadata / Transcript / Captions / Channel / Suggestions / LicenseCheck` |
| 将音视频交付到客户存储 | `client.YouTube.Download`（后台）、`client.Bulk`（多个视频） |
| 后台任务与混合批次 | 来源的 `*Async` 方法、`client.Queries.Batch`、`client.Jobs` |
| 批量数据集、任务控制和流式 JSONL | `client.Bulk` |
| 余额、定价、用量、账户限制 | `client.Account` |
| 来源发现、套餐、服务健康状态 | `client.Public` |

参阅 [API 参考](docs/api-reference.md)、[所有来源参数](docs/sources.md)及[可运行示例](examples/README.md)。可选的代理注册与浏览器认领流程见[接入指南](docs/onboarding.md)；是否可用取决于服务端部署。

参阅[各 SDK 与 CLI 对比（英文）](docs/sdk-parity.md)，比较 API 覆盖、示例和批量恢复行为。

## 在服务中使用 SDK

在请求之间复用客户端，并为每次调用传入 `context.Context`。可选参数使用指针：`flashdata.Ptr(false)` 显式发送 false；`nil` 表示不设置该值。读取结果前请处理错误。

SDK 不会自动重试付费查询或任务创建。等待之前请持久化已受理的任务 ID；提交结果不确定时，先核对状态，再决定是否创建替代任务。详见[错误、超时和 HTTP 配置](docs/api-reference.md#errors-timeouts-and-http-transport)以及[批量恢复](docs/bulk.md)。

[API 文档](https://flashdata.dev/docs) · [pkg.go.dev](https://pkg.go.dev/github.com/flashdatadev/flashdata-go@v1.1.0) · [参与贡献与测试](CONTRIBUTING.md) · [版本说明](docs/release-notes-1.1.0.md) · [安全](SECURITY.md) · [MIT 许可](LICENSE)
