// Copyright (c) the go-ruby-minitest/minitest authors
//
// SPDX-License-Identifier: BSD-3-Clause

package minitest

import (
	"fmt"
	"strings"
)

// ItName reproduces Minitest::Spec::DSL#it's method-name morphing:
//
//	name = "test_%04d_%s" % [@specs, desc]
//
// where seq is the 1-based count of it/specify blocks defined so far in the
// describe class. The host maintains the counter and the define_method; this
// function owns the exact name format the reporter and filters key off of.
func ItName(seq int, desc string) string {
	return fmt.Sprintf("test_%04d_%s", seq, desc)
}

// DefaultItDesc is the description it uses when none is given ("anonymous"); the
// host also substitutes the skip("(no tests defined)") body when no block is
// passed.
const DefaultItDesc = "anonymous"

// ValidateLetName reproduces Minitest::Spec::DSL#let's name guard. It returns a
// non-empty error message (the exact ArgumentError text) when name is illegal: it
// may not begin with "test", and may not override a Minitest::Spec instance
// method (other than "subject"). reservedMethods is the host-supplied list of
// Minitest::Spec instance-method names (already excluding "subject"), so the
// override check stays interpreter-faithful without this package hardcoding the
// method set.
func ValidateLetName(name string, reservedMethods []string) string {
	const pre = "let '"
	suffixBegin := "' cannot begin with 'test'. Please use another name."
	suffixOver := "' cannot override a method in Minitest::Spec. Please use another name."

	if strings.HasPrefix(name, "test") {
		return pre + name + suffixBegin
	}
	for _, m := range reservedMethods {
		if m == name {
			return pre + name + suffixOver
		}
	}
	return ""
}

// SpecType reproduces Minitest::Spec::DSL#spec_type's selection: it walks the
// registered (matcher, class) pairs newest-first and returns the index of the
// first whose matcher matches desc. A matcher is either a callable (the host
// evaluates it and reports the boolean via callMatch) or a Regexp/=== matcher
// (the host reports the boolean via the same callback). The base registration
// (//, Minitest::Spec) always matches, so a valid index is always returned.
//
// matches[i] is the precomputed boolean of registration i against desc, in the
// SAME order DSL::TYPES holds them (newest unshifted to the front). SpecType
// returns the first true index.
func SpecType(matches []bool) int {
	for i, ok := range matches {
		if ok {
			return i
		}
	}
	// The base [//, Minitest::Spec] entry always matches; reaching here means the
	// host passed an inconsistent table. Fall back to the last entry.
	return len(matches) - 1
}

// Flip describes how an expectation's receiver and arguments map onto the
// underlying assertion's positional parameters (the third arg to
// infect_an_assertion). The zero value [FlipDefault] is the common case.
type Flip int

const (
	// FlipDefault: ctx.meth(args.first, target, *args[1..-1]) — the expectation
	// target becomes the assertion's SECOND positional arg (the "actual"), and
	// the first expectation arg becomes the first (the "expected"). Used by
	// must_equal, must_be_instance_of, etc.
	FlipDefault Flip = iota
	// FlipReverse (:reverse / dont_flip): ctx.meth(target, *args) — target stays
	// first. Used by must_include, must_be, must_respond_to, etc.
	FlipReverse
	// FlipUnary (:unary): like reverse, target first, no extra args. Used by
	// must_be_nil, must_be_empty, etc. (modeled identically to reverse here, as
	// the gem's :unary is also dont_flip=true).
	FlipUnary
	// FlipBlock (:block): ctx.meth(*args, &target) when target is a Proc — the
	// expectation target is the BLOCK. Used by must_raise, must_output, etc.
	FlipBlock
)

// Expectation maps a must_*/wont_* expectation to its underlying assertion. Mapped
// is built from minitest/expectations.rb's infect_an_assertion table.
type Expectation struct {
	// Method is the must_/wont_ name (e.g. "must_equal").
	Method string
	// Assertion is the underlying assert_/refute_ name (e.g. "assert_equal").
	Assertion string
	// Flip is how target/args bind to the assertion's parameters.
	Flip Flip
}

