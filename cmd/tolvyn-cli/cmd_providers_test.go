package main

import "testing"

// BUG A: `tolvyn providers add deepseek` must be accepted (the proxy routes it and
// the dashboard offers it); obviously-bogus providers are still rejected. The set
// mirrors the proxy adapters + dashboard dropdown {openai, anthropic, google, deepseek}.
func TestIsAddableProvider(t *testing.T) {
	accept := []string{"openai", "anthropic", "google", "deepseek", "DeepSeek", "OPENAI"}
	for _, p := range accept {
		if !isAddableProvider(p) {
			t.Errorf("isAddableProvider(%q) = false, want true", p)
		}
	}
	reject := []string{"", "deepsek", "openai ", "claude", "gpt", "bogus", "anthropicc"}
	for _, p := range reject {
		if isAddableProvider(p) {
			t.Errorf("isAddableProvider(%q) = true, want false", p)
		}
	}
}
