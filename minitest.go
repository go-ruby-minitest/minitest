// Copyright (c) the go-ruby-minitest/minitest authors
//
// SPDX-License-Identifier: BSD-3-Clause

// Package minitest is a pure-Go (CGO-free) re-implementation of the CORE of
// Ruby's Minitest test framework (targeting Minitest 5.x): the assertion layer,
// the per-test run lifecycle, result aggregation, the spec DSL → assertion
// mapping, and the Mock / Stub object framework.
//
// It is the deterministic, interpreter-independent heart of Minitest — the part
// that, given a Ruby runtime to supply value semantics, produces the exact same
// assertion failure messages, mu_pp inspection output, and run-result counts as
// the minitest gem, byte-for-byte. A host such as go-embedded-ruby (rbgo) binds
// it so that real Minitest test suites run on a pure-Go Ruby.
//
// # What this package owns
//
//   - [Assertions]: every assert_*/refute_* method, flunk, skip, pass — and, the
//     load-bearing part, the EXACT failure message each produces (the
//     "Expected: 1\n  Actual: 2" of assert_equal, the mu_pp inspection, the
//     custom-message prepend via [Assertions.Message]).
//   - The run lifecycle: [Test.Run] drives before_setup/setup/after_setup → the
//     test body → before_teardown/teardown/after_teardown, captures exceptions
//     into failures, and produces a [Result].
//   - [Result]: assertion count, failures / errors / skips, result_code,
//     result_label, location, and the to_s rendering.
//   - The exception model: [Assertion], [Skip], [UnexpectedError].
//   - The spec DSL surface: describe/it/before/after and the must_*/wont_*
//     expectation → assertion mapping ([Expectations]).
//   - [Mock] (expect / verify) and [Stub] (stub a method for a block), including
//     the exact MockExpectationError / ArgumentError messages.
//
// # What is a seam (supplied by the host)
//
// Everything that requires genuine Ruby object semantics is funneled through the
// [Runtime] interface, which the host (rbgo) implements over its own object
// graph:
//
//   - value inspection (#inspect), equality (#==), identity (#equal? / object_id),
//     regexp match (#=~), #respond_to?, #include?, #empty?, #nil?, #instance_of?,
//     #kind_of?, class name, truthiness, and arbitrary #__send__ for operator /
//     predicate assertions.
//
// The IO captured by assert_output / assert_silent and the execution of test
// method BODIES and assertion blocks are also seams — the host wires those to
// the Ruby VM. This package never evaluates Ruby; it only formats, orchestrates,
// and aggregates.
package minitest

// VERSION is the Minitest version this package targets and reports. The failure
// messages, mu_pp output, and run-result semantics reproduced here are the 5.x
// ones (assert_equal nil deprecation-warns rather than failing, etc.).
const VERSION = "5.25.5"
