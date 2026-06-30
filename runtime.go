// Copyright (c) the go-ruby-minitest/minitest authors
//
// SPDX-License-Identifier: BSD-3-Clause

package minitest

// Value is an opaque Ruby value handed across the seam. This package never
// inspects its concrete shape; it only passes it back to the [Runtime] for the
// Ruby-semantic operations (inspect, ==, =~, …). A host represents it however it
// likes (its own object pointer, an interface, etc.).
type Value = any

// Runtime is the seam through which this package obtains genuine Ruby value
// semantics. The host (rbgo) implements it over its object graph. Every method
// corresponds to a Ruby message the assertion layer would otherwise have sent to
// a Ruby object; keeping them here makes the assertion logic pure and the value
// semantics the host's responsibility.
//
// All methods must be deterministic for a given pair of arguments so the failure
// messages this package builds are reproducible.
type Runtime interface {
	// Inspect returns obj.inspect — the Ruby #inspect string. This is the raw
	// material of mu_pp; the package layers encoding annotations and diff cleanup
	// on top of it (see [Assertions.MuPP]).
	Inspect(obj Value) string

	// Encoding returns the name of obj's string encoding (e.g. "UTF-8") and
	// whether the string is validly encoded. It is consulted by mu_pp only for
	// String values whose encoding differs from the default external encoding or
	// which are invalid. For non-strings the result is ignored.
	Encoding(obj Value) (name string, valid bool)

	// DefaultExternalEncoding returns Encoding.default_external's name. mu_pp
	// annotates a String whose encoding differs from this.
	DefaultExternalEncoding() string

	// IsString reports whether obj is a Ruby String (String === obj). Used by
	// mu_pp (encoding annotation) and by assert_match (string→regexp promotion).
	IsString(obj Value) bool

	// Equal reports obj == other (Ruby #==). Drives assert_equal / refute_equal.
	Equal(a, b Value) bool

	// Same reports a.equal?(b) — Ruby object identity. Drives assert_same /
	// refute_same.
	Same(a, b Value) bool

	// ObjectID returns obj.object_id, used verbatim in the assert_same /
	// refute_same failure message.
	ObjectID(obj Value) int64

	// Truthy reports Ruby truthiness: false only for nil and false.
	Truthy(obj Value) bool

	// IsNil reports obj.nil?.
	IsNil(obj Value) bool

	// Match returns the truthiness of (matcher =~ obj). The host evaluates the
	// Ruby =~ operator (Regexp#=~, String#=~, or a custom one). assert_match has
	// already promoted a bare String matcher to a Regexp before calling, by way
	// of [Runtime.StringToRegexp].
	Match(matcher, obj Value) bool

	// StringToRegexp builds Regexp.new(Regexp.escape(s)) for a String matcher, so
	// assert_match / refute_match match the gem's literal-string promotion.
	StringToRegexp(s Value) Value

	// RespondTo reports obj.respond_to?(meth, includeAll).
	RespondTo(obj Value, meth string, includeAll bool) bool

	// Includes reports collection.include?(obj).
	Includes(collection, obj Value) bool

	// Empty reports obj.empty?.
	Empty(obj Value) bool

	// InstanceOf reports obj.instance_of?(cls).
	InstanceOf(obj, cls Value) bool

	// KindOf reports obj.kind_of?(cls).
	KindOf(obj, cls Value) bool

	// ClassName returns obj.class.name — used in several messages (e.g.
	// assert_instance_of's "not #{obj.class}", assert_respond_to's "(#{obj.class})").
	ClassName(obj Value) string

	// Name returns the printable name of a class/module Value (its #to_s / #name),
	// used where the gem interpolates a bare class such as assert_instance_of's
	// "to be an instance of #{cls}".
	Name(cls Value) string

	// Send invokes obj.__send__(op, args...) and returns the result, used by
	// assert_operator / assert_predicate (and their refute_ twins). op is the
	// Ruby method name (e.g. "<=", "empty?").
	Send(obj Value, op string, args ...Value) Value
}
