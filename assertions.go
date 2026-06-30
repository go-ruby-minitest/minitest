// Copyright (c) the go-ruby-minitest/minitest authors
//
// SPDX-License-Identifier: BSD-3-Clause

package minitest

import (
	"math"
	"strconv"
	"strings"
)

// Assertions is Minitest::Assertions bound to a [Runtime]. It owns the assertion
// counter and produces the exact failure messages of the minitest gem. Each
// assert_*/refute_* method returns nil on success or a non-nil error (an
// [*Assertion] or [*Skip]) describing the failure; the host raises it in the
// Ruby VM and the lifecycle ([Test]) captures it.
//
// The value-level comparisons (==, =~, inspect, include?, …) are delegated to
// the Runtime; this type owns only the formatting and the assertion bookkeeping.
type Assertions struct {
	rt Runtime
	// Count mirrors the Ruby `assertions` accessor: every assert call bumps it.
	Count int
}

// NewAssertions returns an Assertions backed by rt.
func NewAssertions(rt Runtime) *Assertions { return &Assertions{rt: rt} }

// fail builds an [*Assertion] carrying msg. bt is the host-supplied backtrace for
// the call site (may be nil; tests of message formatting pass nil).
func fail(msg string) *Assertion { return &Assertion{Msg: msg} }

// message reproduces Minitest::Assertions#message: it composes an optional custom
// message with a default message and an ending.
//
//	proc {
//	  msg = msg.call.chomp(".") if Proc === msg
//	  custom = "#{msg}.\n" unless msg.nil? or msg.to_s.empty?
//	  "#{custom}#{default.call}#{ending || "."}"
//	}
//
// Here msg is the already-resolved custom message string ("" when none) and
// ending is "" or "." (pass "" via the E sentinel for assert_equal's diff form).
// The custom message is chomped of a single trailing "." before the ".\n" is
// appended, matching the Proc branch (the only way a custom message reaches the
// default helpers in the gem is already-stringified, and chomp(".") is applied).
func message(msg, ending string, def func() string) string {
	var custom string
	if msg != "" {
		custom = strings.TrimSuffix(msg, ".") + ".\n"
	}
	if ending == E {
		ending = ""
	} else if ending == "" {
		ending = "."
	}
	return custom + def() + ending
}

// E is the empty-ending sentinel assert_equal passes so its diff message gets no
// trailing ".", matching `message(msg, E) { diff exp, act }`.
const E = "\x00E\x00"

// diff reproduces Minitest::Assertions#diff for the no-diff-command path: with no
// usable external diff binary (the deterministic, interpreter-independent default
// for this library), things_to_diff yields nothing and diff returns the basic
//
//	"Expected: #{mu_pp exp}\n  Actual: #{mu_pp act}"
//
// form. This is the canonical assert_equal failure message byte-faithful tooling
// targets.
func (a *Assertions) diff(exp, act Value) string {
	return "Expected: " + a.MuPP(exp) + "\n  Actual: " + a.MuPP(act)
}

// Assert is Minitest::Assertions#assert: increments the count and fails with msg
// (default "Expected #{mu_pp test} to be truthy.") unless test is truthy.
func (a *Assertions) Assert(test Value, msg string) error {
	a.Count++
	if a.rt.Truthy(test) {
		return nil
	}
	if msg == "" {
		msg = "Expected " + a.MuPP(test) + " to be truthy."
	}
	return fail(msg)
}

// assertBool is the internal form used when the truthiness has already been
// computed in Go (most assertions evaluate a Go bool). It still bumps the count.
func (a *Assertions) assertBool(ok bool, msg string) error {
	a.Count++
	if ok {
		return nil
	}
	return fail(msg)
}

// Refute is Minitest::Assertions#refute: fails if test is truthy.
func (a *Assertions) Refute(test Value, msg string) error {
	if msg == "" {
		msg = message("", "", func() string { return "Expected " + a.MuPP(test) + " to not be truthy" })
	}
	return a.assertBool(!a.rt.Truthy(test), msg)
}

// Flunk is Minitest::Assertions#flunk: always fails with msg (default "Epic Fail!").
func (a *Assertions) Flunk(msg string) error {
	if msg == "" {
		msg = "Epic Fail!"
	}
	return a.assertBool(false, msg)
}

// Pass is Minitest::Assertions#pass: counts an assertion and always succeeds.
func (a *Assertions) Pass() error { return a.assertBool(true, "") }

// SkipError builds the [*Skip] that Minitest::Assertions#skip raises (it does not
// bump the assertion count). Default message "Skipped, no message given".
func (a *Assertions) SkipError(msg string) *Skip {
	if msg == "" {
		msg = "Skipped, no message given"
	}
	return &Skip{Assertion{Msg: msg}}
}

