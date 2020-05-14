package strava

import (
	"encoding/json"
)

func isFault(data []byte) bool {
	var f apiFault
	err := json.Unmarshal(data, &f)
	if err != nil {
		// Failed to unmarshal into a Strava error: assume this is
		// valid data. This test isn't conclusive as almost any hash
		// will unmarshal into {[]}.
		return false
	}
	// Messages present? Assume error.
	if len(f.Message) > 0 {
		return true
	}
	return false
}

type apiError struct {
	Code     string `json:"code"`
	Field    string `json:"field"`
	Resource string `json:"resource"`
}

type apiFault struct {
	Message string     `json:"message"`
	Errors  []apiError `json:"errors"`
}
