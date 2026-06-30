// Copyright (c) the go-ruby-minitest/minitest authors
//
// SPDX-License-Identifier: BSD-3-Clause

// This file pins the residual branches that the behavioral tests don't reach, so
// the package holds at 100% statement coverage including error/edge paths.

package minitest

import "testing"

func TestCallsInspectWithKwargs(t *testing.T) {
	// An undercalled method whose actual call carried kwargs exercises the
	// callsInspect kwargs branch and callInspect's kwargs-after-args joining.
	m := NewMock(newFakeMatcher())
	m.Expect("foo", rInt(1), []Value{rInt(7)}, []KV{{"a", rInt(5)}}, false)
	m.Expect("foo", rInt(2), []Value{rInt(8)}, []KV{{"b", rInt(6)}}, false)
	if _, err := m.Call("foo", []Value{rInt(7)}, []KV{{"a", rInt(5)}}); err != nil {
		t.Fatalf("first call: %v", err)
	}
	got := errMsg(m.Verify())
	want := "Expected foo(8, b: 6) => 2, got [foo(7, a: 5) => 1]"
	if got != want {
		t.Errorf("verify = %q, want %q", got, want)
	}
}

func TestCallInspectKwargsOnly(t *testing.T) {
	// expected[0] has kwargs but no positional args: the "args empty, append
	// kwargs" branch of callInspect.
	m := NewMock(newFakeMatcher())
	m.Expect("foo", rInt(1), nil, []KV{{"a", rInt(5)}}, false)
	if got := errMsg(m.Verify()); got != "Expected foo(a: 5) => 1" {
		t.Errorf("kwargs-only verify = %q", got)
	}
}

func TestSameKeysDiffLen(t *testing.T) {
	if sameKeys([]KV{{"a", nil}}, nil) {
		t.Error("different lengths should not be equal")
	}
	if sameKeys([]KV{{"a", nil}}, []KV{{"b", nil}}) {
		t.Error("different keys should not be equal")
	}
	if !sameKeys([]KV{{"a", nil}, {"b", nil}}, []KV{{"b", nil}, {"a", nil}}) {
		t.Error("same key sets (any order) should be equal")
	}
}

func TestLookupKVMissing(t *testing.T) {
	if lookupKV([]KV{{"a", rInt(1)}}, "z") != nil {
		t.Error("missing key should yield nil")
	}
}

func TestJoinNLEmpty(t *testing.T) {
	if joinNL(nil) != "" {
		t.Error("empty joinNL should be empty")
	}
	if joinNL([]string{"a", "b"}) != "a\nb" {
		t.Error("joinNL two")
	}
}

// plainFail is a Reportable2 that is neither Skip/UnexpectedError/Assertion, to
// drive Result.ResultCode's default branch.
type plainFail struct{}

func (plainFail) ResultLabel() string { return "Custom" }
func (plainFail) Message() string     { return "custom" }
func (plainFail) Location() string    { return "x:1" }

func TestResultCodeDefaultBranch(t *testing.T) {
	r := &Result{Klass: "T", TestName: "t", Failures: []Reportable2{plainFail{}}}
	if r.ResultCode() != "C" {
		t.Errorf("default-branch code = %q, want C", r.ResultCode())
	}
}

func TestRunTestSetupPassthrough(t *testing.T) {
	// A passthrough during a SETUP hook aborts before the body and teardown.
	p := &Passthrough{Err: errString("SIGTERM")}
	b := &fakeBody{name: "test_x", class: "T", results: map[string]error{"before_setup": p}}
	_, abort := RunTest(b, 0)
	if abort == nil || abort.Error() != "SIGTERM" {
		t.Fatalf("expected setup passthrough abort, got %v", abort)
	}
	for _, c := range b.calls {
		if c == "test_x" || c == "teardown" {
			t.Errorf("body/teardown ran after setup passthrough: %v", b.calls)
		}
	}
}

func TestCallsInspectKwargsOnly(t *testing.T) {
	// An actual recorded call with kwargs but no positional args drives the
	// "args empty, append kwargs" else-branch of callsInspect.
	m := NewMock(newFakeMatcher())
	m.Expect("foo", rInt(1), nil, []KV{{"a", rInt(5)}}, false)
	m.Expect("foo", rInt(2), nil, []KV{{"b", rInt(6)}}, false)
	if _, err := m.Call("foo", nil, []KV{{"a", rInt(5)}}); err != nil {
		t.Fatalf("first call: %v", err)
	}
	got := errMsg(m.Verify())
	want := "Expected foo(b: 6) => 2, got [foo(a: 5) => 1]"
	if got != want {
		t.Errorf("verify = %q, want %q", got, want)
	}
}

func TestMuPPForDiffDoubleEscapeUnescape(t *testing.T) {
	// A string whose inspect carries ONLY double-escaped "\\n" (not at the very
	// start, so it is classified double) hits the double-unescape replace branch.
	a := NewAssertions(inspRT{insp: `x\\ny`})
	got := a.MuPPForDiff(rStr("x"))
	// Each "\\n" → "\n" + real newline.
	if got != "x\\n\ny" {
		t.Errorf("double-unescape = %q", got)
	}
}

func TestMuPPForDiffHexInString(t *testing.T) {
	// A single-escaped \n AND a hex tail together: unescape then anonymize.
	a := NewAssertions(inspRT{insp: `#<X:0x00007fff arr="a\nb">`})
	got := a.MuPPForDiff(rStr("x"))
	want := "#<X:0xXXXXXX arr=\"a\nb\">"
	if got != want {
		t.Errorf("combo = %q, want %q", got, want)
	}
}
