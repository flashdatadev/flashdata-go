# Bulk processing and recovery

[FlashData](https://flashdata.dev) · [API documentation](https://flashdata.dev/docs)

Read the complete download task in the [English README](../README.md#complete-bulk-download-task) or [简体中文](../README.zh-CN.md). `go run ./examples/bulk -h` lists the executable example's options.

Bulk accepts `youtube_metadata`, `youtube_transcript`, `youtube_captions`, `youtube_trainability` and `youtube_download`. `CreateBulkJobRequest.Options` accepts the matching typed Params or a JSON map. Downloads require `options.storage`. Data bulk sources may optionally use `options.result_storage` to copy compressed result parts into customer storage; this is distinct from media delivery. Query-specific metadata/transcript options remain available.

## Cloud destinations

| storage_type | Required destination information |
| --- | --- |
| `s3` | storage_url bucket, access_key, secret_key; region when required by bucket configuration |
| `s3_compatible` | bucket, access_key, secret_key, public HTTPS endpoint_url |
| `oss` | bucket, access_key, secret_key, public HTTPS endpoint_url |
| `cos` | bucket including APPID, access_key, secret_key, region |
| `bos` | bucket, access_key, secret_key, region; HTTPS S3 delivery supports bj, su, hkg |
| `tos` | bucket, access_key, secret_key, region |
| `gcs` | bucket; service_account with project_id, client_email, private_key |
| `azure` | container in storage_url, account_name, write-capable container sas_token |

Use the convenient `Storage` and `ServiceAccount` aliases or their generated full names. `storage_url` is a bucket/container name, not a local path or an object URL. `object_prefix` selects an optional folder. The server validates provider-specific requirements and allowed endpoint addresses; local shape validation does not establish that credentials work.

Data-result default prefixes are frozen to the UTC creation date. Customer result objects use `<prefix>/<source>/<bulkId>/results/part-NNNNNNNN.jsonl.gz`, followed by a manifest. Download object prefixes can include `YYYYMMDD` tokens resolved at upload time. Internal signed results remain available for 24 hours. Input upload expires after 15 minutes; the SDK performs a bounded, credential-free PUT pinned to the authenticated result origin without following redirects. It uses the caller's stream or a regular file and never creates an empty upload automatically.

## State and recovery

| State | Caller action |
| --- | --- |
| awaiting_upload | Finish the original signed upload, then Start. Recheck state if a start response was lost. |
| running / finalizing | Get or Wait; do not start a duplicate. |
| paused | Inspect PauseReason and delivery/error metadata, fix the cause with Update, explicitly Resume. |
| completed | Fetch Manifest and Rows before retention expires. Check individual row status. |
| failed / cancelled | Preserve existing state and diagnose; a replacement is a separate explicit task. |

Save only the accepted bulk ID/source in ordinary recovery files. Signed URLs, storage secrets and API keys require secret storage and are intentionally omitted from the example's job record. On an uncertain Create outcome, there may be no known ID; inspect your account/support before replaying. The ID-only example can recover started jobs, but cannot restore an in-memory upload capability after a process exits. For durable pre-upload recovery, save the full `BulkJobCreated` response in an OS-protected application store before uploading, or use the local CLI. Never place signed upload URLs in ordinary job records or logs. An empty reserved record after an uncertain request also needs reconciliation. Cross-machine exactly-once execution is not promised.

`Bulk.Update` merges storage credential fields server-side. `UpdateBulkJobRequest{RemoveCallback:true}` sends an explicit null callback; `CallbackURL:flashdata.Ptr(url)` replaces it. Supply a partial options map to update credentials, for example `map[string]any{"storage": map[string]any{"secret_key": loadedSecret}}`. Do not resend invented empty credentials. After scanning finishes, paused data bulk jobs restrict changes to the permitted credential fields.

The example reserves the recovery file atomically before job creation, syncs the accepted ID before uploading, uses a validated input snapshot, refuses to overwrite result output or recovery records, and preserves partial rows on errors. Recovery with `-job` never creates another job, uploads input, or resumes a pause automatically. `-start` and `-resume` are explicit state transitions; inspect the current state first. A failed/invalid receipt is preserved before the program exits with failure.

## Streaming

`Manifest` validates the requested job ID and unique nonnegative part numbers; `Rows` follows their manifest order using authenticated management result routes, never arbitrary manifest hrefs. `IterRows` handles one gzip NDJSON part with strict UTF-8, object rows, CRLF/blank lines and an unterminated final row. Invalid JSON, truncated gzip, oversized compressed/decompressed parts and oversized rows fail explicitly. Breaking out of the iterator closes readers. `Part` returns bounded compressed bytes for callers that manage decoding themselves. Cancellation and request deadlines remain recognizable through `errors.Is` even during compressed reads, gzip headers and NDJSON streaming. CRLF separators do not consume the configured JSON row limit. Keep the client's HTTP timeout or a caller context long enough for the size of each part.