// expectationTable is the full must_*/wont_* → assert_*/refute_* mapping, in the
// order minitest/expectations.rb declares it. It is the spec→assertion contract
// the host dispatches through.
var expectationTable = []Expectation{
	{"must_be_empty", "assert_empty", FlipUnary},
	{"must_equal", "assert_equal", FlipDefault},
	{"must_be_close_to", "assert_in_delta", FlipDefault},
	{"must_be_within_delta", "assert_in_delta", FlipDefault},
	{"must_be_within_epsilon", "assert_in_epsilon", FlipDefault},
	{"must_include", "assert_includes", FlipReverse},
	{"must_be_instance_of", "assert_instance_of", FlipDefault},
	{"must_be_kind_of", "assert_kind_of", FlipDefault},
	{"must_match", "assert_match", FlipDefault},
	{"must_be_nil", "assert_nil", FlipUnary},
	{"must_be", "assert_operator", FlipReverse},
	{"must_output", "assert_output", FlipBlock},
	{"must_pattern_match", "assert_pattern", FlipBlock},
	{"must_raise", "assert_raises", FlipBlock},
	{"must_respond_to", "assert_respond_to", FlipReverse},
	{"must_be_same_as", "assert_same", FlipDefault},
	{"must_be_silent", "assert_silent", FlipBlock},
	{"must_throw", "assert_throws", FlipBlock},
	{"path_must_exist", "assert_path_exists", FlipUnary},
	{"path_wont_exist", "refute_path_exists", FlipUnary},
	{"wont_be_empty", "refute_empty", FlipUnary},
	{"wont_equal", "refute_equal", FlipDefault},
	{"wont_be_close_to", "refute_in_delta", FlipDefault},
	{"wont_be_within_delta", "refute_in_delta", FlipDefault},
	{"wont_be_within_epsilon", "refute_in_epsilon", FlipDefault},
	{"wont_include", "refute_includes", FlipReverse},
	{"wont_be_instance_of", "refute_instance_of", FlipDefault},
	{"wont_be_kind_of", "refute_kind_of", FlipDefault},
	{"wont_match", "refute_match", FlipDefault},
	{"wont_be_nil", "refute_nil", FlipUnary},
	{"wont_be", "refute_operator", FlipReverse},
	{"wont_pattern_match", "refute_pattern", FlipBlock},
	{"wont_respond_to", "refute_respond_to", FlipReverse},
	{"wont_be_same_as", "refute_same", FlipDefault},
	{"must_verify", "assert_mock", FlipUnary},
}

// expectationIndex memoizes the table by method name.
var expectationIndex = func() map[string]Expectation {
	m := make(map[string]Expectation, len(expectationTable))
	for _, e := range expectationTable {
		m[e.Method] = e
	}
	return m
}()

// LookupExpectation returns the [Expectation] mapping for a must_/wont_ method
// name and whether it is known.
func LookupExpectation(method string) (Expectation, bool) {
	e, ok := expectationIndex[method]
	return e, ok
}

// Expectations lists every supported must_*/wont_* expectation in declaration
// order. Hosts iterate it to install the DSL methods.
func Expectations() []Expectation {
	out := make([]Expectation, len(expectationTable))
	copy(out, expectationTable)
	return out
}

// BindArgs computes the positional arguments to pass to the underlying assertion
// for an expectation call, given the expectation target and the expectation's own
// args, per the Flip rule. For [FlipBlock] the target is the block (not a
// positional arg), so the returned args are the expectation args unchanged and
// blockIsTarget is true; the host passes target as the &block. For the other
// flips the returned slice is the full positional argument list.
func (e Expectation) BindArgs(target Value, args []Value) (out []Value, blockIsTarget bool) {
	switch e.Flip {
	case FlipReverse, FlipUnary:
		// ctx.meth(target, *args)
		out = append([]Value{target}, args...)
		return out, false
	case FlipBlock:
		// ctx.meth(*args, &target)
		out = append([]Value{}, args...)
		return out, true
	default: // FlipDefault
		// ctx.meth(args.first, target, *args[1..-1])
		if len(args) == 0 {
			// args.first is nil; matches the gem passing nil expected.
			return []Value{nil, target}, false
		}
		out = append([]Value{args[0], target}, args[1:]...)
		return out, false
	}
}
