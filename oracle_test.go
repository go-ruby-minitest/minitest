// Copyright (c) the go-ruby-minitest/minitest authors
//
// SPDX-License-Identifier: BSD-3-Clause

package minitest

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

// minitestRuby locates a `ruby` that can load the minitest 5.x gem and returns a
// runner. The oracle tests skip themselves when ruby or the gem is absent (the
// Windows lane and the qemu cross-arch lanes), so the deterministic suite alone
// drives the 100% gate there. The 5.x gem is pinned because the failure messages,
// mu_pp, and run-result semantics this package reproduces are the 5.x ones.
func minitestRuby(t *testing.T) string {
	t.Helper()
	bin, err := exec.LookPath("ruby")
	if err != nil {
		t.Skip("ruby not on PATH; skipping minitest-gem oracle")
	}
	// Confirm a 5.x minitest is loadable. MINITEST5_LIB lets a developer whose
	// system ruby is too new for the 5.x gem point at an unpacked 5.x source tree
	// (e.g. `gem unpack minitest -v 5.25.5`); CI just installs the gem.
	out, err := exec.Command(bin, mtArgs("require \"minitest\"; print Minitest::VERSION")...).CombinedOutput()
	if err != nil || !strings.HasPrefix(string(out), "5.") {
		t.Skipf("minitest 5.x not available (got %q, %v); skipping oracle", string(out), err)
	}
	return bin
}

// mtArgs builds the ruby argv that loads minitest 5.x: either from MINITEST5_LIB
// (an unpacked 5.x lib dir, loaded with --disable-gems so a newer default gem
// does not shadow it) or from the installed 5.x gem.
func mtArgs(script string) []string {
	if lib := os.Getenv("MINITEST5_LIB"); lib != "" {
		return []string{"--disable-gems", "-I" + lib, "-e", script}
	}
	return []string{"-e", `gem "minitest", "~> 5.25"; ` + script}
}

// rubyMinitest runs a script with the 5.x gem loaded and $stdout in binary mode
// (the go-ruby Windows lesson) and returns stdout. The preamble selects the gem
// version, requires minitest + mock, and defines a Test subclass instance `t`.
func rubyMinitest(t *testing.T, bin, script string) string {
	t.Helper()
	preamble := `$stdout.binmode
require "minitest"
require "minitest/mock"
t = Class.new(Minitest::Test).new("oracle")
def cap(t) yield; "NO RAISE"; rescue Minitest::Assertion => e; e.message; end
`
	cmd := exec.Command(bin, mtArgs(preamble+script)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("ruby error: %v\nscript:\n%s\noutput:\n%s", err, script, out)
	}
	return string(out)
}

