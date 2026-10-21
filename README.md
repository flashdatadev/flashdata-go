# FlashData Go

[简体中文](README.zh-CN.md) · [FlashData](https://flashdata.dev) · [API docs](https://flashdata.dev/docs) · [Pricing](https://flashdata.dev/pricing)

The Go SDK for [FlashData](https://flashdata.dev): structured Google search and YouTube data through one API. Retrieve search results, images, news, shopping, videos and places; collect YouTube metadata, transcripts, captions, channel data, suggestions and license labels; deliver media downloads to your own cloud storage. Built for Go backend teams, search and AI applications, researchers, and data pipelines. The [FlashData CLI](https://github.com/flashdatadev/flashdata-cli) is built on this SDK.

**Version 1.0.0.** The `v1.0.0` source tag identifies this version; GitHub Release assets are being prepared as an unpublished draft. The accompanying backend identity patch has not been deployed by this project. Go 1.25 or newer is required. The existing 24 developer operations and 15 sources match the Python and TypeScript SDKs. Verified key identity and onboarding are additional Go capabilities; see [contract status](docs/api-contract.md).

## Installation

From your application's Go module:

```bash
go get github.com/flashdatadev/flashdata-go@v1.0.0
```

Commit the resulting `go.mod` and `go.sum` to pin SDK v1.0.0 and its verified checksums. Go installs from the public source tag independently of GitHub Release publication. See the [v1.0.0 release notes](docs/release-notes-1.0.0.md).

To run the shipped examples or develop the SDK from source:

```bash
git clone https://github.com/flashdatadev/flashdata-go.git
cd flashdata-go
go mod download
```

The SDK requires Go 1.25 or newer. A local `go.work` workspace can be used for SDK development; installed consumers do not need a sibling checkout.

## API key and first query

Obtain a key from your [FlashData account](https://flashdata.dev). Pass it explicitly to `flashdata.New`; the SDK never reads environment variables or `.env` files itself. Keep credentials out of source control.

```bash
export FLASHDATA_API_KEY="YOUR_API_KEY"
go run ./examples/google_search
```

PowerShell:

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

Optional parameters are pointers. `flashdata.Ptr(false)` sends an explicit false; `nil` leaves a field unset so the service chooses its default. `Transcript` and `Captions` retain their own source IDs; the examples explicitly set `auto:true` and `auto:false`. Download is background-only and requires customer storage.

Results retain provider JSON as `json.RawMessage`, preserving extra fields and large integers. Use `flashdata.DecodeResults[flashdata.GoogleSearchPayload](result)` for optional typed payload hints, or decode into your own structs. Provider fields can be absent or null. `DecodeResults[map[string]any]` uses `json.Number` instead of rounding large numbers.

## Background, batch and pagination

Run `go run ./examples/background` to submit one YouTube metadata job, save `background.job.json`, wait and fetch the result. The example reserves its recovery file before submission and refuses to overwrite it; use `-record another.job.json` only for intentional new work. Recover an accepted job with `go run ./examples/background -job job_YOUR_ID`; this does not submit a replacement. `*Async` helpers return `*AsyncSubmission`, while realtime helpers return `*DataQueryResponse`. Set `Query.CallbackURL` for a background callback; only public HTTPS endpoints on port 443 are accepted.

`go run ./examples/batch` submits a mixed Google/YouTube batch. `go run ./examples/pagination` requests **two separately billed** search pages. Batches are subject to the account's current admission limits. These examples incur normal API charges when pointed at FlashData; public discovery and account reads are free.

No operation automatically retries a submission. `SubmissionUnknownError` means a mutation may have reached the server: preserve state and reconcile before retrying. A returned job ID is your recovery handle. When a malformed response (including a field type error) still contains valid IDs, `SubmissionUnknownError.JobIDs` retains them for reconciliation; it does not confirm complete acceptance. Poll cancellation or timeout stops local waiting; accepted server work continues. A client request timeout remains distinct from the overall polling timeout.

## Complete bulk download task

Bulk supports metadata, transcripts, captions, license labels and downloads. Create → upload IDs → start → wait → stream receipts. For downloads, media files go directly to your customer-owned bucket; NDJSON contains per-item delivery outcomes, not the media bytes.

Create `ids.txt`:

```text
dQw4w9WgXcQ
jNQXAC9IVRw
```

Create a private `storage.json`, replacing every placeholder with a real destination you control:

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

The SDK supports S3, S3-compatible, OSS, GCS, Azure, COS, BOS and TOS destination shapes. Provider credentials, permissions, endpoints and conditional format rules are ultimately validated by the server. GCS uses `service_account`; Azure uses `account_name` and a write-capable `sas_token`. See [storage and recovery](docs/bulk.md). Do not commit this file.

The full SDK sequence:

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

For a ready-to-run version with input/output preflight checks and ID recovery:

```bash
go run ./examples/bulk -source youtube_download -ids ids.txt -storage storage.json -out receipts.jsonl
# Recover an already started task, using a fresh output path:
go run ./examples/bulk -job bulk_YOUR_ID -out recovered.jsonl
# Other data sources use the same flow:
go run ./examples/bulk -source youtube_metadata -ids ids.txt -out metadata.jsonl
```

The example saves only the accepted ID in `OUTPUT.job.json`. It reserves recovery paths before creation, syncs accepted IDs before uploading, refuses to overwrite exports or records, and uploads a snapshot of the validated input. Result rows are written before reporting failed/invalid items; inspect every status even when the bulk job itself completed. An `awaiting_upload` task needs its original upload and an explicit start; a paused task needs its cause fixed with `Bulk.Update` and an explicit resume. Use `-job ID -start` or `-job ID -resume` for those transitions, never create replacement work automatically. An expired upload cannot be recovered by replaying job creation; consult the API state/support.

The examples keep signed upload capabilities in memory, so their ID records alone cannot recover an interrupted pre-upload process. Applications must persist the complete creation response in a secure store before uploading if that recovery is required; the CLI provides this journal. A reserved empty record after an uncertain request must be reconciled before creating new work. These ID files contain no credentials; use an OS-protected location for storage JSON. File mode `0600` alone does not create a secure Windows credential vault.

Input is 1 byte–2 GiB, one video ID per line. Upload URLs last 15 minutes. Internal results are retained for 24 hours; customer storage follows your bucket policy. Defaults bound responses/decompressed parts to 64 MiB and rows to 16 MiB. `Rows` and `IterRows` stream and stop on early loop break; `Part` returns compressed bytes. Tune limits deliberately for larger exports.

## Onboarding and verified ownership

Existing account holders supply `Options.APIKey` directly. Optional agent onboarding uses protocol 2, cryptographic proofs and the existing browser claim flow. It is feature-gated; a disabled deployment can return 404. Public capability discovery never creates a trial.

`go run ./examples/onboarding` reads capabilities. See the [complete onboarding integration guide](docs/onboarding.md) for durable registration, claim approval, handoff recovery, fresh-process verification and ACK. All eight machine-side operations are available. Browser confirmation/approval/revocation remain in the verified FlashData browser session.

`Account.Identity` reports authoritative ownership and `recurringAllowed`; its backend implementation is included as a **local patch awaiting deployment**. Fail closed on an unavailable identity endpoint. Never authorize recurring work from a key prefix, plan name, saved flag or cached result. The SDK does not store credentials, open browsers or schedule queries.

## API coverage

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

See the [API reference](docs/api-reference.md), [source reference](docs/sources.md), [SDK parity](docs/parity.md), and [runnable example catalogue](examples/README.md).

## Errors, timeouts and HTTP transport

Use `errors.As` for `APIError`, `ValidationError`, `SubmissionUnknownError`, `PollingTimeoutError`, `PollingInterruptedError`, `JobPausedError`, `JobFailedError` and `ProtocolError`; use `errors.Is` for context cancellation/deadlines and `ErrResponseTooLarge`. `APIError` includes redacted message/code, status, request ID and Retry-After. Decide any retry policy explicitly; do not blindly retry mutations or paid realtime calls.

Clients are reusable and safe for concurrent calls. Every operation takes `context.Context`. Default request timeout is 120 seconds; job polling is 5 minutes/2-second intervals and bulk polling 1 hour/5-second intervals. Uploads have a 15-minute request budget. Caller deadlines and an injected `HTTPClient.Timeout` can shorten those budgets. Inject an HTTP client/transport for proxies or instrumentation; cookies and automatic redirects are disabled on the client's copy. Avoid credential-adding custom transports on signed storage requests.

Use HTTPS origins; HTTP is permitted only on loopback for tests. Production origins are `https://data.flashdata.dev` and `https://api.flashdata.dev`; management methods append `/v2`. Signed result origins are discovered through authenticated account metadata, validated exactly, and contacted without keys or cookies. Caller-supplied `ResultOrigin` is an explicit trust override. Storage errors redact bare signed-URL credentials as well as complete URLs. Client, onboarding credential, storage and signed-upload models redact normal Go diagnostic formatting; JSON serialization intentionally retains wire credentials and must never be logged.

## Development and release

```bash
go mod download
go test ./...
go vet ./...
# On a machine with a supported race detector and C toolchain:
go test -race ./...
# Optional generated type refresh (Python 3.10+ is a development tool only):
python scripts/generate.py
gofmt -w types_generated.go sources_generated.go
```

Tests use local mock HTTP services and sanitized fixtures, with no live keys, paid queries, trials or cloud writes. They also build and run the examples and both README programs offline. Go module packaging and a clean consumer are checked locally; [release instructions](docs/release.md) describe validation and the later tagged-release boundary. See [review fixes](docs/review.md) and [validation](docs/validation.md) for the exact changes and checks in this checkout.

[Joint release check](docs/release-readiness.md) · [Third-party notices](THIRD_PARTY_NOTICES.md) · [Contributing](CONTRIBUTING.md) · [Security](SECURITY.md) · [Changelog](CHANGELOG.md) · [MIT license](LICENSE)
