//go:build unit

package service

import "testing"

func TestIsOpenAIGPT54Model_GuardsNonGPTFamilies(t *testing.T) {
	t.Parallel()

	if !isOpenAIGPT54Model("gpt-5.4") {
		t.Fatal("expected gpt-5.4 to match GPT-5.4 pricing guard")
	}
	if isOpenAIGPT54Model("gpt-4o") {
		t.Fatal("did not expect gpt-4o to match GPT-5.4 pricing guard")
	}
	if isOpenAIGPT54Model("claude-opus-4.6") {
		t.Fatal("did not expect claude-opus-4.6 to match GPT-5.4 pricing guard")
	}
	if isOpenAIGPT54Model("gemini-3-flash-preview") {
		t.Fatal("did not expect gemini-3-flash-preview to match GPT-5.4 pricing guard")
	}
}
