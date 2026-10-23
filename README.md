# FlashData Go

[简体中文](README.zh-CN.md) · [FlashData](https://flashdata.dev) · [API docs](https://flashdata.dev/docs) · [Go reference](https://pkg.go.dev/github.com/flashdatadev/flashdata-go@v1.1.0) · [Pricing](https://flashdata.dev/pricing)

**Add Google search and YouTube data to your Go application.** [FlashData](https://flashdata.dev) provides structured search results, video metadata, existing transcripts and cloud media delivery through one API. Use this SDK to give an AI assistant web context, research videos, or build a data pipeline that processes many video IDs.

Use it in Go services and workers, or use the [FlashData CLI](https://github.com/flashdatadev/flashdata-cli) for the same workflows from your terminal.

## Choose a workflow

| What you want to build | Start here |
| --- | --- |
| Web context and source links for an AI assistant | [Google search](#first-search-in-a-new-project) |
| Industry research or news monitoring | [News search](#search-recent-news) |
| Video discovery for a topic or learning resource | [YouTube search](#find-youtube-videos-about-a-topic) |
| Video analytics and catalog enrichment | [Video metadata](#get-a-videos-metadata) |
| Text for video summaries, search or retrieval | [Existing transcripts](#get-a-transcript-for-summarization-or-analysis) |
| A dataset from a list of video IDs | [Bulk metadata export](#build-a-video-dataset) |
| Audio or video files delivered to your own cloud bucket | [Bulk media download](#complete-bulk-download-task) |
| Continue an interrupted bulk job | [Recovery](#recover-an-already-started-task) |

The examples collect data; your application provides summarization, indexing, scheduling
and AI model calls.

## Install

Version 1.1.0 includes JSON, plain-text and Markdown response methods. Existing
typed JSON methods keep their defaults.

Requires **Go 1.25+**. From your application's Go module:

```bash
go get github.com/flashdatadev/flashdata-go@v1.1.0
```

## First search in a new project

Start a small application:

```bash
mkdir flashdata-demo
cd flashdata-demo
go mod init example.com/flashdata-demo
go get github.com/flashdatadev/flashdata-go@v1.1.0
```

Save this as `main.go`:

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

Get a key from your [FlashData account](https://flashdata.dev), set it in your shell and run the application:

```bash
export FLASHDATA_API_KEY="YOUR_API_KEY"
go run .
```

On Windows PowerShell:

```powershell
$env:FLASHDATA_API_KEY="YOUR_API_KEY"
go run .
```

The program prints structured Google search results as JSON, including page titles, links and snippets. Queries and jobs use your account's credits; see [pricing](https://flashdata.dev/pricing).

Pass `APIKey` explicitly. Here your application reads `FLASHDATA_API_KEY`; the SDK does not read environment variables or `.env` files on its own.

## JSON, plain text and Markdown

JSON remains the default. Use `Google.SearchWithFormat`,
`YouTube.TranscriptWithFormat` or `Jobs.ResultWithFormat` with
`ContentOptions{Accept: AcceptText}` for plain text, and add
`RespondWith: RespondMarkdown` for Markdown content. Read `response.Text` for text
or `response.JSON` for JSON; `response.DecodeJSON(&model)` decodes JSON.
Constants and types belong to the `flashdata` package.

Submission, batch and job-status `WithFormat` methods take `ResponseOptions`,
which selects JSON or text. Markdown applies to realtime queries and completed
results. Existing typed methods, polling and pagination keep returning JSON.
See the [response-format guide](docs/response-formats.md) and
[runnable example](examples/response_formats/main.go).

## Common tasks

The snippets below reuse `client` and `ctx` from `main.go`. **Replace the `result, err := ...` query and its two error/output checks with one snippet**, then run `go run .` again.

### Search recent news

```go
result, err := client.Google.News(ctx, flashdata.GoogleNewsInput{Query: "battery recycling"})
if err != nil {
	log.Fatal(err)
}
if err = json.NewEncoder(os.Stdout).Encode(result); err != nil {
	log.Fatal(err)
}
```

Use `client.Google.Images`, `Shopping`, `Videos` or `Places` for other Google sources. All accept the corresponding typed input; see [source parameters](docs/sources.md).

### Find YouTube videos about a topic

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

Use the returned video IDs in the following calls.

### Get a video's metadata

```go
result, err := client.YouTube.Metadata(ctx, flashdata.YouTubeMetadataInput{VideoID: "dQw4w9WgXcQ"})
if err != nil {
	log.Fatal(err)
}
if err = json.NewEncoder(os.Stdout).Encode(result); err != nil {
	log.Fatal(err)
}
```

The response contains video data such as title, channel, duration, view count and like count.

### Get a transcript for summarization or analysis

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

Replace the example video ID with your own. Transcript availability depends on existing language tracks; this does not transcribe arbitrary audio. `client.YouTube.Captions` requests an existing manual caption track; use `Auto: flashdata.Ptr(false)` with that source.

Results preserve provider JSON, including fields added by the service. For typed access, `flashdata.DecodeResults[flashdata.GoogleSearchPayload](result)` returns the Google search payloads; fields may be absent or null. See [response types](docs/api-reference.md).

### Read an uploaded caption track

Use the same client and context to request an existing uploaded English track:

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

## Run the included examples

The quickstart above works in your own application. To run the repository's standalone examples, clone the SDK into a separate directory and run commands from the clone root, using the same API key environment variable:

```bash
git clone --depth 1 https://github.com/flashdatadev/flashdata-go.git
cd flashdata-go
go mod download
```

Each [example](examples/README.md) is a complete program. For example, `go run ./examples/youtube_metadata` fetches one video's metadata, and `go run ./examples/youtube_transcript` fetches an English transcript.

### Process one query in the background

The background example submits a metadata job, saves its ID before waiting, then fetches the result:

```bash
go run ./examples/background
# After an interruption, use the ID saved in background.job.json:
go run ./examples/background -job job_YOUR_ID
```

For your own service, use a source's `*Async` helper, persist the accepted job ID, then call `client.Jobs.Wait` and `client.Jobs.Result`. A local timeout does not cancel accepted server work. See the [background example](examples/background/main.go) for the complete implementation. Mixed batches and paginated search have [examples](examples/README.md) too.

## Build a video dataset

In the SDK clone, create `ids.txt` with one YouTube video ID per line:

```text
dQw4w9WgXcQ
jNQXAC9IVRw
```

Collect their metadata into a JSONL dataset:

```bash
go run ./examples/bulk -source youtube_metadata -ids ids.txt -out videos.jsonl
```

The program creates the bulk job, uploads the IDs, starts processing, waits and writes one result row per input item. Use `youtube_transcript`, `youtube_captions` or `youtube_trainability` for other bulk datasets. See [bulk options](docs/bulk.md).

## Complete bulk download task

Deliver MP3 audio for all videos in `ids.txt` **to your cloud bucket**. The local JSONL output contains per-video delivery receipts, not audio bytes.

**1. Configure storage.** Create `storage.json` in a private directory outside source control, replacing every placeholder with your own S3 destination and credentials:

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

On Linux/macOS, restrict the file:

```bash
chmod 600 storage.json
```

On Windows, use a directory and file ACL restricted to your user, SYSTEM and optionally Administrators. A Unix file mode alone does not protect Windows credentials. See [cloud destination options](docs/bulk.md) for S3-compatible storage, OSS, GCS, Azure, COS, BOS and TOS.

**2. Start and export.** From the SDK clone, point `-storage` to that private file (the command below assumes `storage.json` is in the current directory):

```bash
go run ./examples/bulk -source youtube_download -ids ids.txt -storage storage.json -out audio-receipts.jsonl
```

This complete example validates the input, reserves output paths, creates the task, saves its ID to `audio-receipts.jsonl.job.json`, uploads, starts, waits and exports receipts. Inspect each row's status; failed items are retained and cause a nonzero exit. Existing output and recovery files are never overwritten.

### Deliver MP4 video instead

Save these non-secret settings as `video-options.json`:

```json
{"download_type":"audio_video","video_quality":"<=720","output_format":"mp4"}
```

```bash
go run ./examples/bulk -source youtube_download -ids ids.txt -storage storage.json -options video-options.json -out video-receipts.jsonl
```

This command uses the current example on `main`, fetched by the clone command above.
The v1.1.0 SDK supports both media option sets.

### Recover an already started task

Read the ID from `audio-receipts.jsonl.job.json` or `video-receipts.jsonl.job.json`, replace `bulk_YOUR_ID`, and choose a new output path:

```bash
go run ./examples/bulk -job bulk_YOUR_ID -out recovered-receipts.jsonl
```

Recovery reads the original job without creating another. If your process stopped before upload/start, the ID-only example needs additional handling; see [bulk recovery](docs/bulk.md). The [CLI](https://github.com/flashdatadev/flashdata-cli) includes a protected journal for that workflow.

<details>
<summary>Use the bulk API directly in your own application</summary>

This standalone program shows create → upload → start → wait → stream. Run it with `ids.txt`, private `storage.json` and an explicit API key as above. The [runnable bulk example](examples/bulk/main.go) adds input/output preflight checks and recovery flags; use it as the starting point for an application integration. Persist the full creation response in secret storage if you need recovery before upload.

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

## Available APIs

| Task | SDK methods |
| --- | --- |
| Google search, images, news, shopping, videos, places | `client.Google.Search / Images / News / Shopping / Videos / Places` |
| YouTube discovery, metadata, transcripts, captions, channel data, suggestions, license labels | `client.YouTube.Search / SearchExtended / Metadata / Transcript / Captions / Channel / Suggestions / LicenseCheck` |
| Audio/video delivery to customer storage | `client.YouTube.Download` (background), `client.Bulk` (multiple videos) |
| Background jobs and mixed batches | Source `*Async` helpers, `client.Queries.Batch`, `client.Jobs` |
| Bulk datasets, lifecycle and streamed JSONL | `client.Bulk` |
| Balance, pricing, usage, account limits | `client.Account` |
| Source discovery, plans and service health | `client.Public` |

See the [API reference](docs/api-reference.md), [all source parameters](docs/sources.md) and [runnable examples](examples/README.md). Optional agent registration and browser claiming are covered in [onboarding](docs/onboarding.md); availability depends on the server deployment.

See the [SDK and CLI comparison](docs/sdk-parity.md) for API coverage, examples and bulk recovery across languages.

## Using the SDK in a service

Reuse a client across requests and pass a `context.Context` to every call. Optional parameters use pointers: `flashdata.Ptr(false)` sends an explicit false; `nil` leaves the value unset. Handle errors before reading results.

The SDK does not automatically retry paid queries or job creation. Persist accepted job IDs before waiting, and reconcile an uncertain submission before creating replacement work. See [errors, timeouts and HTTP configuration](docs/api-reference.md#errors-timeouts-and-http-transport) and [bulk recovery](docs/bulk.md).

[API documentation](https://flashdata.dev/docs) · [pkg.go.dev](https://pkg.go.dev/github.com/flashdatadev/flashdata-go@v1.1.0) · [Contributing and tests](CONTRIBUTING.md) · [Release notes](docs/release-notes-1.1.0.md) · [Security](SECURITY.md) · [MIT license](LICENSE)
