package main

import "testing"

func TestParseClaimsVerdict_PlainJSON(t *testing.T) {
	v, err := parseClaimsVerdict(`{"supported": true, "unsupported_claims": []}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !v.Supported || len(v.UnsupportedClaims) != 0 {
		t.Fatalf("unexpected verdict: %+v", v)
	}
}

func TestParseClaimsVerdict_FencedWithProse(t *testing.T) {
	raw := "Here is my verdict:\n```json\n{\"supported\": false, \"unsupported_claims\": [\"dosage 5x is invented\"]}\n```\n"
	v, err := parseClaimsVerdict(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v.Supported {
		t.Fatal("expected supported=false")
	}
	if len(v.UnsupportedClaims) != 1 || v.UnsupportedClaims[0] != "dosage 5x is invented" {
		t.Fatalf("unexpected claims: %+v", v.UnsupportedClaims)
	}
}

func TestParseClaimsVerdict_NoJSON(t *testing.T) {
	if _, err := parseClaimsVerdict("I cannot answer that."); err == nil {
		t.Fatal("expected error when no JSON object present")
	}
}
