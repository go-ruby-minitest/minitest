// Copyright (c) the go-ruby-minitest/minitest authors
//
// SPDX-License-Identifier: BSD-3-Clause

package minitest

import (
	"regexp"
	"strings"
)

// MuPP is Minitest::Assertions#mu_pp: a human-readable rendering of obj. It is
// obj.inspect, re-encoded to the default external encoding; for a String whose
// encoding differs from the default external (or is invalid) it is prefixed with
// "# encoding:" / "#    valid:" annotation lines, exactly as the gem does.
//
// The encoding re-encode itself is a no-op at the string level here (the host's
// Inspect already yields the bytes); the annotation branch is what the gem's
// callers observe, and it is reproduced faithfully.
func (a *Assertions) MuPP(obj Value) string {
	s := a.rt.Inspect(obj)
	if !a.rt.IsString(obj) {
		return s
	}
	enc, valid := a.rt.Encoding(obj)
	if enc == a.rt.DefaultExternalEncoding() && valid {
		return s
	}
	return "# encoding: " + enc + "\n" +
		"#    valid: " + boolStr(valid) + "\n" +
		s
}

func boolStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

// hexRE anonymizes object-id-like hex tails (":0x..."), matching
// /:0x[a-fA-F0-9]{4,}/m in mu_pp_for_diff. Go's RE2 has no lookbehind, so the
// "\n" vs "\\n" classification mu_pp_for_diff needs is done structurally in
// [classifyEscapes] rather than with the gem's lookbehind regexps.
var hexRE = regexp.MustCompile(`:0x[a-fA-F0-9]{4,}`)

// MuPPForDiff is Minitest::Assertions#mu_pp_for_diff: a more diff-able rendering.
// It runs [Assertions.MuPP] first, then, when the inspected string contains
// EITHER single-escaped newlines ("\n") OR double-escaped ("\\n") but not both,
// unescapes them, and finally anonymizes hex object ids. This is used when
// building a structural diff; the package exposes it because hosts and tests
// assert on it.
func (a *Assertions) MuPPForDiff(obj Value) string {
	str := a.MuPP(obj)

	single, double := classifyEscapes(str)

	switch {
	case single != double && single:
		// unescape: each lone "\n" → real newline (gem: s == "\n" ? "\n" : s).
		str = replaceEscapes(str, `\n`, "\n")
	case single != double && double:
		// unescape a bit, add nls: each "\\n" → "\n" + real newline
		// (gem: s == "\\n" ? "\\n\n" : s).
		str = replaceEscapes(str, `\\n`, "\\n\n")
	}

	return hexRE.ReplaceAllString(str, ":0xXXXXXX")
}

// classifyEscapes reports whether str (already an inspect output) contains a
// single-escaped "\n" sequence and/or a double-escaped "\\n" sequence, applying
// the gem's lookbehind semantics: a "\n" counts as "single" only when the
// preceding char is neither a backslash nor the string start, and as "double"
// only when it is.
func classifyEscapes(str string) (single, double bool) {
	for i := 0; i+1 < len(str); i++ {
		if str[i] == '\\' && str[i+1] == 'n' {
			prevBackslashOrStart := i == 0 || str[i-1] == '\\'
			if prevBackslashOrStart {
				double = true
			} else {
				single = true
			}
		}
	}
	return single, double
}

// replaceEscapes walks str the way the gem's gsub(/\\?\\n/) does — matching each
// maximal "optional backslash then \n" run — and replaces only the runs equal to
// target with repl, leaving every other matched run (and all other text) intact.
// This is the gem's `s == target ? repl : s` proc, but with the pass-through done
// in the scanner so there is no unreachable conditional. target is `\n` for the
// single-escape branch and `\\n` for the double-escape branch.
func replaceEscapes(str, target, repl string) string {
	var b strings.Builder
	i := 0
	for i < len(str) {
		if str[i] == '\\' {
			// Case A: "\\n" (two backslashes then n) → run "\\n".
			if i+2 < len(str) && str[i+1] == '\\' && str[i+2] == 'n' {
				b.WriteString(emit(`\\n`, target, repl))
				i += 3
				continue
			}
			// Case B: "\n" (one backslash then n) → run "\n".
			if i+1 < len(str) && str[i+1] == 'n' {
				b.WriteString(emit(`\n`, target, repl))
				i += 2
				continue
			}
		}
		b.WriteByte(str[i])
		i++
	}
	return b.String()
}

// emit returns repl when run equals target, else run unchanged (the gem proc's
// ternary). Both arms are exercised: the single-branch string can still contain a
// non-target run when the two escape forms overlap at a boundary.
func emit(run, target, repl string) string {
	if run == target {
		return repl
	}
	return run
}