// AssertEqual is assert_equal. On failure the message is the diff form
// ("Expected: …\n  Actual: …"). It returns, additionally, the 5.x nil-deprecation
// signal: when exp is nil and the host wants the warning, deprecated is true (the
// gem warns rather than failing in 5.x). The host emits the warning text; the
// assertion result itself is unaffected.
func (a *Assertions) AssertEqual(exp, act Value, msg string) (err error, deprecated bool) {
	full := message(msg, E, func() string { return a.diff(exp, act) })
	err = a.assertBool(a.rt.Equal(exp, act), full)
	if a.rt.IsNil(exp) {
		deprecated = true
	}
	return err, deprecated
}

// RefuteEqual is refute_equal.
func (a *Assertions) RefuteEqual(exp, act Value, msg string) error {
	full := message(msg, "", func() string {
		return "Expected " + a.MuPP(act) + " to not be equal to " + a.MuPP(exp)
	})
	return a.assertBool(!a.rt.Equal(exp, act), full)
}

// AssertNil is assert_nil.
func (a *Assertions) AssertNil(obj Value, msg string) error {
	full := message(msg, "", func() string { return "Expected " + a.MuPP(obj) + " to be nil" })
	return a.assertBool(a.rt.IsNil(obj), full)
}

// RefuteNil is refute_nil.
func (a *Assertions) RefuteNil(obj Value, msg string) error {
	full := message(msg, "", func() string { return "Expected " + a.MuPP(obj) + " to not be nil" })
	return a.assertBool(!a.rt.IsNil(obj), full)
}

// AssertEmpty is assert_empty. It first asserts obj responds to :empty? (so a
// non-respond_to value fails with the respond_to message), then asserts empty.
func (a *Assertions) AssertEmpty(obj Value, msg string) error {
	full := message(msg, "", func() string { return "Expected " + a.MuPP(obj) + " to be empty" })
	if err := a.AssertRespondTo(obj, "empty?", "", false); err != nil {
		return err
	}
	return a.assertBool(a.rt.Empty(obj), full)
}

// RefuteEmpty is refute_empty.
func (a *Assertions) RefuteEmpty(obj Value, msg string) error {
	full := message(msg, "", func() string { return "Expected " + a.MuPP(obj) + " to not be empty" })
	if err := a.AssertRespondTo(obj, "empty?", "", false); err != nil {
		return err
	}
	return a.assertBool(!a.rt.Empty(obj), full)
}

// AssertIncludes is assert_includes.
func (a *Assertions) AssertIncludes(collection, obj Value, msg string) error {
	full := message(msg, "", func() string {
		return "Expected " + a.MuPP(collection) + " to include " + a.MuPP(obj)
	})
	if err := a.AssertRespondTo(collection, "include?", "", false); err != nil {
		return err
	}
	return a.assertBool(a.rt.Includes(collection, obj), full)
}

// RefuteIncludes is refute_includes.
func (a *Assertions) RefuteIncludes(collection, obj Value, msg string) error {
	full := message(msg, "", func() string {
		return "Expected " + a.MuPP(collection) + " to not include " + a.MuPP(obj)
	})
	if err := a.AssertRespondTo(collection, "include?", "", false); err != nil {
		return err
	}
	return a.assertBool(!a.rt.Includes(collection, obj), full)
}

// AssertInstanceOf is assert_instance_of.
func (a *Assertions) AssertInstanceOf(cls, obj Value, msg string) error {
	full := message(msg, "", func() string {
		return "Expected " + a.MuPP(obj) + " to be an instance of " + a.rt.Name(cls) +
			", not " + a.rt.ClassName(obj)
	})
	return a.assertBool(a.rt.InstanceOf(obj, cls), full)
}

// RefuteInstanceOf is refute_instance_of.
func (a *Assertions) RefuteInstanceOf(cls, obj Value, msg string) error {
	full := message(msg, "", func() string {
		return "Expected " + a.MuPP(obj) + " to not be an instance of " + a.rt.Name(cls)
	})
	return a.assertBool(!a.rt.InstanceOf(obj, cls), full)
}

// AssertKindOf is assert_kind_of.
func (a *Assertions) AssertKindOf(cls, obj Value, msg string) error {
	full := message(msg, "", func() string {
		return "Expected " + a.MuPP(obj) + " to be a kind of " + a.rt.Name(cls) +
			", not " + a.rt.ClassName(obj)
	})
	return a.assertBool(a.rt.KindOf(obj, cls), full)
}

