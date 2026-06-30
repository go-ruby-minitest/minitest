// Copyright (c) the go-ruby-minitest/minitest authors
//
// SPDX-License-Identifier: BSD-3-Clause

package minitest

import "testing"

func TestAssertRaisesMatched(t *testing.T) {
	a := newA()
	out := RaiseOutcome{Raised: true, Matched: true, ErrClass: "ArgumentError"}
	reRaise, err := a.AssertRaises(out, "", "[ArgumentError]", "ArgumentError")
	if reRaise || err != nil {
		t.Errorf("matched should pass: reRaise=%v err=%v", reRaise, err)
	}
	if a.Count != 1 {
		t.Errorf("matched should count one assertion, got %d", a.Count)
	}
}

func TestAssertRaisesNothing(t *testing.T) {
	a := newA()
	// size==1: shows exp.first inspect.
	_, err := a.AssertRaises(RaiseOutcome{}, "", "[ArgumentError]", "ArgumentError")
	if msgOf(err) != "ArgumentError expected but nothing was raised." {
		t.Errorf("nothing single = %q", msgOf(err))
	}
	// multi: shows the array inspect.
	_, err = a.AssertRaises(RaiseOutcome{}, "", "[ArgumentError, TypeError]", "")
	if msgOf(err) != "[ArgumentError, TypeError] expected but nothing was raised." {
		t.Errorf("nothing multi = %q", msgOf(err))
	}
	// with custom message prefix.
	_, err = a.AssertRaises(RaiseOutcome{}, "custom", "[ArgumentError]", "ArgumentError")
	if msgOf(err) != "custom.\nArgumentError expected but nothing was raised." {
		t.Errorf("nothing custom = %q", msgOf(err))
	}
}

func TestAssertRaisesWrong(t *testing.T) {
	a := newA()
	out := RaiseOutcome{
		Raised:     true,
		ErrClass:   "TypeError",
		ErrMessage: "boom",
		Backtrace:  []string{"-e:7:in 'block (2 levels) in <main>'"},
	}
	_, err := a.AssertRaises(out, "", "[ArgumentError]", "ArgumentError")
	want := "[ArgumentError] exception expected, not\n" +
		"Class: <TypeError>\n" +
		"Message: <\"boom\">\n" +
		"---Backtrace---\n" +
		"-e:7:in 'block (2 levels) in <main>'\n" +
		"---------------"
	if msgOf(err) != want {
		t.Errorf("wrong exception = %q\nwant %q", msgOf(err), want)
	}
}

func TestAssertRaisesPassthroughAndAssertion(t *testing.T) {
	a := newA()
	// A Minitest::Assertion raised inside the block re-raises (no count).
	reRaise, err := a.AssertRaises(RaiseOutcome{Raised: true, IsAssertion: true}, "", "[E]", "E")
	if !reRaise || err != nil {
		t.Errorf("assertion should re-raise: %v %v", reRaise, err)
	}
	// SignalException / SystemExit re-raise too.
	reRaise, err = a.AssertRaises(RaiseOutcome{Raised: true, IsPassthrough: true}, "", "[E]", "E")
	if !reRaise || err != nil {
		t.Errorf("passthrough should re-raise: %v %v", reRaise, err)
	}
}

func TestAssertThrows(t *testing.T) {
	a := newA()
	if err := a.AssertThrows(ThrowOutcome{Caught: true}, ":foo", ""); err != nil {
		t.Errorf("caught should pass: %v", err)
	}
	if got := msgOf(a.AssertThrows(ThrowOutcome{}, ":foo", "")); got != "Expected :foo to have been thrown." {
		t.Errorf("not thrown = %q", got)
	}
	// A different symbol was thrown: the ", not ..." tail.
	got := msgOf(a.AssertThrows(ThrowOutcome{OtherSym: ":bar"}, ":foo", ""))
	if got != "Expected :foo to have been thrown, not :bar." {
		t.Errorf("other sym = %q", got)
	}
}

func TestInspectString(t *testing.T) {
	cases := map[string]string{
		"boom": `"boom"`,
		"a\nb": `"a\nb"`,
		"a\tb": `"a\tb"`,
		"a\"b": `"a\"b"`,
		"a\\b": `"a\\b"`,
		"a\rb": `"a\rb"`,
		"":     `""`,
	}
	for in, want := range cases {
		if got := inspectString(in); got != want {
			t.Errorf("inspectString(%q) = %q, want %q", in, got, want)
		}
	}
}
