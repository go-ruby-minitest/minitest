// Copyright (c) the go-ruby-minitest/minitest authors
//
// SPDX-License-Identifier: BSD-3-Clause

package minitest

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// This file provides a small, Ruby-faithful value model and a [Runtime] /
// [MockMatcher] implementation over it, so the deterministic (ruby-free) tests
// can drive every assertion and reproduce the gem's messages without a Ruby VM.
// It is also the reference the oracle test reuses: the same fakeRT produces the
// strings the oracle compares against the live minitest gem.

// Ruby value shims. Each carries just enough to answer the seam queries.
type (
	rNilT  struct{}
	rBool  bool
	rInt   int64
	rFloat float64
	rStr   string
	rSym   string
	rArr   []Value
	rClass string               // a class reference (its name)
	rReg   struct{ src string } // a regexp (its source, no flags)
	rObj   struct {             // an opaque object with identity + class
		id    int64
		class string
		insp  string
	}
)

var rNil = rNilT{}

// fakeRT implements [Runtime] over the shims above with MRI-faithful semantics.
type fakeRT struct{}

func (fakeRT) Inspect(obj Value) string { return inspectV(obj) }

func inspectV(obj Value) string {
	switch v := obj.(type) {
	case rNilT:
		return "nil"
	case nil:
		return "nil"
	case rBool:
		if v {
			return "true"
		}
		return "false"
	case rInt:
		return strconv.FormatInt(int64(v), 10)
	case rFloat:
		return f(float64(v))
	case rStr:
		return rubyStrInspect(string(v))
	case rSym:
		return ":" + string(v)
	case rArr:
		parts := make([]string, len(v))
		for i, e := range v {
			parts[i] = inspectV(e)
		}
		return "[" + strings.Join(parts, ", ") + "]"
	case rClass:
		return string(v)
	case rReg:
		return "/" + v.src + "/"
	case rObj:
		return v.insp
	case *undefined:
		return "UNDEFINED"
	default:
		return fmt.Sprintf("%v", obj)
	}
}

func rubyStrInspect(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		case '\n':
			b.WriteString(`\n`)
		case '\t':
			b.WriteString(`\t`)
		case '\r':
			b.WriteString(`\r`)
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
	return b.String()
}

func (fakeRT) Encoding(obj Value) (string, bool) { return "UTF-8", true }
func (fakeRT) DefaultExternalEncoding() string   { return "UTF-8" }

func (fakeRT) IsString(obj Value) bool { _, ok := obj.(rStr); return ok }

func (fakeRT) Equal(a, b Value) bool { return eqV(a, b) }

func eqV(a, b Value) bool {
	// Normalize nil shims.
	a, b = norm(a), norm(b)
	return fmt.Sprintf("%T%v", a, a) == fmt.Sprintf("%T%v", b, b)
}

func norm(v Value) Value {
	if v == nil {
		return rNil
	}
	return v
}

func (fakeRT) Same(a, b Value) bool {
	oa, aok := a.(rObj)
	ob, bok := b.(rObj)
	if aok && bok {
		return oa.id == ob.id
	}
	// Non-object scalars: identity == equality for our immutable shims, except
	// strings, which are distinct objects unless the very same value pointer —
	// our tests use rObj for identity cases, so fall back to equality here.
	return eqV(a, b)
}

func (fakeRT) ObjectID(obj Value) int64 {
	if o, ok := obj.(rObj); ok {
		return o.id
	}
	return 0
}

func (fakeRT) Truthy(obj Value) bool {
	switch v := norm(obj).(type) {
	case rNilT:
		return false
	case rBool:
		return bool(v)
	default:
		return true
	}
}

func (fakeRT) IsNil(obj Value) bool { _, ok := norm(obj).(rNilT); return ok }

func (fakeRT) Match(matcher, obj Value) bool {
	reg, ok := matcher.(rReg)
	if !ok {
		return false
	}
	s, ok := obj.(rStr)
	if !ok {
		return false
	}
	m, err := regexp.Compile(reg.src)
	if err != nil {
		return false
	}
	return m.MatchString(string(s))
}

func (fakeRT) StringToRegexp(s Value) Value {
	return rReg{src: regexp.QuoteMeta(string(s.(rStr)))}
}

