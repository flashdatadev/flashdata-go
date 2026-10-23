# JSON, plain text and Markdown

[FlashData](https://flashdata.dev) · [API documentation](https://flashdata.dev/docs)

Existing SDK methods return typed JSON. Use their `WithFormat` variants to select
headers for one call. `Accept` sets HTTP `Accept`; `RespondWith` sets
`X-Respond-With`. Neither belongs in query inputs or `Params`.

| `ContentOptions.Accept` | `ContentOptions.RespondWith` | Response |
| --- | --- | --- |
| `AcceptJSON` or omitted | Omitted | Original JSON fields in `response.JSON` |
| `AcceptText` | Omitted | Original fields as indented text in `response.Text` |
| `AcceptJSON` or omitted | `RespondMarkdown` | JSON with Markdown in `content`, replacing `results` |
| `AcceptText` | `RespondMarkdown` | Metadata followed by Markdown in `response.Text` |

Markdown retains source links, transcript timestamps and response metadata. The
API formats retrieved content without generating a summary.

## Search and transcripts

```go
response, err := client.Google.SearchWithFormat(ctx,
    flashdata.GoogleSearchInput{Query: "vector databases"},
    flashdata.ContentOptions{RespondWith: flashdata.RespondMarkdown},
)
if err != nil {
    return err
}
var markdown flashdata.MarkdownQueryResult
if err := response.DecodeJSON(&markdown); err != nil {
    return err
}
fmt.Println(markdown.Content)
```

```go
response, err := client.YouTube.TranscriptWithFormat(ctx,
    flashdata.YouTubeTranscriptInput{
        VideoID: "dQw4w9WgXcQ",
        Params: &flashdata.YouTubeTranscriptParams{
            Language: flashdata.Ptr("en"), Auto: flashdata.Ptr(true),
        },
    },
    flashdata.ContentOptions{
        Accept: flashdata.AcceptText, RespondWith: flashdata.RespondMarkdown,
    },
)
if err != nil {
    return err
}
fmt.Print(response.Text)
```

Each call creates a separate query. All 14 realtime source helpers and
`Queries.RealtimeWithFormat` accept `ContentOptions`. Omit `RespondWith` and set
`Accept: flashdata.AcceptText` for the original fields as text.

`FormattedResponse.JSON` retains the exact body, including future fields and large
numbers. `DecodeJSON` accepts wire models such as `DataQueryResponse`,
`MarkdownQueryResult`, `BatchResponse`, `JobResult` or an application model.
Use `json.RawMessage` or `json.Number` for numeric fields that must remain exact.
`DecodeJSON` rejects text responses; read `Text` instead.

## Background jobs and batches

`Queries.SubmitWithFormat`, `Queries.BatchWithFormat`, source-specific
`AsyncWithFormat` helpers, `YouTube.DownloadWithFormat` and `Jobs.GetWithFormat`
accept `ResponseOptions{Accept: flashdata.AcceptText}` for receipts or status.
Use the existing typed submission methods when you need to save job IDs for recovery.

Choose Markdown when retrieving a completed result:

```go
receipt, err := client.Google.SearchAsync(ctx,
    flashdata.GoogleSearchInput{Query: "vector databases"}, "",
)
if err != nil {
    return err
}
fmt.Println("Save this ID:", receipt.ID)
if _, err := client.Jobs.Wait(ctx, receipt.ID, flashdata.PollOptions{}); err != nil {
    return err
}
response, err := client.Jobs.ResultWithFormat(ctx, receipt.ID,
    flashdata.ContentOptions{
        Accept: flashdata.AcceptText, RespondWith: flashdata.RespondMarkdown,
    },
)
if err != nil {
    return err
}
fmt.Print(response.Text)
```

Reading a completed result in another format does not submit another query or
charge query credits. Preferences are not saved with the job.

`Jobs.Get` returns the typed job directly. `Jobs.GetWithFormat` keeps the full wire
envelope: JSON contains `{"job": {...}}`, and text includes the outer `Job` field.
`Jobs.Wait` continues to return a typed job.

Default JSON result reads follow the signed storage redirect. Plain-text and
Markdown results return directly from the API. The SDK checks storage origins and
keeps API keys off signed storage requests.

Download manifests support Markdown through `Jobs.ResultWithFormat`. Media bytes,
Bulk files, callbacks, account, public discovery and onboarding keep their existing
formats. Pagination and polling continue to use JSON.

## Options and errors

The SDK accepts `AcceptJSON`, `AcceptText` or an omitted `Accept`, and
`RespondMarkdown` or an omitted `RespondWith`. Invalid choices fail before sending
a request. HTTP media ranges, weights and charset parameters are API features;
use the canonical constants in SDK options.

`ResponseOptions` has no `RespondWith`: Markdown applies only to content reads.
The raw HTTP API ignores that header on receipts and status. Errors still return
`APIError` when text was requested. An interrupted submission can return
`SubmissionUnknownError`; reconcile before retrying. Text receipts have no typed
ID validation or automatic recovery-ID extraction. Keep JSON receipts for durable
job processing.

Try the [runnable example](../examples/response_formats/main.go), or see the
[API response-format guide](https://flashdata.dev/docs/guides/response-formats)
for HTTP requests and bodies.
