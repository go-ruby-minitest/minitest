// Copyright (c) the go-ruby-minitest/minitest authors
//
// SPDX-License-Identifier: BSD-3-Clause

package minitest

import (
	"fmt"
	"sort"
	"strings"
)

// MockExpectationError is Minitest's MockExpectationError, raised by [Mock] when
// it is not called as expected (verify, no-more-expects, argument mismatch).
type MockExpectationError struct{ Msg string }

// Error implements error.
func (e *MockExpectationError) Error() string { return e.Msg }

// MockArgumentError is the Ruby ArgumentError [Mock] raises on an arity mismatch.
// It is distinct from MockExpectationError because the gem raises ArgumentError
// (not MockExpectationError) for wrong argument/keyword counts.
type MockArgumentError struct{ Msg string }

// Error implements error.
func (e *MockArgumentError) Error() string { return e.Msg }

// MockNoMethodError is the Ruby NoMethodError [Mock] raises for an unmocked,
// undelegated method.
type MockNoMethodError struct{ Msg string }

// Error implements error.
func (e *MockNoMethodError) Error() string { return e.Msg }

// expectation is one queued expect: either a value-matched call (Args/Kwargs vs
// the expected, returning Retval) or a block-validated call (Block true). The
// Args/Kwargs hold the EXPECTED argument matchers as opaque [Value]s; matching
// against actual args is delegated to the [Runtime] (=== / ==), which the host
// supplies, since case-equality is Ruby semantics.
type expectation struct {
	Retval Value
	Args   []Value
	Kwargs []KV // expected keyword matchers, in declared order
	Block  bool // a block was given to expect instead of args/kwargs
	// KwargsAnyClass is the gem's `Hash == expected_kwargs` shape: when expect was
	// given Hash (the class) as kwargs, every actual keyword matches Object.
	KwargsAnyClass bool
}

// KV is a keyword key/value pair preserving declaration order.
type KV struct {
	Key string
	Val Value
}

// actualCall records a satisfied call for verify accounting.
type actualCall struct {
	Retval Value
	Args   []Value
	Kwargs []KV
}

// MockMatcher is the seam for Ruby case-equality used during mock argument
// matching: Match reports (expected === actual) || (expected == actual), and
// Inspect renders a [Value] for the error messages (Ruby #inspect). It is a
// narrow slice of [Runtime]; a host can reuse the same implementation.
type MockMatcher interface {
	// Match reports whether expected matches actual under Ruby case-equality.
	Match(expected, actual Value) bool
	// Inspect renders v as Ruby #inspect would (for error messages).
	Inspect(v Value) string
	// CallBlock invokes the block given to expect with the actual args/kwargs and
	// reports its truthiness (the gem's val_block.call(*args, **kwargs, &block)).
	CallBlock(idx int, args []Value, kwargs []KV) bool
}

// Mock is Minitest::Mock: a clean mock object. expect queues expectations; the
// host routes method calls into [Mock.Call]; verify checks all queued
// expectations were met. Argument case-equality and block invocation are
// delegated to a [MockMatcher].
type Mock struct {
	matcher  MockMatcher
	expected map[string][]expectation
	actual   map[string][]actualCall
	// order preserves the first-seen order of expected method names so verify
	// reports deterministically (the gem iterates @expected_calls insertion
	// order; Go maps don't, so we track it).
	order []string
}

// NewMock returns a Mock backed by matcher.
func NewMock(matcher MockMatcher) *Mock {
	return &Mock{
		matcher:  matcher,
		expected: map[string][]expectation{},
		actual:   map[string][]actualCall{},
	}
}

// Expect queues an expectation for name with return value retval and expected
// positional args / keyword matchers. It mirrors Mock#expect's argument
// validation, returning the ArgumentError the gem raises when args is misused.
// Pass block=true (with empty args/kwargs) for the block-validated form.
func (m *Mock) Expect(name string, retval Value, args []Value, kwargs []KV, block bool) error {
	if block {
		if len(args) != 0 {
			return &MockArgumentError{"args ignored when block given"}
		}
		if len(kwargs) != 0 {
			return &MockArgumentError{"kwargs ignored when block given"}
		}
		m.push(name, expectation{Retval: retval, Block: true})
		return nil
	}
	// args must be an array — modeled as: a nil slice is allowed (empty), but the
	// host signals a non-array by passing the sentinel; we accept any []Value.
	m.push(name, expectation{Retval: retval, Args: args, Kwargs: kwargs})
	return nil
}

