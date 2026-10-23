# Source reference

[FlashData](https://flashdata.dev) · [API documentation](https://flashdata.dev/docs)

All source inputs expose `Request()` for generic query methods. Source helpers have matching `*Async` variants, except Download which is always submitted. Optional Params fields use pointers; the SDK sends no invented defaults.

All 14 realtime helpers also have `WithFormat` variants taking `ContentOptions`
for JSON, plain text or Markdown. Background helpers, including Download, have
`WithFormat` variants taking `ResponseOptions` for JSON or plain-text receipts.
Select Markdown when reading completed results with `Jobs.ResultWithFormat`.
See [response formats](response-formats.md); input parameters below are unchanged.

## google_search

[Official google_search reference](https://flashdata.dev/docs/sources/google_search) · [Runnable example](../examples/google_search/main.go)

Modes: realtime, async, batch.

| Parameter | Type | Required | Constraints |
| --- | --- | --- | --- |
| `query` | string | yes | {"minLength": 1, "maxLength": 2000} |
| `gl` | string | no | {"minLength": 2, "maxLength": 2, "pattern": "^[A-Za-z]{2}$"} |
| `hl` | string | no | {"minLength": 1, "maxLength": 35, "pattern": "^[A-Za-z0-9-]+$"} |
| `location` | string | no | {"minLength": 1, "maxLength": 200} |
| `num` | integer | no | {"minimum": 1, "maximum": 100} |
| `page` | integer | no | {"minimum": 1, "maximum": 100} |
| `tbs` | string | no | {"minLength": 1, "maxLength": 200} |
| `autocorrect` | boolean | no | {} |

## google_images

[Official google_images reference](https://flashdata.dev/docs/sources/google_images) · [Runnable example](../examples/google_images/main.go)

Modes: realtime, async, batch.

| Parameter | Type | Required | Constraints |
| --- | --- | --- | --- |
| `query` | string | yes | {"minLength": 1, "maxLength": 2000} |
| `gl` | string | no | {"minLength": 2, "maxLength": 2, "pattern": "^[A-Za-z]{2}$"} |
| `hl` | string | no | {"minLength": 1, "maxLength": 35, "pattern": "^[A-Za-z0-9-]+$"} |
| `location` | string | no | {"minLength": 1, "maxLength": 200} |
| `num` | integer | no | {"minimum": 1, "maximum": 100} |
| `page` | integer | no | {"minimum": 1, "maximum": 100} |
| `tbs` | string | no | {"minLength": 1, "maxLength": 200} |
| `autocorrect` | boolean | no | {} |

## google_news

[Official google_news reference](https://flashdata.dev/docs/sources/google_news) · [Runnable example](../examples/google_news/main.go)

Modes: realtime, async, batch.

| Parameter | Type | Required | Constraints |
| --- | --- | --- | --- |
| `query` | string | yes | {"minLength": 1, "maxLength": 2000} |
| `gl` | string | no | {"minLength": 2, "maxLength": 2, "pattern": "^[A-Za-z]{2}$"} |
| `hl` | string | no | {"minLength": 1, "maxLength": 35, "pattern": "^[A-Za-z0-9-]+$"} |
| `location` | string | no | {"minLength": 1, "maxLength": 200} |
| `num` | integer | no | {"minimum": 1, "maximum": 100} |
| `page` | integer | no | {"minimum": 1, "maximum": 100} |
| `tbs` | string | no | {"minLength": 1, "maxLength": 200} |
| `autocorrect` | boolean | no | {} |

## google_shopping

[Official google_shopping reference](https://flashdata.dev/docs/sources/google_shopping) · [Runnable example](../examples/google_shopping/main.go)

Modes: realtime, async, batch.

| Parameter | Type | Required | Constraints |
| --- | --- | --- | --- |
| `query` | string | yes | {"minLength": 1, "maxLength": 2000} |
| `gl` | string | no | {"minLength": 2, "maxLength": 2, "pattern": "^[A-Za-z]{2}$"} |
| `hl` | string | no | {"minLength": 1, "maxLength": 35, "pattern": "^[A-Za-z0-9-]+$"} |
| `location` | string | no | {"minLength": 1, "maxLength": 200} |
| `num` | integer | no | {"minimum": 1, "maximum": 100} |
| `page` | integer | no | {"minimum": 1, "maximum": 100} |
| `tbs` | string | no | {"minLength": 1, "maxLength": 200} |
| `autocorrect` | boolean | no | {} |

## google_videos

[Official google_videos reference](https://flashdata.dev/docs/sources/google_videos) · [Runnable example](../examples/google_videos/main.go)

Modes: realtime, async, batch.

| Parameter | Type | Required | Constraints |
| --- | --- | --- | --- |
| `query` | string | yes | {"minLength": 1, "maxLength": 2000} |
| `gl` | string | no | {"minLength": 2, "maxLength": 2, "pattern": "^[A-Za-z]{2}$"} |
| `hl` | string | no | {"minLength": 1, "maxLength": 35, "pattern": "^[A-Za-z0-9-]+$"} |
| `location` | string | no | {"minLength": 1, "maxLength": 200} |
| `num` | integer | no | {"minimum": 1, "maximum": 100} |
| `page` | integer | no | {"minimum": 1, "maximum": 100} |
| `tbs` | string | no | {"minLength": 1, "maxLength": 200} |
| `autocorrect` | boolean | no | {} |

## google_places

[Official google_places reference](https://flashdata.dev/docs/sources/google_places) · [Runnable example](../examples/google_places/main.go)

Modes: realtime, async, batch.

| Parameter | Type | Required | Constraints |
| --- | --- | --- | --- |
| `query` | string | yes | {"minLength": 1, "maxLength": 2000} |
| `gl` | string | no | {"minLength": 2, "maxLength": 2, "pattern": "^[A-Za-z]{2}$"} |
| `hl` | string | no | {"minLength": 1, "maxLength": 35, "pattern": "^[A-Za-z0-9-]+$"} |
| `location` | string | no | {"minLength": 1, "maxLength": 200} |
| `num` | integer | no | {"minimum": 1, "maximum": 100} |
| `page` | integer | no | {"minimum": 1, "maximum": 100} |
| `tbs` | string | no | {"minLength": 1, "maxLength": 200} |
| `autocorrect` | boolean | no | {} |

## youtube_search

[Official youtube_search reference](https://flashdata.dev/docs/sources/youtube_search) · [Runnable example](../examples/youtube_search/main.go)

Modes: realtime, async, batch.

| Parameter | Type | Required | Constraints |
| --- | --- | --- | --- |
| `query` | string | yes | {"minLength": 1, "maxLength": 2000} |
| `max_results` | integer | no | {"minimum": 1, "maximum": 20, "default": 20} |
| `filters` | object | no | {} |

## youtube_search_extended

[Official youtube_search_extended reference](https://flashdata.dev/docs/sources/youtube_search_extended) · [Runnable example](../examples/youtube_search_extended/main.go)

Modes: realtime, async, batch.

| Parameter | Type | Required | Constraints |
| --- | --- | --- | --- |
| `query` | string | yes | {"minLength": 1, "maxLength": 2000} |
| `max_results` | integer | no | {"minimum": 1, "maximum": 700, "default": 20} |
| `filters` | object | no | {} |

## youtube_metadata

[Official youtube_metadata reference](https://flashdata.dev/docs/sources/youtube_metadata) · [Runnable example](../examples/youtube_metadata/main.go)

Modes: realtime, async, batch.

| Parameter | Type | Required | Constraints |
| --- | --- | --- | --- |
| `video_id` | string | yes | {"minLength": 11, "maxLength": 11, "pattern": "^[A-Za-z0-9_-]{11}$"} |
| `include_formats` | boolean | no | {"default": true} |
| `include_chapters` | boolean | no | {"default": true} |

## youtube_transcript

[Official youtube_transcript reference](https://flashdata.dev/docs/sources/youtube_transcript) · [Runnable example](../examples/youtube_transcript/main.go)

Modes: realtime, async, batch.

| Parameter | Type | Required | Constraints |
| --- | --- | --- | --- |
| `video_id` | string | yes | {"minLength": 11, "maxLength": 11, "pattern": "^[A-Za-z0-9_-]{11}$"} |
| `language` | string | no | {"minLength": 1, "maxLength": 35, "pattern": "^[A-Za-z0-9-]+$", "default": "en"} |
| `auto` | boolean | no | {"default": true} |

## youtube_captions

[Official youtube_captions reference](https://flashdata.dev/docs/sources/youtube_captions) · [Runnable example](../examples/youtube_captions/main.go)

Modes: realtime, async, batch.

| Parameter | Type | Required | Constraints |
| --- | --- | --- | --- |
| `video_id` | string | yes | {"minLength": 11, "maxLength": 11, "pattern": "^[A-Za-z0-9_-]{11}$"} |
| `language` | string | no | {"minLength": 1, "maxLength": 35, "pattern": "^[A-Za-z0-9-]+$", "default": "en"} |
| `auto` | boolean | no | {"default": false} |

## youtube_trainability

[Official youtube_trainability reference](https://flashdata.dev/docs/sources/youtube_trainability) · [Runnable example](../examples/youtube_trainability/main.go)

Modes: realtime, async, batch.

| Parameter | Type | Required | Constraints |
| --- | --- | --- | --- |
| `video_id` | string | yes | {"minLength": 11, "maxLength": 11, "pattern": "^[A-Za-z0-9_-]{11}$"} |

## youtube_channel

[Official youtube_channel reference](https://flashdata.dev/docs/sources/youtube_channel) · [Runnable example](../examples/youtube_channel/main.go)

Modes: realtime, async, batch.

| Parameter | Type | Required | Constraints |
| --- | --- | --- | --- |
| `handle` | string | yes | {"minLength": 1, "maxLength": 100} |
| `video_limit` | integer | no | {"minimum": 1, "maximum": 100, "default": 20} |

## youtube_suggestions

[Official youtube_suggestions reference](https://flashdata.dev/docs/sources/youtube_suggestions) · [Runnable example](../examples/youtube_suggestions/main.go)

Modes: realtime, async, batch.

| Parameter | Type | Required | Constraints |
| --- | --- | --- | --- |
| `query` | string | yes | {"minLength": 1, "maxLength": 2000} |
| `language` | string | no | {"minLength": 1, "maxLength": 35, "pattern": "^[A-Za-z0-9-]+$", "default": "en"} |
| `region` | string | no | {"minLength": 2, "maxLength": 2, "default": "US", "pattern": "^[A-Za-z]{2}$"} |

## youtube_download

[Official youtube_download reference](https://flashdata.dev/docs/sources/youtube_download) · [Runnable example](../examples/youtube_download/main.go)

Modes: async, batch.

| Parameter | Type | Required | Constraints |
| --- | --- | --- | --- |
| `video_id` | string | yes | {"minLength": 11, "maxLength": 11, "pattern": "^[A-Za-z0-9_-]{11}$"} |
| `download_type` | string | no | {"minLength": 1, "maxLength": 11, "default": "audio_video", "enum": ["audio_video", "video", "audio"]} |
| `video_quality` | string | no | {"minLength": 1, "maxLength": 40, "default": "<=720", "enum": ["best", "worst", "<=144", "==144", ">=144", "<=360", "==360", ">=360", "<=480", "==480", ">=480", "<=720", "==720", ">=720", "<=1080", "==1080", ">=1080", "<=1440", "==1440", ">=1440", "<=2160", "==2160", ">=2160", "<=4320", "==4320", ">=4320"]} |
| `fps` | string | no | {"minLength": 1, "maxLength": 6, "pattern": "^(?:>=\|<=\|==)(?:[1-9]\\d?\|1\\d\\d\|2[0-3]\\d\|240)$"} |
| `audio_language` | string | no | {"minLength": 1, "maxLength": 8, "default": "default", "enum": ["default", "original"]} |
| `audio_quality` | string | no | {"minLength": 1, "maxLength": 10, "default": "best", "enum": ["best", "320", "256", "192", "128"]} |
| `yt_dlp_format` | string | no | {"minLength": 1, "maxLength": 1024} |
| `upload_metadata` | boolean | no | {"default": false} |
| `output_format` | string | no | {"minLength": 1, "maxLength": 10, "default": "mp4", "enum": ["mp4", "webm", "mkv", "m4a", "mp3", "opus"]} |
| `start_at` | string | no | {"minLength": 1, "maxLength": 8, "pattern": "^\\d{1,2}:[0-5]\\d:[0-5]\\d$"} |
| `end_at` | string | no | {"minLength": 1, "maxLength": 8, "pattern": "^\\d{1,2}:[0-5]\\d:[0-5]\\d$"} |