// TestOracleAssertionMessages diffs every assertion's failure message against the
// live minitest gem. Each case computes the message here (via the fake runtime)
// and asks the gem for the same failing assertion's message; they must match
// byte-for-byte.
func TestOracleAssertionMessages(t *testing.T) {
	bin := minitestRuby(t)
	a := newA()

	cases := []struct {
		name   string
		got    string // our message
		script string // ruby producing the gem message on stdout
	}{
		{"assert_false", msgOf(a.Assert(rBool(false), "")), `print(cap(t){ t.assert false })`},
		{"assert_nil_arg", msgOf(a.Assert(rNil, "")), `print(cap(t){ t.assert nil })`},
		{"refute_true", msgOf(a.Refute(rBool(true), "")), `print(cap(t){ t.refute true })`},
		{"flunk", msgOf(a.Flunk("")), `print(cap(t){ t.flunk })`},
		{"assert_equal_int", eqMsg(a, rInt(1), rInt(2), ""), `print(cap(t){ t.assert_equal 1,2 })`},
		{"assert_equal_str", eqMsg(a, rStr("a"), rStr("b"), ""), `print(cap(t){ t.assert_equal "a","b" })`},
		{"assert_equal_msg", eqMsg(a, rInt(1), rInt(2), "oops"), `print(cap(t){ t.assert_equal 1,2,"oops" })`},
		{"refute_equal", msgOf(a.RefuteEqual(rInt(1), rInt(1), "")), `print(cap(t){ t.refute_equal 1,1 })`},
		{"assert_nil", msgOf(a.AssertNil(rInt(5), "")), `print(cap(t){ t.assert_nil 5 })`},
		{"refute_nil", msgOf(a.RefuteNil(rNil, "")), `print(cap(t){ t.refute_nil nil })`},
		{"assert_empty", msgOf(a.AssertEmpty(rArr{rInt(1)}, "")), `print(cap(t){ t.assert_empty [1] })`},
		{"assert_empty_respond", msgOf(a.AssertEmpty(rInt(5), "")), `print(cap(t){ t.assert_empty 5 })`},
		{"refute_empty", msgOf(a.RefuteEmpty(rArr{}, "")), `print(cap(t){ t.refute_empty [] })`},
		{"assert_includes", msgOf(a.AssertIncludes(rArr{rInt(1), rInt(2)}, rInt(3), "")), `print(cap(t){ t.assert_includes [1,2],3 })`},
		{"refute_includes", msgOf(a.RefuteIncludes(rArr{rInt(1), rInt(2)}, rInt(2), "")), `print(cap(t){ t.refute_includes [1,2],2 })`},
		{"assert_instance_of", msgOf(a.AssertInstanceOf(rClass("String"), rInt(5), "")), `print(cap(t){ t.assert_instance_of String, 5 })`},
		{"refute_instance_of", msgOf(a.RefuteInstanceOf(rClass("Integer"), rInt(5), "")), `print(cap(t){ t.refute_instance_of Integer, 5 })`},
		{"assert_kind_of", msgOf(a.AssertKindOf(rClass("String"), rInt(5), "")), `print(cap(t){ t.assert_kind_of String, 5 })`},
		{"refute_kind_of", msgOf(a.RefuteKindOf(rClass("Numeric"), rInt(5), "")), `print(cap(t){ t.refute_kind_of Numeric, 5 })`},
		{"assert_respond_to", msgOf(a.AssertRespondTo(rInt(5), "foo", "", false)), `print(cap(t){ t.assert_respond_to 5, :foo })`},
		{"refute_respond_to", msgOf(a.RefuteRespondTo(rInt(5), "to_s", "", false)), `print(cap(t){ t.refute_respond_to 5, :to_s })`},
		{"assert_match", msgOf(a.AssertMatch(rReg{src: "x"}, rStr("y"), "")), `print(cap(t){ t.assert_match(/x/, "y") })`},
		{"assert_match_str", msgOf(a.AssertMatch(rStr("x"), rStr("y"), "")), `print(cap(t){ t.assert_match "x", "y" })`},
		{"refute_match", msgOf(a.RefuteMatch(rReg{src: "y"}, rStr("y"), "")), `print(cap(t){ t.refute_match(/y/, "y") })`},
		{"assert_operator", msgOf(a.AssertOperator(rInt(5), "<", rInt(4), "")), `print(cap(t){ t.assert_operator 5, :<, 4 })`},
		{"refute_operator", msgOf(a.RefuteOperator(rInt(5), "<", rInt(6), "")), `print(cap(t){ t.refute_operator 5, :<, 6 })`},
		{"assert_predicate", msgOf(a.AssertPredicate(rStr("x"), "empty?", "")), `print(cap(t){ t.assert_predicate "x", :empty? })`},
		{"refute_predicate", msgOf(a.RefutePredicate(rStr(""), "empty?", "")), `print(cap(t){ t.refute_predicate "", :empty? })`},
		{"assert_same", msgOf(a.AssertSame(rObj{id: 96, insp: "\"a\""}, rObj{id: 88, insp: "\"a\""}, "")),
			`a="a"; b="a"; print(cap(t){ t.assert_same a, b })`},
		{"assert_in_delta", msgOf(a.AssertInDelta(1.0, 2.0, 0.1, "")), `print(cap(t){ t.assert_in_delta 1.0, 2.0, 0.1 })`},
		{"refute_in_delta", msgOf(a.RefuteInDelta(1.0, 1.0, 0.1, "")), `print(cap(t){ t.refute_in_delta 1.0, 1.0, 0.1 })`},
		{"assert_in_epsilon", msgOf(a.AssertInEpsilon(1.0, 2.0, 0.1, "")), `print(cap(t){ t.assert_in_epsilon 1.0, 2.0, 0.1 })`},
	}

	for _, c := range cases {
		if c.name == "assert_same" {
			// assert_same's oids are runtime object ids; compare the structure with
			// the oids normalized away on both sides.
			gotN := normOID(c.got)
			wantN := normOID(rubyMinitest(t, bin, `a="a"; b="a"; print(cap(t){ t.assert_same a, b })`))
			if gotN != wantN {
				t.Errorf("%s: oid-normalized\n got %q\nwant %q", c.name, gotN, wantN)
			}
			continue
		}
		t.Run(c.name, func(t *testing.T) {
			want := rubyMinitest(t, bin, c.script)
			if c.got != want {
				t.Errorf("message mismatch\n got %q\nwant %q", c.got, want)
			}
		})
	}
}

// eqMsg returns AssertEqual's failure message (dropping the deprecation flag).
func eqMsg(a *Assertions, exp, act Value, msg string) string {
	err, _ := a.AssertEqual(exp, act, msg)
	return msgOf(err)
}

// normOID strips oid=<digits> so assert_same messages compare structurally.
func normOID(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); {
		if strings.HasPrefix(s[i:], "oid=") {
			b.WriteString("oid=N")
			i += 4
			for i < len(s) && s[i] >= '0' && s[i] <= '9' {
				i++
			}
			continue
		}
		b.WriteByte(s[i])
		i++
	}
	return b.String()
}

