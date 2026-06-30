// Copyright (c) the go-ruby-minitest/minitest authors
//
// SPDX-License-Identifier: BSD-3-Clause

package minitest

import "testing"

func TestAssertionBasics(t *testing.T) {
	a := &Assertion{Msg: "boom"}
	if a.Error() != "boom" || a.Message() != "boom" {
		t.Errorf("message/error broken")
	}
	if a.ResultLabel() != "Failure" || a.ResultCode() != "F" {
		t.Errorf("label/code = %s/%s", a.ResultLabel(), a.ResultCode())
	}
	// Empty label yields empty code.
	if resultCode(emptyLabel{}) != "" {
		t.Errorf("empty label code should be empty")
	}
}

type emptyLabel struct{}

func (emptyLabel) ResultLabel() string { return "" }
func (emptyLabel) Message() string     { return "" }
func (emptyLabel) Location() string    { return "" }

func TestLocation(t *testing.T) {
	a := &Assertion{Backtrace: []string{
		"/proj/foo_test.rb:10:in `block in test_x`",
		"/proj/lib/minitest/assertions.rb:5:in `assert`",
		"/proj/foo_test.rb:10:in `test_x`",
	}}
	if got := a.Location(); got != "/proj/foo_test.rb:10" {
		t.Errorf("location = %q", got)
	}
	// No assertion frame: falls back to the last frame.
	b := &Assertion{Backtrace: []string{"a.rb:1:in `block`", "b.rb:2:in `top`"}}
	if got := b.Location(); got != "b.rb:2" {
		t.Errorf("fallback location = %q", got)
	}
	// Empty backtrace: unknown.
	c := &Assertion{}
	if got := c.Location(); got != "unknown:-1" {
		t.Errorf("empty location = %q", got)
	}
	// Owner-prefixed assertion frame ("Foo#assert_bar") is recognized.
	d := &Assertion{Backtrace: []string{
		"x_test.rb:7:in `MyTest#assert_thing`",
		"x_test.rb:7:in `test_z`",
	}}
	if got := d.Location(); got != "x_test.rb:7" {
		t.Errorf("owner-prefixed location = %q", got)
	}
	// single-quote in-frame form.
	e := &Assertion{Backtrace: []string{
		"x.rb:1:in 'assert'",
		"x.rb:2:in 'test_a'",
	}}
	if got := e.Location(); got != "x.rb:2" {
		t.Errorf("single-quote location = %q", got)
	}
}

func TestFrameIsAssertion(t *testing.T) {
	if !frameIsAssertion("x.rb:1:in `assert_equal`") {
		t.Error("assert_equal should match")
	}
	if !frameIsAssertion("x.rb:1:in `must_equal`") {
		t.Error("must_equal should match")
	}
	if frameIsAssertion("x.rb:1:in `helper`") {
		t.Error("helper should not match")
	}
	if frameIsAssertion("no markers here") {
		t.Error("no marker should not match")
	}
}

func TestSkip(t *testing.T) {
	s := &Skip{Assertion{Msg: "later"}}
	if s.ResultLabel() != "Skipped" || s.ResultCode() != "S" {
		t.Errorf("skip label/code = %s/%s", s.ResultLabel(), s.ResultCode())
	}
	if s.Message() != "later" {
		t.Errorf("skip message = %q", s.Message())
	}
}

func TestUnexpectedError(t *testing.T) {
	u := &UnexpectedError{
		Assertion:    Assertion{Backtrace: []string{"foo_test.rb:3:in `test_y`"}},
		ErrorClass:   "RuntimeError",
		ErrorMessage: "kaboom",
	}
	want := "RuntimeError: kaboom\n    foo_test.rb:3:in `test_y`"
	if u.Message() != want {
		t.Errorf("ue message = %q, want %q", u.Message(), want)
	}
	if u.Error() != want {
		t.Errorf("ue error = %q", u.Error())
	}
	if u.ResultLabel() != "Error" || u.ResultCode() != "E" {
		t.Errorf("ue label/code = %s/%s", u.ResultLabel(), u.ResultCode())
	}
}
