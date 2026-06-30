// Copyright (c) the go-ruby-minitest/minitest authors
//
// SPDX-License-Identifier: BSD-3-Clause

package minitest

import (
	"testing"
)

// fakeBody is a programmable TestBody: results maps a hook/body name to the error
// it should "raise" (nil = success). calls records the invocation order.
type fakeBody struct {
	name    string
	class   string
	results map[string]error
	asserts int
	srcFile string
	srcLine int
	calls   []string
}

func (b *fakeBody) Invoke(method string) error {
	b.calls = append(b.calls, method)
	if b.results != nil {
		return b.results[method]
	}
	return nil
}
func (b *fakeBody) Name() string      { return b.name }
func (b *fakeBody) ClassName() string { return b.class }
func (b *fakeBody) Assertions() int   { return b.asserts }
func (b *fakeBody) SourceLocation() (string, int) {
	return b.srcFile, b.srcLine
}

func TestRunTestPass(t *testing.T) {
	b := &fakeBody{name: "test_x", class: "MyTest", asserts: 1, srcFile: "x.rb", srcLine: 3}
	res, abort := RunTest(b, 0.5)
	if abort != nil {
		t.Fatalf("unexpected abort: %v", abort)
	}
	if !res.Passed() || res.Skipped() || res.Errored() {
		t.Errorf("pass predicates wrong: %+v", res)
	}
	if res.ResultCode() != "." || res.Assertions != 1 || res.Time != 0.5 {
		t.Errorf("code/assertions/time wrong: %s/%d/%v", res.ResultCode(), res.Assertions, res.Time)
	}
	if res.SourceFile != "x.rb" || res.SourceLine != 3 {
		t.Errorf("source loc = %s:%d", res.SourceFile, res.SourceLine)
	}
	// Full lifecycle ran: 3 setup + body + 3 teardown.
	want := []string{"before_setup", "setup", "after_setup", "test_x",
		"before_teardown", "teardown", "after_teardown"}
	if len(b.calls) != len(want) {
		t.Fatalf("calls = %v", b.calls)
	}
	for i := range want {
		if b.calls[i] != want[i] {
			t.Errorf("call %d = %q, want %q", i, b.calls[i], want[i])
		}
	}
	if res.Location("") != "MyTest#test_x" {
		t.Errorf("location = %q", res.Location(""))
	}
	if res.String("") != "MyTest#test_x" {
		t.Errorf("to_s = %q", res.String(""))
	}
}

func TestRunTestFailure(t *testing.T) {
	f := &Assertion{Msg: "Expected: 1\n  Actual: 2", Backtrace: []string{"/proj/x_test.rb:5:in `test_x`"}}
	b := &fakeBody{
		name: "test_x", class: "MyTest", asserts: 1,
		results: map[string]error{"test_x": f},
	}
	res, _ := RunTest(b, 0)
	if res.Passed() || res.ResultCode() != "F" {
		t.Errorf("failure predicates wrong")
	}
	if got := res.Location("/proj/"); got != "MyTest#test_x [x_test.rb:5]" {
		t.Errorf("location = %q", got)
	}
	want := "Failure:\nMyTest#test_x [x_test.rb:5]:\nExpected: 1\n  Actual: 2\n"
	if got := res.String("/proj/"); got != want {
		t.Errorf("to_s = %q, want %q", got, want)
	}
	// Teardown still ran after the body failed.
	if b.calls[len(b.calls)-1] != "after_teardown" {
		t.Errorf("teardown should run after failure: %v", b.calls)
	}
}

func TestRunTestSkip(t *testing.T) {
	s := &Skip{Assertion{Msg: "nope", Backtrace: []string{"x.rb:1:in `test_x`"}}}
	b := &fakeBody{name: "test_x", class: "MyTest", results: map[string]error{"test_x": s}}
	res, _ := RunTest(b, 0)
	if !res.Skipped() || res.Passed() || res.ResultCode() != "S" {
		t.Errorf("skip predicates wrong: %+v", res)
	}
	if !contains(res.String(""), "Skipped:") {
		t.Errorf("to_s should mention Skipped: %q", res.String(""))
	}
}

func TestRunTestError(t *testing.T) {
	e := &UnexpectedError{
		Assertion:    Assertion{Backtrace: []string{"x.rb:1:in `test_x`"}},
		ErrorClass:   "RuntimeError",
		ErrorMessage: "boom",
	}
	b := &fakeBody{name: "test_x", class: "MyTest", results: map[string]error{"test_x": e}}
	res, _ := RunTest(b, 0)
	if !res.Errored() || res.Passed() || res.ResultCode() != "E" {
		t.Errorf("error predicates wrong")
	}
	// An errored run's location omits the [file:line] suffix.
	if res.Location("") != "MyTest#test_x" {
		t.Errorf("errored location = %q", res.Location(""))
	}
}

func TestRunTestSetupFailureSkipsBody(t *testing.T) {
	f := &Assertion{Msg: "setup boom"}
	b := &fakeBody{name: "test_x", class: "T", results: map[string]error{"setup": f}}
	res, _ := RunTest(b, 0)
	if res.Passed() {
		t.Errorf("should fail")
	}
	// Body must NOT have run (setup+body share one capture_exceptions).
	for _, c := range b.calls {
		if c == "test_x" {
			t.Errorf("body ran despite setup failure: %v", b.calls)
		}
	}
	// Teardown still ran.
	if b.calls[len(b.calls)-1] != "after_teardown" {
		t.Errorf("teardown missing: %v", b.calls)
	}
}

func TestRunTestPassthrough(t *testing.T) {
	p := &Passthrough{Err: &Assertion{Msg: "SIGINT"}}
	b := &fakeBody{name: "test_x", class: "T", results: map[string]error{"test_x": p}}
	res, abort := RunTest(b, 0)
	if abort == nil {
		t.Fatalf("expected abort")
	}
	if abort.Error() != "SIGINT" {
		t.Errorf("abort error = %q", abort.Error())
	}
	// Teardown must be skipped on a passthrough abort.
	for _, c := range b.calls {
		if c == "teardown" {
			t.Errorf("teardown ran on passthrough: %v", b.calls)
		}
	}
	_ = res
}

func TestPassthroughNilErr(t *testing.T) {
	p := &Passthrough{}
	if p.Error() != "passthrough" {
		t.Errorf("nil passthrough = %q", p.Error())
	}
}

func TestAsReportableFallback(t *testing.T) {
	r := asReportable(errString("weird"))
	ue, ok := r.(*UnexpectedError)
	if !ok || ue.ErrorMessage != "weird" {
		t.Errorf("fallback = %#v", r)
	}
}

type errString string

func (e errString) Error() string { return string(e) }

func TestResultEmptyFailure(t *testing.T) {
	r := &Result{Klass: "T", TestName: "t"}
	if r.Failure() != nil {
		t.Errorf("empty failure should be nil")
	}
	if r.ResultCode() != "." {
		t.Errorf("empty code = %q", r.ResultCode())
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
