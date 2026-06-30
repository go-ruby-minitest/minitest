// Copyright (c) the go-ruby-minitest/minitest authors
//
// SPDX-License-Identifier: BSD-3-Clause

package minitest

import (
	"math"
	"strings"
	"testing"
)

func mathInfPos() float64  { return math.Inf(1) }
func mathInfNeg() float64  { return math.Inf(-1) }
func mathNaN() float64     { return math.NaN() }
func mathNegZero() float64 { return math.Copysign(0, -1) }

func newA() *Assertions { return NewAssertions(fakeRT{}) }

// msgOf returns the failure message of a non-nil assertion error, or "" if nil.
func msgOf(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func TestAssertTruthy(t *testing.T) {
	a := newA()
	if err := a.Assert(rBool(true), ""); err != nil {
		t.Fatalf("assert true should pass: %v", err)
	}
	if a.Count != 1 {
		t.Fatalf("count = %d, want 1", a.Count)
	}
	if got := msgOf(a.Assert(rBool(false), "")); got != "Expected false to be truthy." {
		t.Errorf("assert false msg = %q", got)
	}
	if got := msgOf(a.Assert(rNil, "")); got != "Expected nil to be truthy." {
		t.Errorf("assert nil msg = %q", got)
	}
	if got := msgOf(a.Assert(rBool(false), "boom")); got != "boom" {
		t.Errorf("assert custom msg = %q", got)
	}
}

func TestRefute(t *testing.T) {
	a := newA()
	if err := a.Refute(rBool(false), ""); err != nil {
		t.Fatalf("refute false should pass: %v", err)
	}
	if got := msgOf(a.Refute(rBool(true), "")); got != "Expected true to not be truthy." {
		t.Errorf("refute true msg = %q", got)
	}
	// A custom message replaces the default entirely for refute (msg ||= ...).
	if got := msgOf(a.Refute(rBool(true), "no")); got != "no" {
		t.Errorf("refute custom msg = %q", got)
	}
}

func TestFlunkPassSkip(t *testing.T) {
	a := newA()
	if got := msgOf(a.Flunk("")); got != "Epic Fail!" {
		t.Errorf("flunk default = %q", got)
	}
	if got := msgOf(a.Flunk("nope")); got != "nope" {
		t.Errorf("flunk custom = %q", got)
	}
	if err := a.Pass(); err != nil {
		t.Errorf("pass should be nil: %v", err)
	}
	if s := a.SkipError(""); s.Msg != "Skipped, no message given" {
		t.Errorf("skip default = %q", s.Msg)
	}
	if s := a.SkipError("later"); s.Msg != "later" || s.ResultLabel() != "Skipped" || s.ResultCode() != "S" {
		t.Errorf("skip msg/label/code = %q/%s/%s", s.Msg, s.ResultLabel(), s.ResultCode())
	}
}

func TestAssertEqual(t *testing.T) {
	a := newA()
	err, dep := a.AssertEqual(rInt(1), rInt(1), "")
	if err != nil || dep {
		t.Fatalf("equal pass: err=%v dep=%v", err, dep)
	}
	err, _ = a.AssertEqual(rInt(1), rInt(2), "")
	if got := msgOf(err); got != "Expected: 1\n  Actual: 2" {
		t.Errorf("equal int msg = %q", got)
	}
	err, _ = a.AssertEqual(rStr("a"), rStr("b"), "")
	if got := msgOf(err); got != "Expected: \"a\"\n  Actual: \"b\"" {
		t.Errorf("equal str msg = %q", got)
	}
	err, _ = a.AssertEqual(rInt(1), rInt(2), "oops")
	if got := msgOf(err); got != "oops.\nExpected: 1\n  Actual: 2" {
		t.Errorf("equal custom msg = %q", got)
	}
	// nil exp triggers the 5.x deprecation signal.
	_, dep = a.AssertEqual(rNil, rNil, "")
	if !dep {
		t.Errorf("nil exp should signal deprecated")
	}
}

func TestRefuteEqual(t *testing.T) {
	a := newA()
	if err := a.RefuteEqual(rInt(1), rInt(2), ""); err != nil {
		t.Fatalf("refute_equal distinct should pass: %v", err)
	}
	if got := msgOf(a.RefuteEqual(rInt(1), rInt(1), "")); got != "Expected 1 to not be equal to 1." {
		t.Errorf("refute_equal msg = %q", got)
	}
}

func TestAssertNil(t *testing.T) {
	a := newA()
	if err := a.AssertNil(rNil, ""); err != nil {
		t.Fatalf("assert_nil nil should pass: %v", err)
	}
	if got := msgOf(a.AssertNil(rInt(5), "")); got != "Expected 5 to be nil." {
		t.Errorf("assert_nil msg = %q", got)
	}
	if err := a.RefuteNil(rInt(5), ""); err != nil {
		t.Fatalf("refute_nil 5 should pass: %v", err)
	}
	if got := msgOf(a.RefuteNil(rNil, "")); got != "Expected nil to not be nil." {
		t.Errorf("refute_nil msg = %q", got)
	}
}

func TestAssertEmpty(t *testing.T) {
	a := newA()
	if err := a.AssertEmpty(rArr{}, ""); err != nil {
		t.Fatalf("assert_empty [] should pass: %v", err)
	}
	if got := msgOf(a.AssertEmpty(rArr{rInt(1)}, "")); got != "Expected [1] to be empty." {
		t.Errorf("assert_empty msg = %q", got)
	}
	// non-respond_to value fails the respond_to check first.
	if got := msgOf(a.AssertEmpty(rInt(5), "")); got != "Expected 5 (Integer) to respond to #empty?." {
		t.Errorf("assert_empty respond_to msg = %q", got)
	}
	if err := a.RefuteEmpty(rArr{rInt(1)}, ""); err != nil {
		t.Fatalf("refute_empty [1] should pass: %v", err)
	}
	if got := msgOf(a.RefuteEmpty(rArr{}, "")); got != "Expected [] to not be empty." {
		t.Errorf("refute_empty msg = %q", got)
	}
	if got := msgOf(a.RefuteEmpty(rInt(5), "")); got != "Expected 5 (Integer) to respond to #empty?." {
		t.Errorf("refute_empty respond_to msg = %q", got)
	}
}

func TestAssertIncludes(t *testing.T) {
	a := newA()
	if err := a.AssertIncludes(rArr{rInt(1), rInt(2)}, rInt(1), ""); err != nil {
		t.Fatalf("assert_includes should pass: %v", err)
	}
	if got := msgOf(a.AssertIncludes(rArr{rInt(1), rInt(2)}, rInt(3), "")); got != "Expected [1, 2] to include 3." {
		t.Errorf("assert_includes msg = %q", got)
	}
	if got := msgOf(a.AssertIncludes(rInt(5), rInt(3), "")); got != "Expected 5 (Integer) to respond to #include?." {
		t.Errorf("assert_includes respond_to msg = %q", got)
	}
	if err := a.RefuteIncludes(rArr{rInt(1)}, rInt(3), ""); err != nil {
		t.Fatalf("refute_includes should pass: %v", err)
	}
	if got := msgOf(a.RefuteIncludes(rArr{rInt(1), rInt(2)}, rInt(2), "")); got != "Expected [1, 2] to not include 2." {
		t.Errorf("refute_includes msg = %q", got)
	}
	if got := msgOf(a.RefuteIncludes(rInt(5), rInt(3), "")); got != "Expected 5 (Integer) to respond to #include?." {
		t.Errorf("refute_includes respond_to msg = %q", got)
	}
}

func TestAssertInstanceKindOf(t *testing.T) {
	a := newA()
	if err := a.AssertInstanceOf(rClass("Integer"), rInt(5), ""); err != nil {
		t.Fatalf("assert_instance_of should pass: %v", err)
	}
	if got := msgOf(a.AssertInstanceOf(rClass("String"), rInt(5), "")); got != "Expected 5 to be an instance of String, not Integer." {
		t.Errorf("assert_instance_of msg = %q", got)
	}
	if err := a.RefuteInstanceOf(rClass("String"), rInt(5), ""); err != nil {
		t.Fatalf("refute_instance_of should pass: %v", err)
	}
	if got := msgOf(a.RefuteInstanceOf(rClass("Integer"), rInt(5), "")); got != "Expected 5 to not be an instance of Integer." {
		t.Errorf("refute_instance_of msg = %q", got)
	}
	if err := a.AssertKindOf(rClass("Numeric"), rInt(5), ""); err != nil {
		t.Fatalf("assert_kind_of should pass: %v", err)
	}
	if got := msgOf(a.AssertKindOf(rClass("String"), rInt(5), "")); got != "Expected 5 to be a kind of String, not Integer." {
		t.Errorf("assert_kind_of msg = %q", got)
	}
	if err := a.RefuteKindOf(rClass("String"), rInt(5), ""); err != nil {
		t.Fatalf("refute_kind_of should pass: %v", err)
	}
	if got := msgOf(a.RefuteKindOf(rClass("Numeric"), rInt(5), "")); got != "Expected 5 to not be a kind of Numeric." {
		t.Errorf("refute_kind_of msg = %q", got)
	}
}

func TestAssertRespondTo(t *testing.T) {
	a := newA()
	if err := a.AssertRespondTo(rArr{}, "include?", "", false); err != nil {
		t.Fatalf("assert_respond_to should pass: %v", err)
	}
	if got := msgOf(a.AssertRespondTo(rInt(5), "foo", "", false)); got != "Expected 5 (Integer) to respond to #foo." {
		t.Errorf("assert_respond_to msg = %q", got)
	}
	if err := a.RefuteRespondTo(rInt(5), "foo", "", false); err != nil {
		t.Fatalf("refute_respond_to should pass: %v", err)
	}
	if got := msgOf(a.RefuteRespondTo(rInt(5), "to_s", "", false)); got != "Expected 5 to not respond to to_s." {
		t.Errorf("refute_respond_to msg = %q", got)
	}
}

func TestAssertMatch(t *testing.T) {
	a := newA()
	if err := a.AssertMatch(rReg{src: "x"}, rStr("xy"), ""); err != nil {
		t.Fatalf("assert_match should pass: %v", err)
	}
	if got := msgOf(a.AssertMatch(rReg{src: "x"}, rStr("y"), "")); got != "Expected /x/ to match \"y\"." {
		t.Errorf("assert_match msg = %q", got)
	}
	// String matcher is promoted; the message shows the regexp form.
	if got := msgOf(a.AssertMatch(rStr("x"), rStr("y"), "")); got != "Expected /x/ to match \"y\"." {
		t.Errorf("assert_match string-promoted msg = %q", got)
	}
	// matcher not responding to =~ fails the respond_to check.
	if got := msgOf(a.AssertMatch(rInt(5), rStr("y"), "")); got != "Expected 5 (Integer) to respond to #=~." {
		t.Errorf("assert_match respond_to msg = %q", got)
	}
	if err := a.RefuteMatch(rReg{src: "z"}, rStr("y"), ""); err != nil {
		t.Fatalf("refute_match should pass: %v", err)
	}
	if got := msgOf(a.RefuteMatch(rReg{src: "y"}, rStr("y"), "")); got != "Expected /y/ to not match \"y\"." {
		t.Errorf("refute_match msg = %q", got)
	}
	if got := msgOf(a.RefuteMatch(rStr("y"), rStr("y"), "")); got != "Expected /y/ to not match \"y\"." {
		t.Errorf("refute_match string-promoted msg = %q", got)
	}
	if got := msgOf(a.RefuteMatch(rInt(5), rStr("y"), "")); got != "Expected 5 (Integer) to respond to #=~." {
		t.Errorf("refute_match respond_to msg = %q", got)
	}
}

func TestAssertSame(t *testing.T) {
	a := newA()
	o := rObj{id: 10, class: "Foo", insp: "#<Foo>"}
	if err := a.AssertSame(o, o, ""); err != nil {
		t.Fatalf("assert_same identical should pass: %v", err)
	}
	o2 := rObj{id: 20, class: "Foo", insp: "#<Foo>"}
	if got := msgOf(a.AssertSame(o, o2, "")); got != "Expected #<Foo> (oid=20) to be the same as #<Foo> (oid=10)." {
		t.Errorf("assert_same msg = %q", got)
	}
	if err := a.RefuteSame(o, o2, ""); err != nil {
		t.Fatalf("refute_same distinct should pass: %v", err)
	}
	if got := msgOf(a.RefuteSame(o, o, "")); got != "Expected #<Foo> (oid=10) to not be the same as #<Foo> (oid=10)." {
		t.Errorf("refute_same msg = %q", got)
	}
}

func TestAssertInDeltaEpsilon(t *testing.T) {
	a := newA()
	if err := a.AssertInDelta(1.0, 1.05, 0.1, ""); err != nil {
		t.Fatalf("assert_in_delta should pass: %v", err)
	}
	if got := msgOf(a.AssertInDelta(1.0, 2.0, 0.1, "")); got != "Expected |1.0 - 2.0| (1.0) to be <= 0.1." {
		t.Errorf("assert_in_delta msg = %q", got)
	}
	if got := msgOf(a.AssertInEpsilon(1.0, 2.0, 0.1, "")); got != "Expected |1.0 - 2.0| (1.0) to be <= 0.1." {
		t.Errorf("assert_in_epsilon msg = %q", got)
	}
	if err := a.RefuteInDelta(1.0, 2.0, 0.1, ""); err != nil {
		t.Fatalf("refute_in_delta should pass: %v", err)
	}
	if got := msgOf(a.RefuteInDelta(1.0, 1.0, 0.1, "")); got != "Expected |1.0 - 1.0| (0.0) to not be <= 0.1." {
		t.Errorf("refute_in_delta msg = %q", got)
	}
	if got := msgOf(a.RefuteInEpsilon(1.0, 1.0, 0.1, "")); got != "Expected |1.0 - 1.0| (0.0) to not be <= 0.1." {
		t.Errorf("refute_in_epsilon msg = %q", got)
	}
}

func TestAssertOperatorPredicate(t *testing.T) {
	a := newA()
	if err := a.AssertOperator(rInt(5), "<", rInt(6), ""); err != nil {
		t.Fatalf("assert_operator should pass: %v", err)
	}
	if got := msgOf(a.AssertOperator(rInt(5), "<", rInt(4), "")); got != "Expected 5 to be < 4." {
		t.Errorf("assert_operator msg = %q", got)
	}
	// 2-arg form delegates to predicate.
	if got := msgOf(a.AssertOperator(rStr("x"), "empty?", UNDEFINED, "")); got != "Expected \"x\" to be empty?." {
		t.Errorf("assert_operator->predicate msg = %q", got)
	}
	if err := a.AssertPredicate(rStr(""), "empty?", ""); err != nil {
		t.Fatalf("assert_predicate empty should pass: %v", err)
	}
	if got := msgOf(a.AssertPredicate(rStr("x"), "empty?", "")); got != "Expected \"x\" to be empty?." {
		t.Errorf("assert_predicate msg = %q", got)
	}
	if err := a.RefuteOperator(rInt(5), "<", rInt(4), ""); err != nil {
		t.Fatalf("refute_operator should pass: %v", err)
	}
	if got := msgOf(a.RefuteOperator(rInt(5), "<", rInt(6), "")); got != "Expected 5 to not be < 6." {
		t.Errorf("refute_operator msg = %q", got)
	}
	if got := msgOf(a.RefuteOperator(rStr(""), "empty?", UNDEFINED, "")); got != "Expected \"\" to not be empty?." {
		t.Errorf("refute_operator->predicate msg = %q", got)
	}
	if err := a.RefutePredicate(rStr("x"), "empty?", ""); err != nil {
		t.Fatalf("refute_predicate should pass: %v", err)
	}
	if got := msgOf(a.RefutePredicate(rStr(""), "empty?", "")); got != "Expected \"\" to not be empty?." {
		t.Errorf("refute_predicate msg = %q", got)
	}
}

func TestUNDEFINEDInspect(t *testing.T) {
	if UNDEFINED.Inspect() != "UNDEFINED" {
		t.Errorf("UNDEFINED.Inspect = %q", UNDEFINED.Inspect())
	}
}

func TestFloatRender(t *testing.T) {
	cases := map[float64]string{
		2.0:               "2.0",
		3.14:              "3.14",
		0.0:               "0.0",
		mathInfPos():      "Infinity",
		mathInfNeg():      "-Infinity",
		mathNaN():         "NaN",
		1234567.5:         "1234567.5",
		0.0001:            "0.0001",
		0.00001:           "1.0e-05",
		123456789012345.0: "123456789012345.0",
		1e15:              "1.0e+15",
		1.5e20:            "1.5e+20",
		mathNegZero():     "-0.0",
	}
	for in, want := range cases {
		if got := f(in); got != want {
			t.Errorf("f(%v) = %q, want %q", in, got, want)
		}
	}
}

func TestBlockRequires(t *testing.T) {
	a := newA()
	if got := msgOf(a.OutputRequiresBlock()); got != "assert_output requires a block to capture output." {
		t.Errorf("output block msg = %q", got)
	}
	if got := msgOf(a.RaisesRequiresBlock()); got != "assert_raises requires a block to capture errors." {
		t.Errorf("raises block msg = %q", got)
	}
}

func TestAssertOutput(t *testing.T) {
	a := newA()
	if err := a.AssertOutput(nil, nil); err != nil {
		t.Fatalf("both nil should pass: %v", err)
	}
	stderrErr := fail("In stderr.\nExpected: \"\"\n  Actual: \"x\"")
	if err := a.AssertOutput(stderrErr, nil); err != stderrErr {
		t.Errorf("stderr failure should surface first")
	}
	stdoutErr := fail("In stdout")
	if err := a.AssertOutput(nil, stdoutErr); err != stdoutErr {
		t.Errorf("stdout failure should surface")
	}
}

func TestMessageHelper(t *testing.T) {
	// Custom message chomps a single trailing "." then appends ".\n".
	got := message("hi.", "", func() string { return "DEF" })
	if got != "hi.\nDEF." {
		t.Errorf("message = %q", got)
	}
	// E ending yields no trailing period.
	got = message("", E, func() string { return "DEF" })
	if got != "DEF" {
		t.Errorf("message E = %q", got)
	}
	if !strings.HasSuffix(message("", "", func() string { return "X" }), "X.") {
		t.Errorf("default ending should be .")
	}
}
