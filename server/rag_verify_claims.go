package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// Claim-level answer verification (optional, RAG_VERIFY_CLAIMS_ENABLED=true).
//
// The numeric check in rag_verify.go only guards against hallucinated numbers.
// This module adds an LLM judge pass: it extracts factual claims from the
// answer and checks each one is supported by the retrieved fragments. An
// unsupported claim downgrades the answer to a soft-fail warning, the same as
// a failed numeric check.
//
// The judge is fail-open by design: if the LLM call or verdict parsing fails,
// the answer is served with only the numeric check (logged and counted in
// metrics) — a judge outage must not take the chat down.

const claimsJudgeSystemPrompt = `You are a strict fact-checking judge for a retrieval-augmented assistant.
You receive SOURCES (article fragments) and an ANSWER produced from them.

Check whether every factual claim in the ANSWER is directly supported by the SOURCES.
Ignore the standard disclaimer and generic advice to consult a specialist.
A claim is "unsupported" only if it states a fact (disease, treatment, dosage,
variety, number, procedure) that the SOURCES do not contain or contradict.

Respond with a single JSON object and nothing else:
{"supported": true|false, "unsupported_claims": ["...", "..."]}
Set "supported" to false if there is at least one unsupported claim.`

// claimsVerdict is the judge's JSON response.
type claimsVerdict struct {
	Supported         bool     `json:"supported"`
	UnsupportedClaims []string `json:"unsupported_claims"`
}

// claimVerifyEnabled reports whether the LLM claim judge is turned on.
func claimVerifyEnabled() bool {
	return config.VerifyClaimsEnabled && config.LLMAPIKey != ""
}

// verifyRAGAnswerClaims asks the LLM judge whether the answer's claims are
// grounded in the fragments. Returns (supported, reason). On any judge error
// it returns (true, ...) — fail-open — so callers keep the numeric-only result.
func verifyRAGAnswerClaims(ctx context.Context, answer string, fragments []RAGFragment) (bool, string) {
	body := answerBodyForVerification(answer)
	if strings.TrimSpace(body) == "" || len(fragments) == 0 {
		return true, "claim check skipped"
	}

	var sources strings.Builder
	for i, f := range fragments {
		fmt.Fprintf(&sources, "[%d] %s\n", i+1, f.Content)
	}
	userPrompt := fmt.Sprintf("SOURCES:\n%s\nANSWER:\n%s", sources.String(), body)

	raw, err := callLLMCompletion(ctx, []Message{
		{Role: "system", Content: claimsJudgeSystemPrompt},
		{Role: "user", Content: userPrompt},
	})
	if err != nil {
		recordLLMError()
		return true, "claim check unavailable: " + err.Error()
	}

	verdict, err := parseClaimsVerdict(raw)
	if err != nil {
		return true, "claim verdict unparsable: " + err.Error()
	}
	if verdict.Supported {
		return true, "claims supported by sources"
	}
	reason := "unsupported claim(s)"
	if len(verdict.UnsupportedClaims) > 0 {
		reason = "unsupported: " + strings.Join(verdict.UnsupportedClaims, "; ")
	}
	return false, reason
}

// parseClaimsVerdict extracts the JSON verdict from an LLM reply that may wrap
// it in prose or a ```json fence.
func parseClaimsVerdict(raw string) (claimsVerdict, error) {
	var v claimsVerdict
	start := strings.IndexByte(raw, '{')
	end := strings.LastIndexByte(raw, '}')
	if start < 0 || end < start {
		return v, fmt.Errorf("no JSON object in judge reply")
	}
	if err := json.Unmarshal([]byte(raw[start:end+1]), &v); err != nil {
		return v, err
	}
	return v, nil
}