func (fakeRT) RespondTo(obj Value, meth string, includeAll bool) bool {
	switch obj.(type) {
	case rArr:
		return meth == "include?" || meth == "empty?" || meth == "to_s"
	case rStr:
		return meth == "empty?" || meth == "=~" || meth == "to_s" || meth == "include?"
	case rReg:
		return meth == "=~" || meth == "to_s"
	case rObj:
		return meth == "to_s"
	default:
		return meth == "to_s"
	}
}

func (fakeRT) Includes(collection, obj Value) bool {
	switch c := collection.(type) {
	case rArr:
		for _, e := range c {
			if eqV(e, obj) {
				return true
			}
		}
	case rStr:
		if s, ok := obj.(rStr); ok {
			return strings.Contains(string(c), string(s))
		}
	}
	return false
}

func (fakeRT) Empty(obj Value) bool {
	switch c := obj.(type) {
	case rArr:
		return len(c) == 0
	case rStr:
		return c == ""
	}
	return false
}

func (fakeRT) InstanceOf(obj, cls Value) bool { return className(obj) == string(cls.(rClass)) }

func (fakeRT) KindOf(obj, cls Value) bool {
	name := string(cls.(rClass))
	if className(obj) == name {
		return true
	}
	// Integer/Float are kinds of Numeric; String is a kind of Comparable, etc. —
	// keep a tiny hierarchy for the tests.
	switch className(obj) {
	case "Integer", "Float":
		return name == "Numeric" || name == "Object"
	default:
		return name == "Object"
	}
}

func (fakeRT) ClassName(obj Value) string { return className(obj) }

func className(obj Value) string {
	switch v := norm(obj).(type) {
	case rNilT:
		return "NilClass"
	case rBool:
		if v {
			return "TrueClass"
		}
		return "FalseClass"
	case rInt:
		return "Integer"
	case rFloat:
		return "Float"
	case rStr:
		return "String"
	case rSym:
		return "Symbol"
	case rArr:
		return "Array"
	case rReg:
		return "Regexp"
	case rClass:
		return "Class"
	case rObj:
		return v.class
	default:
		return "Object"
	}
}

func (fakeRT) Name(cls Value) string {
	if c, ok := cls.(rClass); ok {
		return string(c)
	}
	return inspectV(cls)
}

func (fakeRT) Send(obj Value, op string, args ...Value) Value {
	switch op {
	case "empty?":
		return rBool(fakeRT{}.Empty(obj))
	case "<":
		return rBool(toF(obj) < toF(args[0]))
	case "<=":
		return rBool(toF(obj) <= toF(args[0]))
	case ">":
		return rBool(toF(obj) > toF(args[0]))
	case ">=":
		return rBool(toF(obj) >= toF(args[0]))
	case "==":
		return rBool(eqV(obj, args[0]))
	case "even?":
		return rBool(int64(obj.(rInt))%2 == 0)
	}
	return rNil
}

func toF(v Value) float64 {
	switch n := v.(type) {
	case rInt:
		return float64(n)
	case rFloat:
		return float64(n)
	}
	return 0
}

// fakeMatcher implements [MockMatcher] over the shims.
type fakeMatcher struct {
	// blocks holds the per-expect block predicate, indexed by expect order.
	blocks map[int]func(args []Value, kwargs []KV) bool
}

func newFakeMatcher() *fakeMatcher {
	return &fakeMatcher{blocks: map[int]func([]Value, []KV) bool{}}
}

func (m *fakeMatcher) Match(expected, actual Value) bool {
	if _, ok := expected.(anyClass); ok {
		return true // Object === x
	}
	if c, ok := expected.(rClass); ok {
		return className(actual) == string(c) || classKindOf(actual, string(c))
	}
	return eqV(expected, actual)
}

func classKindOf(actual Value, name string) bool {
	switch className(actual) {
	case "Integer", "Float":
		return name == "Numeric"
	}
	return name == "Object"
}

func (m *fakeMatcher) Inspect(v Value) string { return inspectV(v) }

func (m *fakeMatcher) CallBlock(idx int, args []Value, kwargs []KV) bool {
	if fn, ok := m.blocks[idx]; ok {
		return fn(args, kwargs)
	}
	return true
}

// sortedSyms is a tiny helper used by a couple of tests.
func sortedSyms(ss []string) []string {
	out := append([]string{}, ss...)
	sort.Strings(out)
	return out
}
