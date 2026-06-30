// Copyright (c) the go-ruby-minitest/minitest authors
//
// SPDX-License-Identifier: BSD-3-Clause

package minitest

import (
	"reflect"
	"testing"
)

func TestItName(t *testing.T) {
	if got := ItName(1, "does a thing"); got != "test_0001_does a thing" {
		t.Errorf("it name = %q", got)
	}
	if got := ItName(42, "x"); got != "test_0042_x" {
		t.Errorf("it name 42 = %q", got)
	}
	if DefaultItDesc != "anonymous" {
		t.Errorf("default desc = %q", DefaultItDesc)
	}
}

func TestValidateLetName(t *testing.T) {
	reserved := []string{"name", "setup", "teardown"}
	if got := ValidateLetName("foo", reserved); got != "" {
		t.Errorf("valid name should pass: %q", got)
	}
	wantBegin := "let 'testing' cannot begin with 'test'. Please use another name."
	if got := ValidateLetName("testing", reserved); got != wantBegin {
		t.Errorf("test-prefixed = %q, want %q", got, wantBegin)
	}
	wantOver := "let 'setup' cannot override a method in Minitest::Spec. Please use another name."
	if got := ValidateLetName("setup", reserved); got != wantOver {
		t.Errorf("override = %q, want %q", got, wantOver)
	}
}

func TestSpecType(t *testing.T) {
	// First matching index wins.
	if got := SpecType([]bool{false, true, true}); got != 1 {
		t.Errorf("spec_type = %d, want 1", got)
	}
	// Only the base entry (last) matches.
	if got := SpecType([]bool{false, false, true}); got != 2 {
		t.Errorf("spec_type base = %d, want 2", got)
	}
	// Inconsistent (nothing matches): fall back to last index.
	if got := SpecType([]bool{false, false, false}); got != 2 {
		t.Errorf("spec_type fallback = %d, want 2", got)
	}
}

func TestLookupExpectation(t *testing.T) {
	e, ok := LookupExpectation("must_equal")
	if !ok || e.Assertion != "assert_equal" || e.Flip != FlipDefault {
		t.Errorf("must_equal mapping = %+v ok=%v", e, ok)
	}
	e, _ = LookupExpectation("must_include")
	if e.Assertion != "assert_includes" || e.Flip != FlipReverse {
		t.Errorf("must_include mapping = %+v", e)
	}
	e, _ = LookupExpectation("must_be_nil")
	if e.Assertion != "assert_nil" || e.Flip != FlipUnary {
		t.Errorf("must_be_nil mapping = %+v", e)
	}
	e, _ = LookupExpectation("must_raise")
	if e.Assertion != "assert_raises" || e.Flip != FlipBlock {
		t.Errorf("must_raise mapping = %+v", e)
	}
	e, _ = LookupExpectation("wont_equal")
	if e.Assertion != "refute_equal" {
		t.Errorf("wont_equal mapping = %+v", e)
	}
	if _, ok := LookupExpectation("nope"); ok {
		t.Errorf("unknown should not be found")
	}
}

func TestExpectationsList(t *testing.T) {
	all := Expectations()
	// 34 from expectations.rb + must_verify added by mock.rb.
	if len(all) != 35 {
		t.Errorf("expectation count = %d, want 35", len(all))
	}
	// Mutating the returned slice must not affect the package table.
	all[0].Method = "TAMPERED"
	if again := Expectations(); again[0].Method == "TAMPERED" {
		t.Errorf("Expectations() should return a copy")
	}
}

func TestBindArgs(t *testing.T) {
	// Default flip: ctx.meth(args.first, target, *args[1..-1]).
	e := Expectation{Flip: FlipDefault}
	out, blk := e.BindArgs(rInt(99), []Value{rInt(1), rInt(2)})
	if blk || !reflect.DeepEqual(out, []Value{rInt(1), rInt(99), rInt(2)}) {
		t.Errorf("default bind = %v blk=%v", out, blk)
	}
	// Default flip, no args: nil expected, target as actual.
	out, _ = e.BindArgs(rInt(99), nil)
	if !reflect.DeepEqual(out, []Value{nil, rInt(99)}) {
		t.Errorf("default bind no-args = %v", out)
	}
	// Reverse flip: ctx.meth(target, *args).
	e = Expectation{Flip: FlipReverse}
	out, _ = e.BindArgs(rInt(99), []Value{rInt(1)})
	if !reflect.DeepEqual(out, []Value{rInt(99), rInt(1)}) {
		t.Errorf("reverse bind = %v", out)
	}
	// Unary flip: ctx.meth(target).
	e = Expectation{Flip: FlipUnary}
	out, _ = e.BindArgs(rInt(99), nil)
	if !reflect.DeepEqual(out, []Value{rInt(99)}) {
		t.Errorf("unary bind = %v", out)
	}
	// Block flip: target is the block; args passed through.
	e = Expectation{Flip: FlipBlock}
	out, blk = e.BindArgs(rInt(99), []Value{rClass("ArgumentError")})
	if !blk || !reflect.DeepEqual(out, []Value{rClass("ArgumentError")}) {
		t.Errorf("block bind = %v blk=%v", out, blk)
	}
}
