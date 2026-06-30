// Copyright (c) the go-ruby-minitest/minitest authors
//
// SPDX-License-Identifier: BSD-3-Clause

package minitest

// SetupMethods is Minitest::Test::SETUP_METHODS — the hooks run, in order,
// before the test body, all inside one capture_exceptions.
var SetupMethods = []string{"before_setup", "setup", "after_setup"}

// TeardownMethods is Minitest::Test::TEARDOWN_METHODS — the hooks run, in order,
// after the test body, EACH inside its own capture_exceptions.
var TeardownMethods = []string{"before_teardown", "teardown", "after_teardown"}

// TestBody is the seam the host implements to run one test method instance: it
// invokes a named method (a setup/teardown hook or the test body) in the Ruby VM.
// Invoke returns nil if the method completed normally, or an error describing the
// exception it raised. The error must be an [*Assertion] / [*Skip] when an
// assertion failed or a skip was requested, an [*UnexpectedError] for any other
// Ruby exception, or one of the passthrough sentinels (see [Passthrough]) for
// NoMemoryError / SignalException / SystemExit, which abort the run.
type TestBody interface {
	// Invoke calls the named instance method (e.g. "setup", "test_foo"). It
	// returns the captured exception, already classified, or nil on success.
	Invoke(method string) error
	// Name is the name of the test method this instance runs (the Ruby `name`).
	Name() string
	// ClassName is the test's class name, for Result/location.
	ClassName() string
	// Assertions returns the current assertion count to record into the Result.
	Assertions() int
	// SourceLocation returns the ["file", line] of the test method (Result.from).
	SourceLocation() (string, int)
}

// Passthrough wraps an exception that must abort the run rather than be recorded
// as a failure (NoMemoryError, SignalException, SystemExit). The host returns it
// from [TestBody.Invoke]; [RunTest] propagates it via the returned abort value.
type Passthrough struct{ Err error }

// Error implements error.
func (p *Passthrough) Error() string {
	if p.Err != nil {
		return p.Err.Error()
	}
	return "passthrough"
}

// Result is Minitest::Result — a marshalable snapshot of one test run. It holds
// the class/name, assertion count, the captured failures (in occurrence order),
// and timing. Its rendering and predicates match the gem.
type Result struct {
	Klass      string
	TestName   string
	Assertions int
	// Failures holds Assertion/Skip/UnexpectedError values captured during the
	// run, in the order capture_exceptions appended them.
	Failures []Reportable2
	Time     float64
	// SourceFile/SourceLine mirror Result#source_location.
	SourceFile string
	SourceLine int
}

// RunTest reproduces Minitest::Test#run: it runs the setup hooks and the body in
// one capture, then each teardown hook in its own capture, accumulating failures,
// and returns the [*Result] (Result.from self). If a passthrough exception
// occurs, abort is the wrapping [*Passthrough] and the partial result is still
// returned (Ruby would unwind, but the host decides what to do with abort).
//
// elapsed is the measured wall time for the whole run (time_it); the host clocks
// it around the call, or passes a deterministic value in tests.
func RunTest(body TestBody, elapsed float64) (res *Result, abort *Passthrough) {
	var failures []Reportable2

	capture := func(method string) (stop bool) {
		err := body.Invoke(method)
		if err == nil {
			return false
		}
		if p, ok := err.(*Passthrough); ok {
			abort = p
			return true
		}
		failures = append(failures, asReportable(err))
		return false
	}

	// SETUP_METHODS + body share one capture_exceptions: the first raise stops
	// the rest of that block.
	func() {
		for _, hook := range SetupMethods {
			if capture(hook) {
				return
			}
			if len(failures) > 0 {
				// An assertion/error in a setup hook stops the body (single
				// capture_exceptions unwinds the whole block).
				return
			}
		}
		capture(body.Name())
	}()

	// Each teardown hook gets its own capture_exceptions and always runs.
	for _, hook := range TeardownMethods {
		if abort != nil {
			break
		}
		capture(hook)
	}

	res = &Result{
		Klass:      body.ClassName(),
		TestName:   body.Name(),
		Assertions: body.Assertions(),
		Failures:   failures,
		Time:       elapsed,
	}
	res.SourceFile, res.SourceLine = body.SourceLocation()
	return res, abort
}

// asReportable narrows a captured run error to the Reportable2 contract. It is
// always an *Assertion, *Skip, or *UnexpectedError by construction.
func asReportable(err error) Reportable2 {
	if r, ok := err.(Reportable2); ok {
		return r
	}
	// Defensive: wrap any stray error as an UnexpectedError-shaped failure.
	return &UnexpectedError{
		Assertion:    Assertion{Msg: err.Error()},
		ErrorClass:   "RuntimeError",
		ErrorMessage: err.Error(),
	}
}

// Failure returns the first captured failure, or nil (Reportable#failure).
func (r *Result) Failure() Reportable2 {
	if len(r.Failures) == 0 {
		return nil
	}
	return r.Failures[0]
}

// Passed reports whether the run passed: no failure (Reportable#passed?). Skips
// are NOT passing.
func (r *Result) Passed() bool { return r.Failure() == nil }

// Skipped reports whether the (first) failure is a Skip (Reportable#skipped?).
func (r *Result) Skipped() bool {
	_, ok := r.Failure().(*Skip)
	return ok
}

// Errored reports whether any failure is an UnexpectedError (Reportable#error?).
func (r *Result) Errored() bool {
	for _, f := range r.Failures {
		if _, ok := f.(*UnexpectedError); ok {
			return true
		}
	}
	return false
}

// ResultCode returns ".", "F", "E", or "S" (Reportable#result_code): the failure's
// code, or "." when passed.
func (r *Result) ResultCode() string {
	f := r.Failure()
	if f == nil {
		return "."
	}
	switch v := f.(type) {
	case *Skip:
		return v.ResultCode()
	case *UnexpectedError:
		return v.ResultCode()
	case *Assertion:
		return v.ResultCode()
	default:
		return resultCode(f)
	}
}

// Location returns the test's location string (Reportable#location):
// "Class#name" plus " [file:line]" of the failure unless the run passed or
// errored. baseDir, when non-empty and a prefix of the failure location, is
// stripped (Ruby's delete_prefix BASE_DIR); the host passes Dir.pwd+"/".
func (r *Result) Location(baseDir string) string {
	base := r.Klass + "#" + r.TestName
	if r.Passed() || r.Errored() {
		return base
	}
	loc := r.Failure().Location()
	if baseDir != "" && len(loc) >= len(baseDir) && loc[:len(baseDir)] == baseDir {
		loc = loc[len(baseDir):]
	}
	return base + " [" + loc + "]"
}

// String renders Result#to_s: for a passed (non-skipped) run, just the location;
// otherwise each failure as "<label>:\n<location>:\n<message>\n", joined by "\n".
func (r *Result) String(baseDir string) string {
	if r.Passed() && !r.Skipped() {
		return r.Location(baseDir)
	}
	parts := make([]string, len(r.Failures))
	loc := r.Location(baseDir)
	for i, f := range r.Failures {
		parts[i] = f.ResultLabel() + ":\n" + loc + ":\n" + f.Message() + "\n"
	}
	return joinNL(parts)
}

func joinNL(parts []string) string {
	out := ""
	for i, p := range parts {
		if i > 0 {
			out += "\n"
		}
		out += p
	}
	return out
}
