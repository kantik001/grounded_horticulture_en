package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Verifies that a RAG 422 response is a soft miss, not a transport error.
func TestFetchRAGContext422IsSoftMiss(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write([]byte(`{"success":false,"error":"No information found in articles."}`))
	}))
	defer srv.Close()

	old := config
	config = &Config{PythonRAGURL: srv.URL}
	defer func() { config = old }()

	out, err := fetchRAGContext(context.Background(), "test", "apple")
	if err != nil {
		t.Fatalf("422 soft miss should not return transport error: %v", err)
	}
	if out == nil || out.Success {
		t.Fatalf("expected success=false, got %+v", out)
	}
	if out.Error == "" {
		t.Fatal("expected error message in body")
	}
}

// Verifies that decimal numbers are extracted from answer text.
func TestExtractNumbersFromText(t *testing.T) {
	nums := extractNumbersFromText("Growth 748.5 cm and 31.8%")
	if len(nums) != 2 {
		t.Fatalf("expected 2 numbers, got %v", nums)
	}
	if nums[0] != 748.5 || nums[1] != 31.8 {
		t.Fatalf("unexpected values: %v", nums)
	}
}

// Verifies that answers without numbers pass verification.
func TestVerifyRAGAnswer_NoNumbersOK(t *testing.T) {
	fragments := []RAGFragment{{Filename: "Article", Content: "Scab appears as spots on leaves."}}
	answer := appendRAGDisclaimer("Scab appears as spots on leaves.")
	ok, reason := verifyRAGAnswer(answer, fragments)
	if !ok {
		t.Fatalf("expected pass, got: %s", reason)
	}
}

// Verifies that numbers present in the source fragments pass verification.
func TestVerifyRAGAnswer_NumberInContextOK(t *testing.T) {
	fragments := []RAGFragment{{Filename: "Table", Content: "Mean value 77 and replication 3-72."}}
	answer := appendRAGDisclaimer("Mean 77.")
	ok, reason := verifyRAGAnswer(answer, fragments)
	if !ok {
		t.Fatalf("expected pass, got: %s", reason)
	}
}

// Verifies that a number missing from the sources fails verification.
func TestVerifyRAGAnswer_HallucinatedNumberFails(t *testing.T) {
	fragments := []RAGFragment{{Filename: "Article", Content: "No numbers in the text."}}
	answer := appendRAGDisclaimer("Profitability 72%.")
	ok, reason := verifyRAGAnswer(answer, fragments)
	if ok {
		t.Fatal("expected verification to fail for hallucinated number")
	}
	if reason == "" {
		t.Fatal("expected non-empty reason")
	}
}

// Verifies that source attributions are stripped and the disclaimer is appended.
func TestAppendRAGDisclaimer_StripsSourceAndAddsDisclaimer(t *testing.T) {
	raw := "Answer on topic.\n\nSource: \"Secret article\""
	out := appendRAGDisclaimer(raw)
	if strings.Contains(out, "Source:") || strings.Contains(out, "Secret article") {
		t.Fatalf("source attribution should be removed: %q", out)
	}
	if !strings.Contains(out, "Does not replace an on-site agronomist") {
		t.Fatalf("expected disclaimer, got: %q", out)
	}
}

// Verifies that filler intro phrases are removed from the answer.
func TestCleanRAGAnswer_StripsIntroPhrase(t *testing.T) {
	out := cleanRAGAnswer("I think apple scab is dangerous for the harvest.")
	if strings.Contains(out, "I think") {
		t.Fatalf("intro should be stripped, got: %q", out)
	}
	if !strings.Contains(out, "scab") {
		t.Fatalf("got %q", out)
	}
}