// TestOracleRaisesThrowsSkip diffs the block-assertion and skip messages.
func TestOracleRaisesThrowsSkip(t *testing.T) {
	bin := minitestRuby(t)
	a := newA()

	// assert_raises: nothing raised (single class).
	_, e1 := a.AssertRaises(RaiseOutcome{}, "", "[ArgumentError]", "ArgumentError")
	want1 := rubyMinitest(t, bin, `print(cap(t){ t.assert_raises(ArgumentError){} })`)
	if msgOf(e1) != want1 {
		t.Errorf("raises-nothing\n got %q\nwant %q", msgOf(e1), want1)
	}

	// assert_raises: nothing raised (multiple classes).
	_, e2 := a.AssertRaises(RaiseOutcome{}, "", "[ArgumentError, TypeError]", "")
	want2 := rubyMinitest(t, bin, `print(cap(t){ t.assert_raises(ArgumentError, TypeError){} })`)
	if msgOf(e2) != want2 {
		t.Errorf("raises-nothing-multi\n got %q\nwant %q", msgOf(e2), want2)
	}

	// assert_raises: nothing raised, with a custom message.
	_, e3 := a.AssertRaises(RaiseOutcome{}, "custom", "[ArgumentError]", "ArgumentError")
	want3 := rubyMinitest(t, bin, `print(cap(t){ t.assert_raises(ArgumentError, "custom"){} })`)
	if msgOf(e3) != want3 {
		t.Errorf("raises-custom\n got %q\nwant %q", msgOf(e3), want3)
	}

	// assert_throws: not thrown.
	want4 := rubyMinitest(t, bin, `print(cap(t){ t.assert_throws(:foo){} })`)
	if got := msgOf(a.AssertThrows(ThrowOutcome{}, ":foo", "")); got != want4 {
		t.Errorf("throws\n got %q\nwant %q", got, want4)
	}

	// skip messages (Skip < Assertion < Exception, so cap's rescue catches it).
	wantSkip := rubyMinitest(t, bin, `print(cap(t){ t.skip })`)
	if got := a.SkipError("").Msg; got != wantSkip {
		t.Errorf("skip default\n got %q\nwant %q", got, wantSkip)
	}
	wantSkipMsg := rubyMinitest(t, bin, `print(cap(t){ t.skip "later" })`)
	if got := a.SkipError("later").Msg; got != wantSkipMsg {
		t.Errorf("skip msg\n got %q\nwant %q", got, wantSkipMsg)
	}
}

// TestOracleMuPP diffs mu_pp output for a representative value set.
func TestOracleMuPP(t *testing.T) {
	bin := minitestRuby(t)
	a := newA()
	cases := []struct {
		v      Value
		script string
	}{
		{rInt(5), `print t.mu_pp(5)`},
		{rStr("hi"), `print t.mu_pp("hi")`},
		{rArr{rInt(1), rInt(2)}, `print t.mu_pp([1,2])`},
		{rNil, `print t.mu_pp(nil)`},
		{rSym("foo"), `print t.mu_pp(:foo)`},
		{rStr("a\nb"), "print t.mu_pp(\"a\\nb\")"},
	}
	for _, c := range cases {
		want := rubyMinitest(t, bin, c.script)
		if got := a.MuPP(c.v); got != want {
			t.Errorf("mu_pp(%v)\n got %q\nwant %q", c.v, got, want)
		}
	}
}

// TestOracleRunResults diffs run-result semantics (code/passed/skipped/error/
// assertions) for pass/fail/skip/error tests.
func TestOracleRunResults(t *testing.T) {
	bin := minitestRuby(t)

	// Build the four results here.
	pass := mustResult(&fakeBody{name: "t", class: "T", asserts: 1})
	failA := &Assertion{Msg: "x", Backtrace: []string{"t.rb:1:in `t`"}}
	fail := mustResult(&fakeBody{name: "t", class: "T", asserts: 1, results: map[string]error{"t": failA}})
	skip := mustResult(&fakeBody{name: "t", class: "T", results: map[string]error{"t": &Skip{Assertion{Msg: "s"}}}})
	errR := mustResult(&fakeBody{name: "t", class: "T", results: map[string]error{"t": &UnexpectedError{ErrorClass: "RuntimeError", ErrorMessage: "boom"}}})

	got := map[string][4]string{
		"pass": resultTuple(pass),
		"fail": resultTuple(fail),
		"skip": resultTuple(skip),
		"err":  resultTuple(errR),
	}

	script := `
klass = Class.new(Minitest::Test) do
  def self.name; "T"; end
  def test_pass; assert true; end
  def test_fail; assert_equal 1,2; end
  def test_skip; skip "s"; end
  def test_err; raise "boom"; end
end
{"pass"=>"test_pass","fail"=>"test_fail","skip"=>"test_skip","err"=>"test_err"}.each do |k,m|
  r = Minitest.run_one_method(klass, m)
  printf("%s|%s|%s|%s|%s\n", k, r.result_code, r.passed?, (r.skipped? ? true : false), r.error?)
end
`
	out := rubyMinitest(t, bin, script)
	wantTuples := map[string][4]string{}
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		f := strings.Split(line, "|")
		wantTuples[f[0]] = [4]string{f[1], f[2], f[3], f[4]}
	}
	for k, g := range got {
		if w, ok := wantTuples[k]; !ok || g != w {
			t.Errorf("run-result %s: got %v, want %v", k, g, w)
		}
	}
}

