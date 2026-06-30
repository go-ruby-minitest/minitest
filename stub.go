// Copyright (c) the go-ruby-minitest/minitest authors
//
// SPDX-License-Identifier: BSD-3-Clause

package minitest

// StubHarness is the seam the host implements to stub a single method on an
// object for the duration of a block (Object#stub). The library owns only the
// install → run → restore orchestration (the Ruby method's `ensure`); the host
// performs the actual singleton-method alias/define/undef and runs the user block.
type StubHarness interface {
	// Install replaces the method named by the stub: it aliases the original
	// under "__minitest_stub__<name>" and defines a replacement that, when the
	// stub value responds to :call, calls it, else returns the value as-is
	// (yielding block_args to a passed block when given). Returns an error if the
	// method does not already exist (the gem requires it).
	Install() error
	// RunBlock runs the user block with the (possibly stubbed) receiver as its
	// argument (block[self]) and returns its error/exception, if any.
	RunBlock() error
	// Restore undoes Install: undef the replacement, alias the original back, and
	// undef the temp alias. It must run even when RunBlock errors (the ensure).
	Restore()
}

// Stub reproduces Object#stub's control flow: Install the stub, run the block,
// and ALWAYS Restore afterward (the Ruby `ensure`), even if Install's
// precondition fails or the block raises. The block's error (or Install's) is
// returned; Restore's effect is unconditional.
func Stub(h StubHarness) (err error) {
	if ierr := h.Install(); ierr != nil {
		// The method must exist before stubbing; nothing was installed, but the
		// gem's ensure still fires its alias/undef dance against the temp name. The
		// host's Restore is written to tolerate a failed Install (no-op), so we
		// still call it for symmetry.
		defer h.Restore()
		return ierr
	}
	defer h.Restore()
	return h.RunBlock()
}
