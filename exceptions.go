// Copyright (c) the go-ruby-minitest/minitest authors
//
// SPDX-License-Identifier: BSD-3-Clause

package minitest

import "strings"

// Assertion is Minitest::Assertion — the failure raised by a failing assertion.
// In Ruby it subclasses Exception (not StandardError) so test bodies don't
// accidentally rescue it. Here it is a Go error carrying the failure message and
// the backtrace the host supplies for the raising frame.
type Assertion struct {
	Msg string
	// Backtrace is the (already filtered or raw) Ruby backtrace, most-recent
	// frame first, as the host captured it. Location uses it to point at the
	// offending line.
	Backtrace []string
}

// Error implements error; it returns the assertion message.
func (a *Assertion) Error() string { return a.Msg }

// Message returns the assertion message (Ruby Exception#message).
func (a *Assertion) Message() string { return a.Msg }

// ResultLabel is the long label for this kind of result: "Failure".
func (a *Assertion) ResultLabel() string { return "Failure" }

// ResultCode is the single-character code: the first letter of the label.
func resultCode(r Reportable2) string {
	l := r.ResultLabel()
	if l == "" {
		return ""
	}
	return l[:1]
}

// ResultCode is the single-character code for a plain Assertion ("F").
func (a *Assertion) ResultCode() string { return resultCode(a) }

// assertionRE matches the backtrace frames that belong to the assertion
// machinery itself (assert/refute/flunk/pass/fail/raise/must/wont), mirroring
// Minitest::Assertion::RE. Location skips past the deepest such frame to report
// the user's call site.
var assertionMethods = []string{
	"assert", "refute", "flunk", "pass", "fail", "raise", "must", "wont",
}

// frameIsAssertion reports whether a backtrace frame names one of the assertion
// helper methods, matching the spirit of Minitest::Assertion::RE
// (/in [`'](?:[^']+[#.])?(?:assert|refute|flunk|pass|fail|raise|must|wont)/).
func frameIsAssertion(frame string) bool {
	// Find an "in `" or "in '" marker, then check the method name after it,
	// optionally prefixed by "Receiver#" / "Receiver.".
	for _, lead := range []string{"in `", "in '"} {
		i := strings.Index(frame, lead)
		if i < 0 {
			continue
		}
		rest := frame[i+len(lead):]
		// Optional "owner#" / "owner." prefix: skip to the last '#' or '.' that
		// precedes the method name token.
		if j := strings.IndexAny(rest, "#."); j >= 0 {
			// Only treat it as an owner prefix when what follows still starts an
			// identifier (so we don't eat into the method itself for "a.b").
			cand := rest[j+1:]
			if startsWithAssertion(cand) {
				return true
			}
		}
		if startsWithAssertion(rest) {
			return true
		}
	}
	return false
}

func startsWithAssertion(s string) bool {
	for _, m := range assertionMethods {
		if strings.HasPrefix(s, m) {
			return true
		}
	}
	return false
}

// Location returns the "file:line" the assertion should be attributed to: the
// frame just past the deepest assertion-helper frame (Minitest::Assertion#location).
// bt is the already-backtrace-filtered list (host responsibility), most-recent
// frame first as Ruby presents it. The ":in ..." suffix is stripped.
func (a *Assertion) Location() string {
	return locationFromBacktrace(a.Backtrace)
}

func locationFromBacktrace(bt []string) string {
	idx := -1
	for i, s := range bt {
		if frameIsAssertion(s) {
			idx = i
		}
	}
	var loc string
	switch {
	case idx+1 < len(bt) && idx >= 0:
		loc = bt[idx+1]
	case len(bt) > 0:
		loc = bt[len(bt)-1]
	default:
		loc = "unknown:-1"
	}
	// Strip ":in ...".
	if i := strings.Index(loc, ":in "); i >= 0 {
		loc = loc[:i]
	}
	return loc
}

// Skip is Minitest::Skip — the assertion raised by skip. It is an Assertion
// subclass whose result label is "Skipped".
type Skip struct {
	Assertion
}

// ResultLabel returns "Skipped".
func (s *Skip) ResultLabel() string { return "Skipped" }

// ResultCode returns "S".
func (s *Skip) ResultCode() string { return resultCode(s) }

// UnexpectedError is Minitest::UnexpectedError — wraps an error that was raised
// during a run but was not an Assertion (a genuine test error). Its message and
// backtrace come from the wrapped error; the host supplies ErrorClass /
// ErrorMessage / the (filtered) backtrace.
type UnexpectedError struct {
	Assertion
	// ErrorClass is the wrapped error's class name (e.g. "RuntimeError").
	ErrorClass string
	// ErrorMessage is the wrapped error's #message.
	ErrorMessage string
}

// ResultLabel returns "Error".
func (u *UnexpectedError) ResultLabel() string { return "Error" }

// ResultCode returns "E".
func (u *UnexpectedError) ResultCode() string { return resultCode(u) }

// Message renders the Minitest::UnexpectedError#message:
//
//	"#{error.class}: #{error.message}\n    #{filtered_backtrace.join("\n    ")}"
//
// The host has already filtered the backtrace and stripped the Dir.pwd prefix
// from each frame (those are environment concerns); this method only joins.
func (u *UnexpectedError) Message() string {
	bt := strings.Join(u.Backtrace, "\n    ")
	return u.ErrorClass + ": " + u.ErrorMessage + "\n    " + bt
}

// Error implements error.
func (u *UnexpectedError) Error() string { return u.Message() }

// Reportable2 is the minimal interface a failure exposes for result coding: a
// long label whose first letter is the single-character code. ([Assertion],
// [Skip], [UnexpectedError] all satisfy it.)
type Reportable2 interface {
	ResultLabel() string
	Message() string
	Location() string
}
