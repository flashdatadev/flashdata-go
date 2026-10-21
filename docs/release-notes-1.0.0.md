# FlashData Go v1.0.0

[FlashData](https://flashdata.dev) · [API documentation](https://flashdata.dev/docs) · [English README](https://github.com/flashdatadev/flashdata-go/blob/v1.0.0/README.md) · [简体中文](https://github.com/flashdatadev/flashdata-go/blob/v1.0.0/README.zh-CN.md)

Initial version of the FlashData Go SDK. The GitHub Release is prepared as a draft; the public v1.0.0 source tag is independently installable through Go tooling.

## Install

Requires Go 1.25 or newer:

```bash
go get github.com/flashdatadev/flashdata-go@v1.0.0
```

Pass your API key explicitly to `flashdata.New(flashdata.Options{APIKey: key})`. The SDK does not read environment variables or `.env` files. Get a key from your [FlashData account](https://flashdata.dev).

## Included

- All 15 Google and YouTube sources, typed inputs and optional provider payload hints.
- Realtime, background and mixed-source batch queries; accepted-job polling and result retrieval.
- Complete bulk create/upload/start/control/recovery workflows, streaming results and customer-storage media delivery.
- Public discovery and account reads, verified API-key identity, and protocol-2 machine onboarding.
- Explicit timeouts and cancellation, isolated API-key/bearer/signed-storage authentication, redacted diagnostics, and recovery IDs for uncertain submissions without automatic paid retries.
- English and Chinese READMEs, 22 runnable examples, API/reference documentation and offline regression tests.

## Packages and verification

The assets contain `flashdata-go_1.0.0.zip` (canonical Go module ZIP), `v1.0.0.mod`, `v1.0.0.info`, `build-report.json`, these release notes and `SHA256SUMS`. The ZIP uses Go's standard `github.com/flashdatadev/flashdata-go@v1.0.0/` prefix. Normal users install with Go rather than manually unpacking it.

`build-report.json` records the source commit, module checksums and validation. Check all downloaded assets from one directory:

```bash
sha256sum -c SHA256SUMS
```

PowerShell: use `Get-FileHash -Algorithm SHA256 <file>` and compare its hash with the corresponding `SHA256SUMS` entry. A checksum verifies the downloaded bytes; it is not a publisher signature.

## Operational limits

Onboarding and recurring identity checks depend on server feature gates and the reviewed backend identity endpoint. Draft preparation does not deploy or enable those features. Paid production queries, live claiming/callbacks and customer cloud storage have not been exercised by the offline release checks. A timeout stops local waiting; accepted server work may continue. Preserve recovery IDs and reconcile uncertain submissions before intentional resubmission.

## 简体中文摘要

首个 Go SDK 版本，覆盖 15 个 Google/YouTube 数据源、实时/后台/批量查询、完整 Bulk 流程、账户读取和代理接入。需要 Go 1.25+，并显式传入 API 密钥。源码标签 `v1.0.0` 可通过 Go 安装；GitHub Release 保持草稿，不会自动发布。附件提供标准 Go 模块 ZIP、模块元数据、构建记录及 SHA-256 校验和。接入功能仍受后端部署和服务端开关控制，离线检查不代表已验证真实付费调用或客户云存储。
