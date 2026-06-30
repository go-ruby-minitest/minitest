// Copyright (c) the go-ruby-minitest/minitest authors
//
// SPDX-License-Identifier: BSD-3-Clause

package minitest

import "strings"

// RaiseOutcome is what the host reports after running an assert_raises block: the
// block either raised an exception (Raised true) or returned normally. When it
// raised, ErrClass/ErrMessage/Backtrace describe it. The host classifies whether
// the raised class is one of the expected classes (Matched) and whether it was
// itself a Minitest Assertion/Skip (IsAssertion) or a SignalException/SystemExit
// (IsPassthrough), since those re-raise rather than fail.
type RaiseOutcome struct {
	Raised        bool
	ErrClass      string
	ErrMessage    string
	Backtrace     []string // already minitest-filtered by the host
	Matched       bool     // raised class is a member of exp
	IsAssertion   bool     // raised was Minitest::Assertion (incl Skip/UnexpectedError)
	IsPassthrough bool     // raised was SignalException or SystemExit
}

// AssertRaises is assert_raises. exp are the expected exception-class Values
// (their inspect is used in messages); customMsg is an optional leading message
// the gem renders as "#{msg}.\n" when present (it was the trailing String arg).
//
// It returns (reRaise, err): when reRaise is non-nil the host must re-raise that
// exact outcome's exception (the Assertion/Skip/Signal/SystemExit passthrough
// cases). Otherwise err is nil on success (the matched exception is "returned" by
// the host from its own block driver) or an [*Assertion] describing the failure.
//
// expInspect is mu_pp(exp) when exp has >1 element, else mu_pp(exp.first) — the
// host passes the already-inspected forms via expSingleInspect (for the size==1
// "expected but nothing raised" / unexpected-error case) and expArrayInspect.
func (a *Assertions) AssertRaises(out RaiseOutcome, customMsg, expArrayInspect, expSingleInspect string) (reRaise bool, err error) {
	var msg string
	if customMsg != "" {
		msg = customMsg + ".\n"
	}

	if out.Raised {
		switch {
		case out.Matched:
			// pass # count assertion; the host returns the exception object.
			_ = a.Pass()
			return false, nil
		case out.IsAssertion:
			// don't count assertion; re-raise.
			return true, nil
		case out.IsPassthrough:
			return true, nil
		default:
			// flunk with exception_details
			details := a.exceptionDetails(out, msg+expArrayInspect+" exception expected, not")
			return false, a.Flunk(details)
		}
	}

	// Nothing was raised.
	expShown := expArrayInspect
	if expSingleInspect != "" {
		expShown = expSingleInspect
	}
	return false, a.Flunk(msg + expShown + " expected but nothing was raised.")
}

// exceptionDetails is Minitest::Assertions#exception_details: the multi-line
// rendering used when assert_raises catches the wrong exception class.
func (a *Assertions) exceptionDetails(out RaiseOutcome, msg string) string {
	bt := strings.Join(out.Backtrace, "\n")
	return strings.Join([]string{
		msg,
		"Class: <" + out.ErrClass + ">",
		"Message: <" + inspectString(out.ErrMessage) + ">",
		"---Backtrace---",
		bt,
		"---------------",
	}, "\n")
}

// inspectString renders a Go string the way Ruby String#inspect would for the
// exception-message case: wrap in double quotes and escape the usual controls.
// (The host could supply this, but assert_raises's detail line is pure text the
// library owns, so it formats it here for the common ASCII case.)
func inspectString(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		case '\n':
			b.WriteString(`\n`)
		case '\t':
			b.WriteString(`\t`)
		case '\r':
			b.WriteString(`\r`)
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
	return b.String()
}

// ThrowOutcome is what the host reports after an assert_throws block: whether the
// expected symbol was thrown (Caught), and, for the "not :other" message tail,
// the name of a different symbol that was thrown instead (OtherSym, "" if none).
type ThrowOutcome struct {
	Caught   bool
	OtherSym string // the symbol actually thrown, when different from sym
}

// AssertThrows is assert_throws. symInspect is mu_pp(sym) (e.g. ":foo"). When the
// host caught a *different* symbol it sets OtherSym so the message gains the
// ", not …" tail, matching the gem's ArgumentError/NameError handling.
func (a *Assertions) AssertThrows(out ThrowOutcome, symInspect string, msg string) error {
	def := "Expected " + symInspect + " to have been thrown"
	if out.OtherSym != "" {
		def += ", not " + out.OtherSym
	}
	full := message(msg, "", func() string { return def })
	return a.assertBool(out.Caught, full)
}

// AssertOutput is assert_output. The host captures stdout/stderr from the block
// and reports them. stdoutExp/stderrExp are the expectations: nil means "don't
// care"; a string means exact match; a regexp means pattern. The host signals
// which mode applies per stream and supplies, for an exact-string mismatch, the
// diff message, or for a regexp mismatch, the assert_match message.
//
// To keep the assertion logic pure, the comparison itself is done by the host
// (it owns == and =~ over Ruby strings/regexps); AssertOutput is handed the
// already-decided pass/fail and, on failure, the composed sub-message. It returns
// nil on full success or the first failing stream's [*Assertion].
//
// outErr/outStd are per-stream results: nil = stream not checked or passed.
func (a *Assertions) AssertOutput(stderrResult, stdoutResult error) error {
	// The gem evaluates stderr first (y = ... "In stderr") then stdout
	// (x = ... "In stdout"); both run, so both bump the assertion count, but the
	// first failure is what surfaces. The host has already run the per-stream
	// assert_equal/assert_match (bumping Count) and passes their errors.
	if stderrResult != nil {
		return stderrResult
	}
	return stdoutResult
}

// OutputRequiresBlock returns the flunk the gem raises when assert_output is
// called without a block.
func (a *Assertions) OutputRequiresBlock() error {
	return a.Flunk("assert_output requires a block to capture output.")
}

// RaisesRequiresBlock returns the flunk assert_raises raises without a block.
func (a *Assertions) RaisesRequiresBlock() error {
	return a.Flunk("assert_raises requires a block to capture errors.")
}
