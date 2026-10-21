# Runnable examples

[FlashData](https://flashdata.dev) · [API documentation](https://flashdata.dev/docs)

Run these from the repository root after `go mod download`. Supply the key explicitly through `FLASHDATA_API_KEY`; each program passes it to `flashdata.New`. All example programs accept optional `FLASHDATA_DATA_ORIGIN` and `FLASHDATA_MANAGEMENT_ORIGIN` overrides, permitting loopback fixtures. Never point an offline test at a live paid endpoint.

| Directory | Purpose |
| --- | --- |
| [account](account/main.go) | Free API-key balance read |
| [background](background/main.go) | Submit once, persist ID, wait/fetch or recover by -job |
| [batch](batch/main.go) | Mixed-source background batch |
| [bulk](bulk/main.go) | Complete bulk data/download task; -h lists flags |
| [discover](discover/main.go) | Free public source discovery |
| [google_images](google_images/main.go) | Run google_images using its typed input |
| [google_news](google_news/main.go) | Run google_news using its typed input |
| [google_places](google_places/main.go) | Run google_places using its typed input |
| [google_search](google_search/main.go) | Run google_search using its typed input |
| [google_shopping](google_shopping/main.go) | Run google_shopping using its typed input |
| [google_videos](google_videos/main.go) | Run google_videos using its typed input |
| [onboarding](onboarding/main.go) | Read capabilities/identity/status or create a browser claim; no implicit registration |
| [pagination](pagination/main.go) | Two separately billed Google Search pages |
| [youtube_captions](youtube_captions/main.go) | Run youtube_captions using its typed input |
| [youtube_channel](youtube_channel/main.go) | Run youtube_channel using its typed input |
| [youtube_download](youtube_download/main.go) | Run youtube_download using its typed input |
| [youtube_metadata](youtube_metadata/main.go) | Run youtube_metadata using its typed input |
| [youtube_search](youtube_search/main.go) | Run youtube_search using its typed input |
| [youtube_search_extended](youtube_search_extended/main.go) | Run youtube_search_extended using its typed input |
| [youtube_suggestions](youtube_suggestions/main.go) | Run youtube_suggestions using its typed input |
| [youtube_trainability](youtube_trainability/main.go) | Run youtube_trainability using its typed input |
| [youtube_transcript](youtube_transcript/main.go) | Run youtube_transcript using its typed input |

Background reserves `background.job.json` before submitting. A second invocation refuses to overwrite it; recover with `-job` or explicitly choose `-record another.job.json` for new work. Reserved empty records after an uncertain request need reconciliation.

Download needs `-storage storage.json` or `-job job_ID`. Bulk needs `-ids ids.txt` and an appropriate source; downloads additionally need `-storage`. Supply a fresh `-out` name for recovery to retain previous partial exports. `go run ./examples/bulk -job ID` does not replay input or submit replacement work. See the [complete README task](../README.md#complete-bulk-download-task) and [recovery details](../docs/bulk.md).

Onboarding registration and installation ACK require an application-owned secure durable store and fresh-process verification. The example intentionally covers the safe read/claim primitives; [the integration guide](../docs/onboarding.md) describes the complete lifecycle. Offline tests exercise all machine methods.
