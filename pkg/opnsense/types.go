// Copyright (c) Matthew Mellor
// SPDX-License-Identifier: MPL-2.0

package opnsense

import (
	"bytes"
	"encoding/json"
	"sort"
	"strconv"
	"strings"
)

// BoolToString converts a Go bool to OPNsense's string boolean format.
// true → "1", false → "0".
func BoolToString(b bool) string {
	if b {
		return "1"
	}
	return "0"
}

// StringToBool converts OPNsense's string boolean to a Go bool.
// "1" → true, anything else → false.
func StringToBool(s string) bool {
	return s == "1"
}

// CSVToSlice splits a comma-separated string into a string slice.
// Returns an empty slice (not nil) for empty strings.
func CSVToSlice(s string) []string {
	if s == "" {
		return []string{}
	}
	parts := strings.Split(s, ",")
	for i, p := range parts {
		parts[i] = strings.TrimSpace(p)
	}
	return parts
}

// DelimitedToSlice accepts the comma-separated values used by most OPNsense
// selectors and the newline-separated values used by alias content fields.
func DelimitedToSlice(s string) []string {
	if strings.TrimSpace(s) == "" {
		return []string{}
	}
	parts := strings.FieldsFunc(s, func(r rune) bool { return r == ',' || r == '\n' || r == '\r' })
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if value := strings.TrimSpace(part); value != "" {
			result = append(result, value)
		}
	}
	return result
}

// SliceToCSV joins a string slice into a comma-separated string.
func SliceToCSV(s []string) string {
	return strings.Join(s, ",")
}

// StringToInt64 converts a string to int64. Returns an error for invalid input.
func StringToInt64(s string) (int64, error) {
	return strconv.ParseInt(s, 10, 64)
}

// Int64ToString converts an int64 to its string representation.
func Int64ToString(n int64) string {
	return strconv.FormatInt(n, 10)
}

// SelectedMap is a custom type for OPNsense single-select enum fields.
// OPNsense returns these as {"key": {"value": "...", "selected": 1}}.
// UnmarshalJSON extracts the key where selected == 1.
type SelectedMap string

// selectedEntry represents a single option in the OPNsense SelectedMap.
type selectedEntry struct {
	Value    string      `json:"value"`
	Selected json.Number `json:"selected"`
}

// UnmarshalJSON extracts the selected key from an OPNsense SelectedMap response.
// Handles the "selected" field as both int (1/0) and string ("1"/"0").
// Also handles empty array ([]), null, and plain string values.
func (s *SelectedMap) UnmarshalJSON(data []byte) error {
	// OPNsense sometimes returns [] or "" for empty enum fields.
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "[]" || trimmed == "null" {
		*s = ""
		return nil
	}
	// Handle plain string values (some fields return "value" not {"key":{"selected":1}}).
	if len(trimmed) > 0 && trimmed[0] == '"' {
		var str string
		if err := json.Unmarshal(data, &str); err == nil {
			*s = SelectedMap(str)
			return nil
		}
	}

	var raw map[string]selectedEntry
	if err := json.Unmarshal(data, &raw); err != nil {
		*s = ""
		return nil
	}
	for key, entry := range raw {
		if entry.Selected.String() == "1" {
			*s = SelectedMap(key)
			return nil
		}
	}
	*s = ""
	return nil
}

// SelectedMapList is a custom type for OPNsense multi-select fields.
// UnmarshalJSON extracts all keys where selected == 1, sorted alphabetically.
type SelectedMapList []string

