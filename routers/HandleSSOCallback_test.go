package routers

import (
	"encoding/json"
	"testing"
)

func TestMustJSON_Basic(t *testing.T) {
	input := map[string]any{
		"enabled": true,
		"label":   "Sign in",
	}

	result := mustJSON(input)

	var parsed map[string]any
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatalf("mustJSON produced invalid JSON: %v\nOutput: %s", err, result)
	}

	if parsed["enabled"] != true {
		t.Fatalf("expected enabled=true, got %v", parsed["enabled"])
	}
	if parsed["label"] != "Sign in" {
		t.Fatalf("expected label='Sign in', got %v", parsed["label"])
	}
}

func TestMustJSON_SpecialCharacters(t *testing.T) {
	// This was the original bug — unescaped strings could inject JSON
	input := map[string]any{
		"label": `He said "hello" & <script>alert('xss')</script>`,
	}

	result := mustJSON(input)

	var parsed map[string]any
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatalf("mustJSON failed with special characters: %v\nOutput: %s", err, result)
	}

	if parsed["label"] != input["label"] {
		t.Fatalf("expected label to be preserved, got %v", parsed["label"])
	}
}

func TestMustJSON_EmptyMap(t *testing.T) {
	result := mustJSON(map[string]any{})

	var parsed map[string]any
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatalf("mustJSON failed on empty map: %v", err)
	}
	if len(parsed) != 0 {
		t.Fatalf("expected empty map, got %v", parsed)
	}
}

func TestMustJSON_BooleanValues(t *testing.T) {
	input := map[string]any{
		"enabled":  true,
		"disabled": false,
	}

	result := mustJSON(input)

	var parsed map[string]any
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatalf("mustJSON failed with booleans: %v\nOutput: %s", err, result)
	}

	if parsed["enabled"] != true {
		t.Fatalf("expected enabled=true, got %v", parsed["enabled"])
	}
	if parsed["disabled"] != false {
		t.Fatalf("expected disabled=false, got %v", parsed["disabled"])
	}
}

func TestMustJSON_URLValue(t *testing.T) {
	input := map[string]any{
		"login_url": "/auth/sso/login",
	}

	result := mustJSON(input)

	var parsed map[string]any
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatalf("mustJSON failed with URL: %v\nOutput: %s", err, result)
	}

	if parsed["login_url"] != "/auth/sso/login" {
		t.Fatalf("expected '/auth/sso/login', got %v", parsed["login_url"])
	}
}

func TestMustJSON_Newlines(t *testing.T) {
	input := map[string]any{
		"text": "line1\nline2\ttab",
	}

	result := mustJSON(input)

	var parsed map[string]any
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatalf("mustJSON failed with newlines: %v\nOutput: %s", err, result)
	}

	if parsed["text"] != "line1\nline2\ttab" {
		t.Fatalf("expected newlines preserved, got %v", parsed["text"])
	}
}
