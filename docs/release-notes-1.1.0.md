# FlashData Go v1.1.0

[FlashData](https://flashdata.dev) · [API documentation](https://flashdata.dev/docs)

Use new `WithFormat` methods to choose JSON, plain text or Markdown for each call.
Existing typed JSON methods and their serialization remain compatible.

- All 14 realtime source helpers and completed-job results support `ContentOptions`.
- Submissions, batches and job status support `ResponseOptions` for JSON or text.
- `AcceptText` selects plain text independently. `RespondMarkdown` selects Markdown
  content, either inside JSON `content` or following metadata in a text response.
- `FormattedResponse.JSON` preserves wire fields and numbers; `DecodeJSON` accepts
  application models. `Text` contains the server's UTF-8 response.
- Preserve job metadata, structured errors, gzip/size bounds, cancellation and
  credential separation for signed storage reads.

Requires Go 1.25+. Install or upgrade:

```bash
go get github.com/flashdatadev/flashdata-go@v1.1.0
```

See [response formats](response-formats.md) for examples. The
[GitHub release](https://github.com/flashdatadev/flashdata-go/releases/tag/v1.1.0)
provides the canonical Go module ZIP, `.mod`/`.info` metadata, both READMEs,
release notes, a build report and `SHA256SUMS`. Assets and the module tag identify
the same source commit. Normal Go installs retain checksum verification.

## 简体中文

1.1.0 新增按调用选择响应格式的 `WithFormat` 方法，原有 JSON 方法兼容。
`AcceptText` 独立选择纯文本；`RespondMarkdown` 用于实时内容和已完成结果。
后台提交、批量提交和任务状态也支持 JSON 或纯文本。保留任务元数据、错误类型、
响应大小限制和存储凭据隔离。需要 Go 1.25+。
