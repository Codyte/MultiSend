package main

import "testing"

func TestRedactDiagnosticSecrets(t *testing.T) {
	input := map[string]any{
		"display_name": "node",
		"node_secret":  "sensitive",
		"nested": map[string]any{
			"access_token": "token-value",
			"password":     "password-value",
			"safe":         "visible",
		},
	}
	redacted := redactDiagnosticSecrets(input).(map[string]any)
	if redacted["node_secret"] != "[redacted]" {
		t.Fatalf("node secret was not redacted: %#v", redacted)
	}
	nested := redacted["nested"].(map[string]any)
	if nested["access_token"] != "[redacted]" || nested["password"] != "[redacted]" || nested["safe"] != "visible" {
		t.Fatalf("unexpected nested redaction: %#v", nested)
	}
	if input["node_secret"] != "sensitive" {
		t.Fatal("redaction mutated the source object")
	}
}