// ExpectAnyKwargs queues an expectation whose keyword matching accepts any
// keywords (the gem's `Hash == expected_kwargs` branch, set when expect is given
// the Hash class itself as the kwargs matcher).
func (m *Mock) ExpectAnyKwargs(name string, retval Value, args []Value) {
	m.push(name, expectation{Retval: retval, Args: args, KwargsAnyClass: true})
}

func (m *Mock) push(name string, e expectation) {
	if _, seen := m.expected[name]; !seen {
		m.order = append(m.order, name)
	}
	m.expected[name] = append(m.expected[name], e)
}

// Call routes a method invocation into the mock, reproducing Mock#method_missing.
// It returns the expectation's retval on success, or an error: [*MockNoMethodError]
// for an unmocked name, [*MockExpectationError] for no-more-expects / wrong args /
// failed block / wrong keywords, or [*MockArgumentError] for an arity mismatch.
func (m *Mock) Call(name string, args []Value, kwargs []KV) (Value, error) {
	exps, ok := m.expected[name]
	if !ok {
		// unmocked method (delegation is a host concern handled before Call).
		keys := append([]string{}, m.order...)
		sort.Strings(keys)
		return nil, &MockNoMethodError{fmt.Sprintf("unmocked method %s, expected one of %s",
			symInspect(name), symListInspect(keys))}
	}

	index := len(m.actual[name])
	if index >= len(exps) {
		return nil, &MockExpectationError{fmt.Sprintf("No more expects available for %s: %s %s",
			symInspect(name), arrInspect(m.matcher, args), kwargsInspect(m.matcher, kwargs))}
	}
	e := exps[index]

	if e.Block {
		// keep verify happy: record the call.
		m.actual[name] = append(m.actual[name], actualCall{Retval: e.Retval})
		if !m.matcher.CallBlock(index, args, kwargs) {
			return nil, &MockExpectationError{fmt.Sprintf("mocked method %s failed block w/ %s %s",
				symInspect(name), arrInspect(m.matcher, args), kwargsInspect(m.matcher, kwargs))}
		}
		return e.Retval, nil
	}

	if len(e.Args) != len(args) {
		return nil, &MockArgumentError{fmt.Sprintf("mocked method %s expects %d arguments, got %s",
			symInspect(name), len(e.Args), arrInspect(m.matcher, args))}
	}

	expKwargs := e.Kwargs
	if e.KwargsAnyClass {
		// expected_kwargs = kwargs.to_h { |ak, av| [ak, Object] }
		expKwargs = make([]KV, len(kwargs))
		for i, kv := range kwargs {
			expKwargs[i] = KV{Key: kv.Key, Val: anyClass{}}
		}
	}

	if len(expKwargs) != len(kwargs) {
		return nil, &MockExpectationError{fmt.Sprintf("mocked method %s expects %d keyword arguments, got %s",
			symInspect(name), len(expKwargs), arrInspect(m.matcher, args))}
	}

	for i := range e.Args {
		if !m.matcher.Match(e.Args[i], args[i]) {
			return nil, &MockExpectationError{fmt.Sprintf("mocked method %s called with unexpected arguments %s",
				symInspect(name), arrInspect(m.matcher, args))}
		}
	}

	if !sameKeys(expKwargs, kwargs) {
		return nil, &MockExpectationError{fmt.Sprintf("mocked method %s called with unexpected keywords %s vs %s",
			symInspect(name), kwKeysInspect(expKwargs), kwKeysInspect(kwargs))}
	}

	for _, ekv := range expKwargs {
		av := lookupKV(kwargs, ekv.Key)
		if !m.matcher.Match(ekv.Val, av) {
			return nil, &MockExpectationError{fmt.Sprintf("mocked method %s called with unexpected keyword arguments %s vs %s",
				symInspect(name), kwargsHashInspect(m.matcher, expKwargs), kwargsHashInspect(m.matcher, kwargs))}
		}
	}

	m.actual[name] = append(m.actual[name], actualCall{Retval: e.Retval, Args: args, Kwargs: kwargs})
	return e.Retval, nil
}

// Verify reproduces Mock#verify. For each queued method: if it was never called
// at all (actual defaults to nil), it raises "Expected <expected[0]>"; if it was
// called but fewer times than expected, it raises
// "Expected <expected[actual.size]>, got [<actual calls>]". Else returns nil.
func (m *Mock) Verify() error {
	for _, name := range m.order {
		expected := m.expected[name]
		actual, called := m.actual[name]
		if !called {
			return &MockExpectationError{"Expected " + m.callInspect(name, expected[0])}
		}
		if len(actual) < len(expected) {
			return &MockExpectationError{fmt.Sprintf("Expected %s, got [%s]",
				m.callInspect(name, expected[len(actual)]), m.callsInspect(name, actual))}
		}
	}
	return nil
}

