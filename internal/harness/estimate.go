package harness

import "unicode/utf8"

// EstimateUsage approximates tokens from character counts using the Metrics
// Procedure formula (chars ÷ 4), rounded to the nearest integer.
func EstimateUsage(prompt, output string) Usage {
	return Usage{
		InputTokens:  roundCharsToTokens(prompt),
		OutputTokens: roundCharsToTokens(output),
	}
}

// ResolveUsage prefers harness-reported counts when either side is non-zero;
// otherwise falls back to EstimateUsage(prompt, output).
func ResolveUsage(reported Usage, prompt, output string) Usage {
	if reported.InputTokens > 0 || reported.OutputTokens > 0 {
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
