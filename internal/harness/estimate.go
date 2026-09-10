package harness

import "unicode/utf8"

// EstimateUsage approximates tokens from character counts using the Metrics
// Procedure formula (chars ÷ 4), rounded to the nearest integer.
func EstimateUsage(prompt, output string) Usage {
	return Usage{
		InputTokens:  roundCharsToTokens(prompt),
		OutputTokens: roundCharsToTokens(output),
	}.WithCallOccupancy()
}

// ResolveUsage prefers harness-reported billed counts when any usage field is
// non-zero; otherwise falls back to EstimateUsage(prompt, output). It does
// not invent ContextTokens from a billed aggregate.
func ResolveUsage(reported Usage, prompt, output string) Usage {
	if reported.HasCounts() {
		return reported
	}
	return EstimateUsage(prompt, output)
}

func roundCharsToTokens(s string) int64 {
	n := utf8.RuneCountInString(s)
	if n <= 0 {
		return 0
	}
	return int64((n + 2) / 4) // round(n/4) for positive ints
}