func resultTuple(r *Result) [4]string {
	return [4]string{r.ResultCode(), b2s(r.Passed()), b2s(r.Skipped()), b2s(r.Errored())}
}

func b2s(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

func mustResult(b *fakeBody) *Result {
	r, _ := RunTest(b, 0)
	return r
}

// TestOracleMockVerify diffs Mock verify / call error messages against the gem.
func TestOracleMockVerify(t *testing.T) {
	bin := minitestRuby(t)

	build := func(fn func(*Mock) string, script string) func() (string, string) {
		return func() (string, string) {
			m := NewMock(newFakeMatcher())
			return fn(m), script
		}
	}
	cases := []struct {
		name string
		run  func() (string, string)
	}{
		{"verify_uncalled", build(func(m *Mock) string {
			m.Expect("foo", rInt(42), nil, nil, false)
			return errMsg(m.Verify())
		}, `m=Minitest::Mock.new; m.expect(:foo,42); print((m.verify rescue $!).message)`)},
		{"verify_undercalled", build(func(m *Mock) string {
			m.Expect("foo", rInt(1), nil, nil, false)
			m.Expect("foo", rInt(2), nil, nil, false)
			m.Call("foo", nil, nil)
			return errMsg(m.Verify())
		}, `m=Minitest::Mock.new; m.expect(:foo,1); m.expect(:foo,2); m.foo; print((m.verify rescue $!).message)`)},
		{"verify_args", build(func(m *Mock) string {
			m.Expect("foo", rInt(1), []Value{rInt(1), rInt(2)}, nil, false)
			return errMsg(m.Verify())
		}, `m=Minitest::Mock.new; m.expect(:foo,1,[1,2]); print((m.verify rescue $!).message)`)},
		{"verify_kwargs", build(func(m *Mock) string {
			m.Expect("foo", rInt(1), nil, []KV{{"key", rInt(5)}}, false)
			return errMsg(m.Verify())
		}, `m=Minitest::Mock.new; m.expect(:foo,1,[],key: 5); print((m.verify rescue $!).message)`)},
		{"no_more", build(func(m *Mock) string {
			m.Expect("foo", rInt(1), nil, nil, false)
			m.Call("foo", nil, nil)
			_, e := m.Call("foo", nil, nil)
			return errMsg(e)
		}, `m=Minitest::Mock.new; m.expect(:foo,1); m.foo; print((m.foo rescue $!).message)`)},
		{"unexpected_args", build(func(m *Mock) string {
			m.Expect("foo", rInt(1), []Value{rInt(1), rInt(2)}, nil, false)
			_, e := m.Call("foo", []Value{rInt(3), rInt(4)}, nil)
			return errMsg(e)
		}, `m=Minitest::Mock.new; m.expect(:foo,1,[1,2]); print((m.foo(3,4) rescue $!).message)`)},
		{"arity", build(func(m *Mock) string {
			m.Expect("foo", rInt(1), []Value{rInt(1), rInt(2)}, nil, false)
			_, e := m.Call("foo", []Value{rInt(1)}, nil)
			return errMsg(e)
		}, `m=Minitest::Mock.new; m.expect(:foo,1,[1,2]); print((m.foo(1) rescue $!).message)`)},
		{"unmocked", build(func(m *Mock) string {
			m.Expect("foo", rInt(1), nil, nil, false)
			_, e := m.Call("bar", nil, nil)
			return errMsg(e)
		}, `m=Minitest::Mock.new; m.expect(:foo,1); print((m.bar rescue $!).message)`)},
		{"expect_block_args", build(func(m *Mock) string {
			return errMsg(m.Expect("foo", rInt(1), []Value{rInt(1)}, nil, true))
		}, `m=Minitest::Mock.new; print((m.expect(:foo,1,[1]){true} rescue $!).message)`)},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, script := c.run()
			want := rubyMinitest(t, bin, script)
			if got != want {
				t.Errorf("%s\n got %q\nwant %q", c.name, got, want)
			}
		})
	}
}