// callInspect reproduces Mock#__call for a single expectation:
// "name(args) => retval.inspect", where kwargs (if any) are appended to args.
func (m *Mock) callInspect(name string, e expectation) string {
	args := innerArrayInspect(m.matcher, e.Args)
	if len(e.Kwargs) > 0 {
		ki := innerHashInspect(m.matcher, e.Kwargs)
		if args != "" && ki != "" {
			args += ", " + ki
		} else {
			args += ki
		}
	}
	return fmt.Sprintf("%s(%s) => %s", name, args, m.matcher.Inspect(e.Retval))
}

// callsInspect reproduces __call for a list of actual calls (joined by ", ").
func (m *Mock) callsInspect(name string, calls []actualCall) string {
	parts := make([]string, len(calls))
	for i, c := range calls {
		args := innerArrayInspect(m.matcher, c.Args)
		if len(c.Kwargs) > 0 {
			ki := innerHashInspect(m.matcher, c.Kwargs)
			if args != "" && ki != "" {
				args += ", " + ki
			} else {
				args += ki
			}
		}
		parts[i] = fmt.Sprintf("%s(%s) => %s", name, args, m.matcher.Inspect(c.Retval))
	}
	return strings.Join(parts, ", ")
}

// anyClass is the Object-class matcher used by the any-kwargs form; the host's
// matcher should treat it as matching anything (Object === x is always true).
type anyClass struct{}

// sameKeys reports whether the sorted key sets of expected and actual kwargs are
// equal (the gem's expected_kwargs.keys.sort == kwargs.keys.sort).
func sameKeys(a, b []KV) bool {
	if len(a) != len(b) {
		return false
	}
	ak := make([]string, len(a))
	bk := make([]string, len(b))
	for i := range a {
		ak[i] = a[i].Key
	}
	for i := range b {
		bk[i] = b[i].Key
	}
	sort.Strings(ak)
	sort.Strings(bk)
	for i := range ak {
		if ak[i] != bk[i] {
			return false
		}
	}
	return true
}

func lookupKV(kvs []KV, key string) Value {
	for _, kv := range kvs {
		if kv.Key == key {
			return kv.Val
		}
	}
	return nil
}

// --- inspect helpers (Ruby-ish rendering of the bits the message owns) ---

// symInspect renders a Ruby Symbol inspect (":name"); the gem's %p on a Symbol.
func symInspect(name string) string { return ":" + name }

// symListInspect renders an array of symbols: "[:a, :b]".
func symListInspect(names []string) string {
	parts := make([]string, len(names))
	for i, n := range names {
		parts[i] = symInspect(n)
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

// arrInspect renders %p of an args array: "[1, \"x\"]".
func arrInspect(m MockMatcher, args []Value) string {
	return "[" + innerArrayInspect(m, args) + "]"
}

// innerArrayInspect renders the comma-joined element inspects (no brackets),
// matching the gem's data[:args].inspect[1..-2].
func innerArrayInspect(m MockMatcher, args []Value) string {
	parts := make([]string, len(args))
	for i, a := range args {
		parts[i] = m.Inspect(a)
	}
	return strings.Join(parts, ", ")
}

// kwargsInspect renders %p of a kwargs hash ("{:a=>1}" / "{}").
func kwargsInspect(m MockMatcher, kwargs []KV) string {
	return "{" + innerHashInspect(m, kwargs) + "}"
}

// innerHashInspect renders "key: val" pairs joined by ", ", matching Ruby 3.4+
// Hash#inspect for symbol keys (the gem's __call uses kwargs.inspect[1..-2], and
// mock kwargs always have Symbol keys). The minitest oracle runs on Ruby 3.4, so
// this is the format the failure messages must reproduce.
func innerHashInspect(m MockMatcher, kwargs []KV) string {
	parts := make([]string, len(kwargs))
	for i, kv := range kwargs {
		parts[i] = kv.Key + ": " + m.Inspect(kv.Val)
	}
	return strings.Join(parts, ", ")
}

// kwargsHashInspect renders the full hash form "{:a=>1}".
func kwargsHashInspect(m MockMatcher, kwargs []KV) string {
	return "{" + innerHashInspect(m, kwargs) + "}"
}

// kwKeysInspect renders an array of the keyword keys as symbols ("[:a, :b]").
func kwKeysInspect(kwargs []KV) string {
	names := make([]string, len(kwargs))
	for i, kv := range kwargs {
		names[i] = kv.Key
	}
	return symListInspect(names)
}
