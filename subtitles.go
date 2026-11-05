package flashdata

import "encoding/json"

// Preserve the existing Language *string field while encoding a language list
// under the same API property. A non-nil empty slice is invalid, not omitted.
func marshalSubtitleParams(language *string, languages []string, auto *bool) ([]byte, error) {
	if language != nil && languages != nil {
		return nil, &ValidationError{Field: "language", Message: "set either Language or Languages"}
	}
	var value any
	if language != nil {
		value = *language
	}
	if languages != nil {
		value = languages
	}
	return json.Marshal(struct {
		Language any   `json:"language,omitempty"`
		Auto     *bool `json:"auto,omitempty"`
	}{value, auto})
}

func unmarshalSubtitleParams(data []byte) (*string, []string, *bool, error) {
	var wire struct {
		Language json.RawMessage `json:"language"`
		Auto     *bool           `json:"auto"`
	}
	if err := json.Unmarshal(data, &wire); err != nil {
		return nil, nil, nil, err
	}
	if len(wire.Language) == 0 || string(wire.Language) == "null" {
		return nil, nil, wire.Auto, nil
	}
	var language string
	if err := json.Unmarshal(wire.Language, &language); err == nil {
		return &language, nil, wire.Auto, nil
	}
	var languages []string
	if err := json.Unmarshal(wire.Language, &languages); err != nil {
		return nil, nil, nil, err
	}
	return nil, languages, wire.Auto, nil
}

func (p YouTubeTranscriptParams) MarshalJSON() ([]byte, error) {
	return marshalSubtitleParams(p.Language, p.Languages, p.Auto)
}
func (p *YouTubeTranscriptParams) UnmarshalJSON(data []byte) error {
	language, languages, auto, err := unmarshalSubtitleParams(data)
	if err == nil {
		p.Language, p.Languages, p.Auto = language, languages, auto
	}
	return err
}
func (p YouTubeCaptionsParams) MarshalJSON() ([]byte, error) {
	return marshalSubtitleParams(p.Language, p.Languages, p.Auto)
}
func (p *YouTubeCaptionsParams) UnmarshalJSON(data []byte) error {
	language, languages, auto, err := unmarshalSubtitleParams(data)
	if err == nil {
		p.Language, p.Languages, p.Auto = language, languages, auto
	}
	return err
}
