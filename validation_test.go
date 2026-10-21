package flashdata

import (
	"encoding/json"
	"testing"
)

func TestPublicOfflineValidationForCLI(t *testing.T) {
	if len(Sources()) != 15 {
		t.Fatal("missing source")
	}
	for _, source := range Sources() {
		schema, err := QuerySchema(source)
		if err != nil || !json.Valid(schema) {
			t.Fatal(source, err)
		}
	}
	if err := ValidateQuery(Query{Source: SourceGoogleSearch, Query: "hello", Params: map[string]any{"num": 5}}, false); err != nil {
		t.Fatal(err)
	}
	if err := ValidateQuery(Query{Source: SourceGoogleSearch, Query: "hello", Params: map[string]any{"num": 101}}, false); err == nil {
		t.Fatal("invalid query accepted")
	}
	if err := ValidateBulkCreate(CreateBulkJobRequest{Source: SourceYouTubeMetadata}); err != nil {
		t.Fatal(err)
	}
	if err := ValidateBulkCreate(CreateBulkJobRequest{Source: SourceYouTubeDownload}); err == nil {
		t.Fatal("missing download storage")
	}
	if err := ValidateRegistration(RegistrationRequest{Name: "Go CLI", Scopes: []Source{SourceGoogleSearch}}); err != nil {
		t.Fatal(err)
	}
	if err := ValidateRegistration(RegistrationRequest{Name: "", Scopes: []Source{SourceGoogleSearch}}); err == nil {
		t.Fatal("invalid registration accepted")
	}
	if _, err := NormalizeOrigin("http://example.com"); err == nil {
		t.Fatal("unsafe origin")
	}
}