// RefuteKindOf is refute_kind_of.
func (a *Assertions) RefuteKindOf(cls, obj Value, msg string) error {
	full := message(msg, "", func() string {
		return "Expected " + a.MuPP(obj) + " to not be a kind of " + a.rt.Name(cls)
	})
	return a.assertBool(!a.rt.KindOf(obj, cls), full)
}

// AssertRespondTo is assert_respond_to.
func (a *Assertions) AssertRespondTo(obj Value, meth, msg string, includeAll bool) error {
	full := message(msg, "", func() string {
		return "Expected " + a.MuPP(obj) + " (" + a.rt.ClassName(obj) + ") to respond to #" + meth
	})
	return a.assertBool(a.rt.RespondTo(obj, meth, includeAll), full)
}

// RefuteRespondTo is refute_respond_to.
func (a *Assertions) RefuteRespondTo(obj Value, meth, msg string, includeAll bool) error {
	full := message(msg, "", func() string {
		return "Expected " + a.MuPP(obj) + " to not respond to " + meth
	})
	return a.assertBool(!a.rt.RespondTo(obj, meth, includeAll), full)
}

// AssertMatch is assert_match. A String matcher is promoted to a Regexp first
// (so the message shows the original matcher's inspect). It asserts matcher
// responds to :=~, then asserts the match.
func (a *Assertions) AssertMatch(matcher, obj Value, msg string) error {
	// The gem's message proc closes over the local `matcher`, which is reassigned
	// to the promoted Regexp BEFORE the proc is forced — so the failure shows the
	// Regexp inspect, not the original string. Promote up front and message off it.
	if err := a.AssertRespondTo(matcher, "=~", "", false); err != nil {
		return err
	}
	m := matcher
	if a.rt.IsString(matcher) {
		m = a.rt.StringToRegexp(matcher)
	}
	full := message(msg, "", func() string {
		return "Expected " + a.MuPP(m) + " to match " + a.MuPP(obj)
	})
	return a.assertBool(a.rt.Match(m, obj), full)
}

// RefuteMatch is refute_match.
func (a *Assertions) RefuteMatch(matcher, obj Value, msg string) error {
	if err := a.AssertRespondTo(matcher, "=~", "", false); err != nil {
		return err
	}
	m := matcher
	if a.rt.IsString(matcher) {
		m = a.rt.StringToRegexp(matcher)
	}
	full := message(msg, "", func() string {
		return "Expected " + a.MuPP(m) + " to not match " + a.MuPP(obj)
	})
	return a.assertBool(!a.rt.Match(m, obj), full)
}

// AssertSame is assert_same: fails unless exp.equal?(act).
func (a *Assertions) AssertSame(exp, act Value, msg string) error {
	full := message(msg, "", func() string {
		return "Expected " + a.MuPP(act) + " (oid=" + strconv.FormatInt(a.rt.ObjectID(act), 10) +
			") to be the same as " + a.MuPP(exp) + " (oid=" + strconv.FormatInt(a.rt.ObjectID(exp), 10) + ")"
	})
	return a.assertBool(a.rt.Same(exp, act), full)
}

// RefuteSame is refute_same.
func (a *Assertions) RefuteSame(exp, act Value, msg string) error {
	full := message(msg, "", func() string {
		return "Expected " + a.MuPP(act) + " (oid=" + strconv.FormatInt(a.rt.ObjectID(act), 10) +
			") to not be the same as " + a.MuPP(exp) + " (oid=" + strconv.FormatInt(a.rt.ObjectID(exp), 10) + ")"
	})
	return a.assertBool(!a.rt.Same(exp, act), full)
}

// AssertInDelta is assert_in_delta. n = |exp - act|; fails unless delta >= n. The
// message interpolates exp/act/n/delta via Ruby #to_s, which the host renders by
// formatting the float values; here exp/act/delta/n are Go float64.
func (a *Assertions) AssertInDelta(exp, act, delta float64, msg string) error {
	n := math.Abs(exp - act)
	full := message(msg, "", func() string {
		return "Expected |" + f(exp) + " - " + f(act) + "| (" + f(n) + ") to be <= " + f(delta)
	})
	return a.assertBool(delta >= n, full)
}

// RefuteInDelta is refute_in_delta.
func (a *Assertions) RefuteInDelta(exp, act, delta float64, msg string) error {
	n := math.Abs(exp - act)
	full := message(msg, "", func() string {
		return "Expected |" + f(exp) + " - " + f(act) + "| (" + f(n) + ") to not be <= " + f(delta)
	})
	return a.assertBool(!(delta >= n), full)
}

