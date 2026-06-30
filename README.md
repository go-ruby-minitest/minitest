<p align="center"><img src="https://raw.githubusercontent.com/go-ruby-minitest/brand/main/social/go-ruby-minitest-minitest.png" alt="go-ruby-minitest/minitest" width="720"></p>

# minitest — go-ruby-minitest

[![Docs](https://img.shields.io/badge/docs-mkdocs--material-DC2626)](https://go-ruby-minitest.github.io/docs/)
[![License](https://img.shields.io/badge/license-BSD--3--Clause-blue)](LICENSE)
[![Go](https://img.shields.io/badge/go-1.26.4%2B-00ADD8)](https://go.dev/dl/)
[![Coverage](https://img.shields.io/badge/coverage-100%25-1a7f37)](#tests--coverage)

**A pure-Go (no cgo) reimplementation of the core of Ruby's
[Minitest](https://github.com/minitest/minitest) test framework** (targeting
Minitest 5.x) — the assertion layer, the per-test run lifecycle, result
aggregation, the spec DSL → assertion mapping, and the Mock / Stub object
framework. It is the deterministic, interpreter-independent heart of Minitest:
given a Ruby runtime to supply value semantics, it produces the **exact same**
assertion failure messages, `mu_pp` inspection output, and run-result counts as
the `minitest` gem, byte-for-byte.

It is the Minitest backend for
[go-embedded-ruby](https://github.com/go-embedded-ruby/ruby) (rbgo) — binding it
lets real Minitest test suites run on a pure-Go Ruby — but is a **standalone,
reusable** module with no dependency on the Ruby runtime, a sibling of
[go-ruby-regexp](https://github.com/go-ruby-regexp/regexp),
[go-ruby-erb](https://github.com/go-ruby-erb/erb), and
[go-ruby-yaml](https://github.com/go-ruby-yaml/yaml).

> **What it is — and isn't.** The load-bearing part of Minitest is the *wording*:
> the `"Expected: 1\n  Actual: 2"` of `assert_equal`, the `mu_pp` inspection, the
> custom-message prepend, the mock-verify diagnostics. That formatting, the run
> orchestration (`before_setup`/`setup`/… → body → teardown), result coding, and
> mock bookkeeping are fully deterministic and live here as pure Go. Evaluating
> the test *bodies* and the value-level operations an assertion compares with
> (`#==`, `#=~`, `#inspect`, `#include?`, `#respond_to?`, …) are the host's job,
> funnelled through one explicit [`Runtime`](runtime.go) seam.

## Features

A faithful port of Minitest 5.x's core, validated against the `minitest` gem on
every supported platform:

- **`Assertions`** — `assert`/`refute`, `assert_equal`/`refute_equal`,
  `assert_nil`, `assert_empty`, `assert_includes`, `assert_match`/`refute_match`,
  `assert_instance_of`/`assert_kind_of`, `assert_respond_to`, `assert_raises`,
  `assert_throws`, `assert_in_delta`/`assert_in_epsilon`, `assert_operator`,
  `assert_predicate`, `assert_same`/`refute_same`, `assert_output`,
  `assert_silent`, `flunk`, `skip`, `pass` — each with its **byte-exact** failure
  message, `mu_pp` / `mu_pp_for_diff`, the `message`/`msg` prepend, and the
  assertion counter.
- **Run lifecycle** — `Test#run` driving `before_setup`/`setup`/`after_setup` →
  the test body → `before_teardown`/`teardown`/`after_teardown`, capturing
  exceptions into failures (one `capture_exceptions` for setup+body, one per
  teardown hook), and producing a `Result`.
- **`Result`** — assertion count, failures / errors / skips, `result_code`,
  `result_label`, `location`, and the `to_s` rendering, with the `Assertion` /
  `Skip` / `UnexpectedError` exception model.
- **Spec DSL** — `describe`/`it`/`before`/`after`, the `it`-name morphing
  (`test_%04d_%s`), `let` name validation, `spec_type` selection, and the full
  `must_*`/`wont_*` → assertion mapping table with each method's flip rule.
- **`Mock`** (expect / verify) and **`Stub`** (stub a method for a block),
  including the exact `MockExpectationError` / `ArgumentError` / `NoMethodError`
  messages and the under-called-vs-never-called verify distinction.

CGO-free, dependency-free, **100% test coverage**, `gofmt` + `go vet` clean, and
green across the six 64-bit Go targets (amd64, arm64, riscv64, loong64, ppc64le,
s390x) and three OSes (Linux, macOS, Windows).

## Install

```sh
go get github.com/go-ruby-minitest/minitest
```

## Usage

A host implements the [`Runtime`](runtime.go) seam over its own Ruby value model;
the library then produces gem-identical messages and run results.

```go
package main

import (
	"fmt"

	"github.com/go-ruby-minitest/minitest"
)

func main() {
	a := minitest.NewAssertions(myRuntime{}) // myRuntime implements Runtime

	// A failing assert_equal yields the gem's exact message.
	err, _ := a.AssertEqual(rInt(1), rInt(2), "")
	fmt.Println(err)
	// Expected: 1
	//   Actual: 2

	// Drive a test's lifecycle and aggregate the result.
	res, _ := minitest.RunTest(myBody{}, elapsed)
	fmt.Println(res.ResultCode(), res.Passed(), res.Assertions)
}
```

## The seams (what the host wires)

Everything requiring genuine Ruby semantics is a seam; the library owns only the
pure formatting / orchestration / aggregation:

| Seam | Interface | What the host supplies |
| ---- | --------- | ---------------------- |
| Value semantics | [`Runtime`](runtime.go) | `#inspect`, `#==`, `#equal?`/`object_id`, `#=~`, `#respond_to?`, `#include?`, `#empty?`, `#nil?`, `#instance_of?`/`#kind_of?`, class name, truthiness, `#__send__` |
| Test body & hooks | [`TestBody`](lifecycle.go) | invoke a named setup/teardown hook or `test_*` body in the VM; report the captured exception |
| `assert_raises` / `assert_throws` / `assert_output` | [`RaiseOutcome`](blocks.go) / `ThrowOutcome` / per-stream results | run the block, classify what it raised / threw / printed |
| Mock matching | [`MockMatcher`](mock.go) | Ruby case-equality (`===`/`==`), `#inspect`, and `expect`-block invocation |
| Stub | [`StubHarness`](stub.go) | alias/define/undef the singleton method; run the user block |

## API

```go
// Assertions — every assert_*/refute_* returns nil on pass or a *Assertion/*Skip
// describing the failure; the host raises it and the lifecycle captures it.
func NewAssertions(rt Runtime) *Assertions
func (a *Assertions) AssertEqual(exp, act Value, msg string) (err error, deprecated bool)
func (a *Assertions) AssertNil/AssertEmpty/AssertIncludes/AssertMatch/…(…) error
func (a *Assertions) MuPP(obj Value) string          // mu_pp
func (a *Assertions) MuPPForDiff(obj Value) string   // mu_pp_for_diff

// Lifecycle — RunTest reproduces Test#run and returns a *Result (Result.from).
func RunTest(body TestBody, elapsed float64) (res *Result, abort *Passthrough)
type Result struct { /* Klass, TestName, Assertions, Failures, Time, … */ }
func (r *Result) Passed/Skipped/Errored() bool
func (r *Result) ResultCode() string                 // ".", "F", "E", "S"
func (r *Result) Location(baseDir string) string
func (r *Result) String(baseDir string) string       // Result#to_s

// Spec DSL → assertion mapping.
func ItName(seq int, desc string) string             // "test_%04d_%s"
func ValidateLetName(name string, reserved []string) string
func LookupExpectation(method string) (Expectation, bool) // must_*/wont_* table

// Mock / Stub.
func NewMock(m MockMatcher) *Mock
func (m *Mock) Expect(name string, retval Value, args []Value, kwargs []KV, block bool) error
func (m *Mock) Call(name string, args []Value, kwargs []KV) (Value, error)
func (m *Mock) Verify() error
func Stub(h StubHarness) error
```

## Tests & coverage

The suite pairs deterministic, ruby-free tests (which alone hold coverage at
100%, so the qemu cross-arch and Windows lanes pass the gate) with a
**differential gem oracle**: every assertion's failure message, `mu_pp` output,
run-result tuple, and mock-verify diagnostic is computed here and compared
byte-for-byte against the live `minitest` 5.x gem. The oracle scripts
`$stdout.binmode` so Windows text-mode never pollutes the bytes, and skip
themselves where ruby or the 5.x gem is absent. (A developer whose system ruby is
too new for the 5.x gem can point the oracle at an unpacked 5.x tree via
`MINITEST5_LIB=…/lib`.)

```sh
COVERPKG=$(go list ./... | paste -sd, -)
go test -race -coverpkg="$COVERPKG" -coverprofile=cover.out ./...
go tool cover -func=cover.out | tail -1   # 100.0%
```

## License

BSD-3-Clause — see [LICENSE](LICENSE). Copyright the go-ruby-minitest/minitest authors.
