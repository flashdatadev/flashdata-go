package flashdata

// Storage configures a customer-owned download destination or bulk result copy.
// Credentials must come from the caller's secret store. Provider requirements
// remain authoritative on the service; see https://flashdata.dev/docs.
type Storage = YouTubeDownloadParamsStorage

// ServiceAccount contains the service-account fields used for GCS destinations.
type ServiceAccount = YouTubeDownloadParamsStorageServiceAccount

// JSON serialization intentionally retains credentials for requests and
// caller-owned secure persistence. Ordinary diagnostic formatting does not.
func (v YouTubeDownloadParamsStorage) String() string {
	return "flashdata.Storage{credentials:redacted}"
}
func (v YouTubeDownloadParamsStorage) GoString() string { return v.String() }
func (v YouTubeDownloadParamsStorageServiceAccount) String() string {
	return "flashdata.ServiceAccount{credentials:redacted}"
}
func (v YouTubeDownloadParamsStorageServiceAccount) GoString() string { return v.String() }
func (v BulkJobCreated) String() string                               { return "flashdata.BulkJobCreated{upload:redacted}" }
func (v BulkJobCreated) GoString() string                             { return v.String() }
func (v BulkJobCreatedUpload) String() string                         { return "flashdata.BulkJobCreatedUpload{redacted}" }
func (v BulkJobCreatedUpload) GoString() string                       { return v.String() }
