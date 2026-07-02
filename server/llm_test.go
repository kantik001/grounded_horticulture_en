package main

import (
	"strings"
	"testing"
)

func TestParseLLMStreamData(t *testing.T) {
	content, err := parseLLMStreamData(`{"choices":[{"delta":{"content":"scab"}}]}`)
	if err != nil {
		t.Fatal(err)
	}
	if content != "scab" {
		t.Fatalf("got %q", content)
	}
}

func TestReadLLMStream(t *testing.T) {
	body :=
		"data: {\"choices\":[{\"delta\":{\"content\":\"A\"}}]}\n\n" +
			"data: {\"choices\":[{\"delta\":{\"content\":\"B\"}}]}\n\n" +
			"data: [DONE]\n\n"
	full, err := readLLMStream(strings.NewReader(body), nil)
	if err != nil {
		t.Fatal(err)
	}
	if full != "AB" {
		t.Fatalf("got %q", full)
	}
}