// AssertInEpsilon is assert_in_epsilon: delegates to assert_in_delta with
// delta = min(|exp|, |act|) * epsilon.
func (a *Assertions) AssertInEpsilon(exp, act, epsilon float64, msg string) error {
	delta := math.Min(math.Abs(exp), math.Abs(act)) * epsilon
	return a.AssertInDelta(exp, act, delta, msg)
}

// RefuteInEpsilon is refute_in_epsilon: delegates to refute_in_delta with
// delta = exp * epsilon (note: the gem uses a*epsilon, not min, for refute).
func (a *Assertions) RefuteInEpsilon(exp, act, epsilon float64, msg string) error {
	return a.RefuteInDelta(exp, act, exp*epsilon, msg)
}

// AssertOperator is assert_operator: fails unless o1.__send__(op, o2). When o2 is
// the [UNDEFINED] sentinel it delegates to assert_predicate.
func (a *Assertions) AssertOperator(o1 Value, op string, o2 Value, msg string) error {
	if o2 == UNDEFINED {
		return a.AssertPredicate(o1, op, msg)
	}
	full := message(msg, "", func() string {
		return "Expected " + a.MuPP(o1) + " to be " + op + " " + a.MuPP(o2)
	})
	return a.assertBool(a.rt.Truthy(a.rt.Send(o1, op, o2)), full)
}

// RefuteOperator is refute_operator.
func (a *Assertions) RefuteOperator(o1 Value, op string, o2 Value, msg string) error {
	if o2 == UNDEFINED {
		return a.RefutePredicate(o1, op, msg)
	}
	full := message(msg, "", func() string {
		return "Expected " + a.MuPP(o1) + " to not be " + op + " " + a.MuPP(o2)
	})
	return a.assertBool(!a.rt.Truthy(a.rt.Send(o1, op, o2)), full)
}

// AssertPredicate is assert_predicate: fails unless o1.__send__(op).
func (a *Assertions) AssertPredicate(o1 Value, op, msg string) error {
	full := message(msg, "", func() string {
		return "Expected " + a.MuPP(o1) + " to be " + op
	})
	return a.assertBool(a.rt.Truthy(a.rt.Send(o1, op)), full)
}

// RefutePredicate is refute_predicate.
func (a *Assertions) RefutePredicate(o1 Value, op, msg string) error {
	full := message(msg, "", func() string {
		return "Expected " + a.MuPP(o1) + " to not be " + op
	})
	return a.assertBool(!a.rt.Truthy(a.rt.Send(o1, op)), full)
}

// UNDEFINED is Minitest::Assertions::UNDEFINED, the sentinel distinguishing
// assert_operator's 2-arg (predicate) and 3-arg (operator) forms. Its inspect is
// "UNDEFINED".
var UNDEFINED = &undefined{}

type undefined struct{}

// Inspect renders the sentinel as "UNDEFINED".
func (*undefined) Inspect() string { return "UNDEFINED" }

// f renders a Go float64 the way Ruby's Float#to_s does, which the delta/epsilon
// messages interpolate. Ruby uses the shortest decimal that round-trips and
// prints it in positional notation unless the decimal point position falls
// outside (decpt < -4 || decpt > 15) — i.e. the decimal exponent is < -4 or
// >= 15 — in which case it switches to "d.dddde±NN" with at least two exponent
// digits and a mandatory fractional part. strconv's shortest 'e'/'f' give the
// same digits; this function only reshapes them to Ruby's thresholds/notation.
func f(v float64) string {
	switch {
	case math.IsInf(v, 1):
		return "Infinity"
	case math.IsInf(v, -1):
		return "-Infinity"
	case math.IsNaN(v):
		return "NaN"
	case v == 0:
		if math.Signbit(v) {
			return "-0.0"
		}
		return "0.0"
	}

	// Shortest scientific form gives us the decimal exponent and mantissa digits.
	sci := strconv.FormatFloat(v, 'e', -1, 64) // e.g. "1.2345675e+06"
	mant, expStr, _ := strings.Cut(sci, "e")
	exp, _ := strconv.Atoi(expStr)

	if exp < -4 || exp >= 15 {
		// Ruby's exponential notation: keep the shortest mantissa, ensure it has a
		// fractional part, and render the exponent with a sign and >=2 digits.
		if !strings.Contains(mant, ".") {
			mant += ".0"
		}
		sign := "+"
		e := exp
		if e < 0 {
			sign = "-"
			e = -e
		}
		es := strconv.Itoa(e)
		if len(es) < 2 {
			es = "0" + es
		}
		return mant + "e" + sign + es
	}

	// Positional notation with a mandatory ".0" for integral values.
	s := strconv.FormatFloat(v, 'f', -1, 64)
	if !strings.Contains(s, ".") {
		s += ".0"
	}
	return s
}
