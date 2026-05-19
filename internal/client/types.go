package client

import (
	"encoding/json"
	"strconv"
)

type Schedule struct {
	Repeat     string `json:"repeat"`
	Paused     bool   `json:"paused"`
	IsArchived *bool  `json:"is_archived,omitempty"`
}

type ConditionValue struct {
	String *string
	Bool   *bool
	Number *float64
}

func (c ConditionValue) MarshalJSON() ([]byte, error) {
	switch {
	case c.String != nil:
		return json.Marshal(*c.String)
	case c.Bool != nil:
		return json.Marshal(*c.Bool)
	case c.Number != nil:
		return json.Marshal(*c.Number)
	}
	return []byte("null"), nil
}

func (c *ConditionValue) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		c.String = &s
		return nil
	}
	var b bool
	if err := json.Unmarshal(data, &b); err == nil {
		c.Bool = &b
		return nil
	}
	var n float64
	if err := json.Unmarshal(data, &n); err == nil {
		c.Number = &n
		return nil
	}
	return nil
}

// ConditionValueFromString builds a ConditionValue, inferring its JSON type:
// "true"/"false" become booleans, integers and decimals become numbers,
// everything else stays a string. This lets Terraform users keep
// `values = list(string)` while the API receives correctly-typed JSON.
//
// Caveats: the literal strings "true", "false", and bare numeric strings
// cannot be sent as JSON strings through this path. Wrap them differently
// upstream if you hit that edge case (rare for filter conditions).
func ConditionValueFromString(s string) ConditionValue {
	switch s {
	case "true":
		b := true
		return ConditionValue{Bool: &b}
	case "false":
		b := false
		return ConditionValue{Bool: &b}
	}
	if n, err := strconv.ParseFloat(s, 64); err == nil && isCanonicalNumberString(s) {
		return ConditionValue{Number: &n}
	}
	v := s
	return ConditionValue{String: &v}
}

// isCanonicalNumberString rejects strings that parse as numbers but look like
// they shouldn't (leading zeros, whitespace, etc.) so e.g. "007" stays a string.
func isCanonicalNumberString(s string) bool {
	if s == "" {
		return false
	}
	// Must round-trip through ParseFloat → FormatFloat without losing form.
	// We accept anything strconv considers a number with no leading zeros
	// (except "0" or "0.x") and no leading/trailing whitespace.
	if s[0] == ' ' || s[len(s)-1] == ' ' {
		return false
	}
	start := 0
	if s[0] == '-' || s[0] == '+' {
		start = 1
	}
	if start >= len(s) {
		return false
	}
	if len(s)-start >= 2 && s[start] == '0' && s[start+1] != '.' && s[start+1] != 'e' && s[start+1] != 'E' {
		return false
	}
	return true
}

type Condition struct {
	Modifier *string          `json:"modifier,omitempty"`
	Operator string           `json:"operator"`
	Values   []ConditionValue `json:"values"`
}

type DynamicFilterPreset struct {
	ID              *int        `json:"id,omitempty"`
	DynamicFilterID json.Number `json:"dynamic_filter_id"`
	PresetCondition Condition   `json:"preset_condition"`
	PublicHidden    *bool       `json:"public_hidden,omitempty"`
}