// UnmarshalJSON extracts all selected keys from an OPNsense multi-select response.
// Returns a sorted slice for deterministic output. Returns empty slice (not nil) when
// no keys are selected.
// Handles both map format ({"key": {"value":"...","selected":1}}) and empty array ([]).
func (s *SelectedMapList) UnmarshalJSON(data []byte) error {
	// OPNsense returns [] for empty multi-select fields instead of {}.
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "[]" || trimmed == "null" || trimmed == `""` {
		*s = []string{}
		return nil
	}
	if len(trimmed) > 0 && trimmed[0] == '"' {
		var str string
		if err := json.Unmarshal(data, &str); err == nil {
			*s = []string{str}
			return nil
		}
	}

	var raw map[string]selectedEntry
	if err := json.Unmarshal(data, &raw); err != nil {
		// If it fails as a map, treat as empty.
		*s = []string{}
		return nil
	}

	var selected []string
	for key, entry := range raw {
		if entry.Selected.String() == "1" {
			selected = append(selected, key)
		}
	}

	sort.Strings(selected)

	if selected == nil {
		*s = []string{}
	} else {
		*s = selected
	}
	return nil
}

// NewlineSelectedMapList decodes alias content, whose scalar wire form uses
// newlines. It deliberately keeps comma-containing entries intact.
type NewlineSelectedMapList []string

// UnmarshalJSON preserves comma-containing entries and splits only newlines.
func (s *NewlineSelectedMapList) UnmarshalJSON(data []byte) error {
	trimmed := strings.TrimSpace(string(data))
	if len(trimmed) > 0 && trimmed[0] == '"' {
		var value string
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		parts := strings.FieldsFunc(value, func(r rune) bool { return r == '\n' || r == '\r' })
		for i := range parts {
			parts[i] = strings.TrimSpace(parts[i])
		}
		sort.Strings(parts)
		*s = parts
		return nil
	}
	var selected SelectedMapList
	if err := json.Unmarshal(data, &selected); err != nil {
		return err
	}
	*s = NewlineSelectedMapList(selected)
	return nil
}

// CSVSelectedMapList decodes selector fields whose scalar wire form is CSV.
type CSVSelectedMapList []string

// UnmarshalJSON splits scalar CSV and otherwise decodes the selected-map form.
func (s *CSVSelectedMapList) UnmarshalJSON(data []byte) error {
	trimmed := strings.TrimSpace(string(data))
	if len(trimmed) > 0 && trimmed[0] == '"' {
		var value string
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		values := CSVToSlice(value)
		sort.Strings(values)
		*s = values
		return nil
	}
	var selected SelectedMapList
	if err := json.Unmarshal(data, &selected); err != nil {
		return err
	}
	*s = CSVSelectedMapList(selected)
	return nil
}

// OrderedSelectedMapList is a custom type for OPNsense multi-select fields
// where JSON object order is semantically significant.
type OrderedSelectedMapList []string

// UnmarshalJSON extracts selected keys in the order returned by OPNsense.
// Unlike SelectedMapList, it does not sort the selected keys.
func (s *OrderedSelectedMapList) UnmarshalJSON(data []byte) error {
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "[]" || trimmed == "null" || trimmed == `""` {
		*s = []string{}
		return nil
	}
	if len(trimmed) > 0 && trimmed[0] == '"' {
		var str string
		if err := json.Unmarshal(data, &str); err == nil {
			*s = []string{str}
			return nil
		}
	}

	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	token, err := decoder.Token()
	if err != nil {
		*s = []string{}
		return nil
	}
	delim, ok := token.(json.Delim)
	if !ok || delim != '{' {
		*s = []string{}
		return nil
	}

	selected := make([]string, 0)
	for decoder.More() {
		keyToken, err := decoder.Token()
		if err != nil {
			*s = []string{}
			return nil
		}
		key, ok := keyToken.(string)
		if !ok {
			*s = []string{}
			return nil
		}

		var entry selectedEntry
		if err := decoder.Decode(&entry); err != nil {
			*s = []string{}
			return nil
		}
		if entry.Selected.String() == "1" {
			selected = append(selected, key)
		}
	}
	if _, err := decoder.Token(); err != nil {
		*s = []string{}
		return nil
	}

	*s = selected
	return nil
}
