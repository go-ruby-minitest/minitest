// Copyright (c) the go-ruby-minitest/minitest authors
//
// SPDX-License-Identifier: BSD-3-Clause

package minitest

import "testing"

func errMsg(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func TestMockHappyPath(t *testing.T) {
	m := NewMock(newFakeMatcher())
	if err := m.Expect("foo", rInt(42), []Value{rInt(1), rStr("a")}, nil, false); err != nil {
		t.Fatal(err)
	}
	got, err := m.Call("foo", []Value{rInt(1), rStr("a")}, nil)
	if err != nil || !eqV(got, rInt(42)) {
		t.Fatalf("call = %v, %v", got, err)
	}
	if err := m.Verify(); err != nil {
		t.Errorf("verify should pass: %v", err)
	}
}

func TestMockVerifyUncalled(t *testing.T) {
	m := NewMock(newFakeMatcher())
	m.Expect("foo", rInt(42), nil, nil, false)
	if got := errMsg(m.Verify()); got != "Expected foo() => 42" {
		t.Errorf("verify uncalled = %q", got)
	}
}

func TestMockVerifyUndercalled(t *testing.T) {
	m := NewMock(newFakeMatcher())
	m.Expect("foo", rInt(1), nil, nil, false)
	m.Expect("foo", rInt(2), nil, nil, false)
	m.Call("foo", nil, nil)
	if got := errMsg(m.Verify()); got != "Expected foo() => 2, got [foo() => 1]" {
		t.Errorf("verify undercalled = %q", got)
	}
}

func TestMockVerifyArgsRendered(t *testing.T) {
	m := NewMock(newFakeMatcher())
	m.Expect("foo", rInt(1), []Value{rInt(1), rInt(2)}, nil, false)
	if got := errMsg(m.Verify()); got != "Expected foo(1, 2) => 1" {
		t.Errorf("verify args = %q", got)
	}
}

func TestMockVerifyKwargs(t *testing.T) {
	m := NewMock(newFakeMatcher())
	m.Expect("foo", rInt(1), nil, []KV{{"key", rInt(5)}}, false)
	if got := errMsg(m.Verify()); got != "Expected foo(key: 5) => 1" {
		t.Errorf("verify kwargs = %q", got)
	}
}

func TestMockNoMore(t *testing.T) {
	m := NewMock(newFakeMatcher())
	m.Expect("foo", rInt(1), nil, nil, false)
	m.Call("foo", nil, nil)
	_, err := m.Call("foo", nil, nil)
	if got := errMsg(err); got != "No more expects available for :foo: [] {}" {
		t.Errorf("no more = %q", got)
	}
	if _, ok := err.(*MockExpectationError); !ok {
		t.Errorf("no more should be MockExpectationError")
	}
}

func TestMockUnexpectedArgs(t *testing.T) {
	m := NewMock(newFakeMatcher())
	m.Expect("foo", rInt(1), []Value{rInt(1), rInt(2)}, nil, false)
	_, err := m.Call("foo", []Value{rInt(3), rInt(4)}, nil)
	if got := errMsg(err); got != "mocked method :foo called with unexpected arguments [3, 4]" {
		t.Errorf("unexpected args = %q", got)
	}
}

func TestMockArity(t *testing.T) {
	m := NewMock(newFakeMatcher())
	m.Expect("foo", rInt(1), []Value{rInt(1), rInt(2)}, nil, false)
	_, err := m.Call("foo", []Value{rInt(1)}, nil)
	if got := errMsg(err); got != "mocked method :foo expects 2 arguments, got [1]" {
		t.Errorf("arity = %q", got)
	}
	if _, ok := err.(*MockArgumentError); !ok {
		t.Errorf("arity should be ArgumentError")
	}
}

func TestMockUnmocked(t *testing.T) {
	m := NewMock(newFakeMatcher())
	m.Expect("foo", rInt(1), nil, nil, false)
	_, err := m.Call("bar", nil, nil)
	if got := errMsg(err); got != "unmocked method :bar, expected one of [:foo]" {
		t.Errorf("unmocked = %q", got)
	}
	if _, ok := err.(*MockNoMethodError); !ok {
		t.Errorf("unmocked should be NoMethodError")
	}
}

func TestMockExpectBlockArgErrors(t *testing.T) {
	m := NewMock(newFakeMatcher())
	if got := errMsg(m.Expect("foo", rInt(1), []Value{rInt(1)}, nil, true)); got != "args ignored when block given" {
		t.Errorf("block+args = %q", got)
	}
	if got := errMsg(m.Expect("foo", rInt(1), nil, []KV{{"k", rInt(1)}}, true)); got != "kwargs ignored when block given" {
		t.Errorf("block+kwargs = %q", got)
	}
}

func TestMockBlockExpectation(t *testing.T) {
	fm := newFakeMatcher()
	fm.blocks[0] = func(args []Value, kwargs []KV) bool {
		return len(args) == 2 && eqV(args[0], rStr("buggs")) && eqV(args[1], rSym("bunny"))
	}
	m := NewMock(fm)
	m.Expect("foo", rInt(7), nil, nil, true)
	got, err := m.Call("foo", []Value{rStr("buggs"), rSym("bunny")}, nil)
	if err != nil || !eqV(got, rInt(7)) {
		t.Fatalf("block call = %v %v", got, err)
	}
	if err := m.Verify(); err != nil {
		t.Errorf("block verify = %v", err)
	}
}

func TestMockBlockFails(t *testing.T) {
	fm := newFakeMatcher()
	fm.blocks[0] = func(args []Value, kwargs []KV) bool { return false }
	m := NewMock(fm)
	m.Expect("foo", rInt(7), nil, nil, true)
	_, err := m.Call("foo", []Value{rInt(1)}, nil)
	if got := errMsg(err); got != "mocked method :foo failed block w/ [1] {}" {
		t.Errorf("block fail = %q", got)
	}
}

func TestMockArgMatchClass(t *testing.T) {
	// A class matcher accepts any instance (case equality String === "x").
	m := NewMock(newFakeMatcher())
	m.Expect("foo", rInt(1), []Value{rClass("String")}, nil, false)
	if got, err := m.Call("foo", []Value{rStr("x")}, nil); err != nil || !eqV(got, rInt(1)) {
		t.Errorf("class-match call = %v %v", got, err)
	}
}

func TestMockKwargMatch(t *testing.T) {
	m := NewMock(newFakeMatcher())
	m.Expect("foo", rInt(1), nil, []KV{{"a", rInt(5)}}, false)
	if got, err := m.Call("foo", nil, []KV{{"a", rInt(5)}}); err != nil || !eqV(got, rInt(1)) {
		t.Errorf("kwarg call = %v %v", got, err)
	}
	if err := m.Verify(); err != nil {
		t.Errorf("kwarg verify = %v", err)
	}
}

func TestMockKwargCountMismatch(t *testing.T) {
	m := NewMock(newFakeMatcher())
	m.Expect("foo", rInt(1), nil, []KV{{"a", rInt(5)}}, false)
	_, err := m.Call("foo", nil, nil)
	if got := errMsg(err); got != "mocked method :foo expects 1 keyword arguments, got []" {
		t.Errorf("kwarg count = %q", got)
	}
}

func TestMockKwargKeyMismatch(t *testing.T) {
	m := NewMock(newFakeMatcher())
	m.Expect("foo", rInt(1), nil, []KV{{"a", rInt(5)}}, false)
	_, err := m.Call("foo", nil, []KV{{"b", rInt(5)}})
	if got := errMsg(err); got != "mocked method :foo called with unexpected keywords [:a] vs [:b]" {
		t.Errorf("kwarg keys = %q", got)
	}
}

func TestMockKwargValueMismatch(t *testing.T) {
	m := NewMock(newFakeMatcher())
	m.Expect("foo", rInt(1), nil, []KV{{"a", rInt(5)}}, false)
	_, err := m.Call("foo", nil, []KV{{"a", rInt(6)}})
	if got := errMsg(err); got != "mocked method :foo called with unexpected keyword arguments {a: 5} vs {a: 6}" {
		t.Errorf("kwarg value = %q", got)
	}
}

func TestMockAnyKwargs(t *testing.T) {
	m := NewMock(newFakeMatcher())
	m.ExpectAnyKwargs("foo", rInt(1), nil)
	if got, err := m.Call("foo", nil, []KV{{"whatever", rInt(99)}}); err != nil || !eqV(got, rInt(1)) {
		t.Errorf("any-kwargs call = %v %v", got, err)
	}
}

func TestMockVerifyEmpty(t *testing.T) {
	m := NewMock(newFakeMatcher())
	if err := m.Verify(); err != nil {
		t.Errorf("empty verify = %v", err)
	}
}

func TestMockErrorTypes(t *testing.T) {
	if (&MockExpectationError{"x"}).Error() != "x" {
		t.Error("MockExpectationError")
	}
	if (&MockArgumentError{"y"}).Error() != "y" {
		t.Error("MockArgumentError")
	}
	if (&MockNoMethodError{"z"}).Error() != "z" {
		t.Error("MockNoMethodError")
	}
}

// --- Stub ---

type fakeStub struct {
	installErr error
	blockErr   error
	steps      []string
}

func (s *fakeStub) Install() error {
	s.steps = append(s.steps, "install")
	return s.installErr
}
func (s *fakeStub) RunBlock() error {
	s.steps = append(s.steps, "run")
	return s.blockErr
}
func (s *fakeStub) Restore() {
	s.steps = append(s.steps, "restore")
}

func TestStubHappy(t *testing.T) {
	s := &fakeStub{}
	if err := Stub(s); err != nil {
		t.Fatalf("stub err = %v", err)
	}
	if !eqSteps(s.steps, []string{"install", "run", "restore"}) {
		t.Errorf("steps = %v", s.steps)
	}
}

func TestStubBlockErrorStillRestores(t *testing.T) {
	s := &fakeStub{blockErr: errString("boom")}
	if err := Stub(s); err == nil || err.Error() != "boom" {
		t.Errorf("block err = %v", err)
	}
	if s.steps[len(s.steps)-1] != "restore" {
		t.Errorf("restore must run: %v", s.steps)
	}
}

func TestStubInstallFails(t *testing.T) {
	s := &fakeStub{installErr: errString("no such method")}
	err := Stub(s)
	if err == nil || err.Error() != "no such method" {
		t.Errorf("install err = %v", err)
	}
	// Block must not run; restore still fires.
	for _, st := range s.steps {
		if st == "run" {
			t.Errorf("block ran despite install failure: %v", s.steps)
		}
	}
	if s.steps[len(s.steps)-1] != "restore" {
		t.Errorf("restore must run on install fail: %v", s.steps)
	}
}

func eqSteps(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestSortedSyms(t *testing.T) {
	got := sortedSyms([]string{"b", "a", "c"})
	if got[0] != "a" || got[1] != "b" || got[2] != "c" {
		t.Errorf("sortedSyms = %v", got)
	}
}
