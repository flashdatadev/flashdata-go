package flashdata

import "encoding/json"

// ValidateQuery validates a request without dispatching or spending credits.
// asynchronous permits all background/batch sources and callbacks.
func ValidateQuery(query Query, asynchronous bool) error { return validateQuery(query, asynchronous) }

// NormalizeOrigin validates an HTTPS service origin (or loopback HTTP for tests).
func NormalizeOrigin(value string) (string, error) { return origin(value) }

// QuerySchema returns a copy of the SDK's source query schema. It can be used
// for offline command discovery and input normalization; no defaults are added.
func QuerySchema(source Source) (json.RawMessage, error) {
	var schemas map[string]json.RawMessage
	if err := json.Unmarshal(sourceSchemaBytes, &schemas); err != nil {
		return nil, &ProtocolError{"invalid embedded source contract"}
	}
	value, ok := schemas[string(source)]
	if !ok {
		return nil, &ValidationError{Field: "source", Message: "unsupported source"}
	}
	return value, nil
}

// Sources returns all source identifiers in deterministic display order.
func Sources() []Source {
	return []Source{SourceGoogleSearch, SourceGoogleImages, SourceGoogleNews, SourceGoogleShopping, SourceGoogleVideos, SourceGooglePlaces, SourceYouTubeSearch, SourceYouTubeSearchExtended, SourceYouTubeMetadata, SourceYouTubeTranscript, SourceYouTubeCaptions, SourceYouTubeTrainability, SourceYouTubeChannel, SourceYouTubeSuggestions, SourceYouTubeDownload}
}
