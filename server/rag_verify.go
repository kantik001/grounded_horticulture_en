package main

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
)

// ragAnswerDisclaimer is appended to RAG answers (no article titles).
// Keep in sync with rag/verifier.py (RAG_ANSWER_DISCLAIMER) and tests/test_verifier.py.
const ragAnswerDisclaimer = "Reference information from the knowledge base. Does not replace an on-site agronomist visit; product decisions must follow labels and local regulations."

var (
	reNumberWord = regexp.MustCompile(`\b\d+(?:\.\d+)?\b`)
	reMultiSpace = regexp.MustCompile(`\s+`)
	reThink      = regexp.MustCompile(`(?i)</?think>`)
	reAnswerTag  = regexp.MustCompile(`(?i)</?answer>`)
	reSystemTag  = regexp.MustCompile(`(?i)</?system>`)
	reAbot       = regexp.MustCompile(`(?i)\bworker\b`)
	reIntro      = regexp.MustCompile(`(?i)^(Well|Let's look|So|I think|I need to answer|From the context|Now I understand|From the table)[,:.]?\s*`)
	reSourceLine = regexp.MustCompile(`(?im)^\s*Source:.*\n?`)
)

// extractNumbersFromText collects numbers from text for RAG answer verification.
func extractNumbersFromText(s string) []float64 {
	s = strings.ReplaceAll(s, ",", ".")
	var out []float64
	for _, m := range reNumberWord.FindAllString(s, -1) {
		v, err := strconv.ParseFloat(m, 64)
		if err == nil {
			out = append(out, v)
		}
	}
	return out
}

// cleanRAGAnswer removes service tags and filler phrases from the LLM reply.
func cleanRAGAnswer(text string) string {
	if text == "" {
		return "Answer was not formed correctly."
	}
	text = reThink.ReplaceAllString(text, "")
	text = reAnswerTag.ReplaceAllString(text, "")
	text = reSystemTag.ReplaceAllString(text, "")
	text = reAbot.ReplaceAllString(text, "")
	text = reIntro.ReplaceAllString(text, "")
	text = strings.TrimSpace(reMultiSpace.ReplaceAllString(text, " "))
	if text == "" {
		return "Answer was not formed correctly."
	}
	return text
}

// stripSourceAttribution removes "Source:" lines before showing the answer to the user.
func stripSourceAttribution(answer string) string {
	s := reSourceLine.ReplaceAllString(answer, "")
	return strings.TrimSpace(reMultiSpace.ReplaceAllString(s, " "))
}

// appendRAGDisclaimer appends the standard disclaimer to a RAG answer.
func appendRAGDisclaimer(answer string) string {
	body := stripSourceAttribution(answer)
	if body == "" {
		return ragAnswerDisclaimer
	}
	if strings.Contains(body, "Does not replace an on-site agronomist") {
		return body
	}
	return body + "\n\n" + ragAnswerDisclaimer
}

// answerBodyForVerification returns answer text without disclaimer and sources for number checks.
func answerBodyForVerification(answer string) string {
	s := stripSourceAttribution(answer)
	s = strings.ReplaceAll(s, ragAnswerDisclaimer, "")
	return strings.TrimSpace(s)
}

// verifyRAGAnswer checks that all numbers in the answer appear in article fragments.
func verifyRAGAnswer(answer string, fragments []RAGFragment) (bool, string) {
	if answer == "" {
		return false, "Answer is missing"
	}
	var ctx strings.Builder
	for _, f := range fragments {
		ctx.WriteString(f.Content)
		ctx.WriteByte('\n')
	}
	numsAns := extractNumbersFromText(answerBodyForVerification(answer))
	if len(numsAns) == 0 {
		return true, "Verification passed"
	}
	numsCtx := extractNumbersFromText(ctx.String())
	var missing []float64
	for _, n := range numsAns {
		found := false
		for _, c := range numsCtx {
			if math.Abs(n-c) < 0.01 {
				found = true
				break
			}
		}
		if !found {
			missing = append(missing, n)
		}
	}
	if len(missing) > 0 {
		return false, fmt.Sprintf("Number(s) %v not found in sources.", missing)
	}
	return true, "Verification passed"
}
