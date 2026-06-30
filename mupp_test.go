// Copyright (c) the go-ruby-minitest/minitest authors
//
// SPDX-License-Identifier: BSD-3-Clause

package minitest

import "testing"

// encRT overrides Encoding/DefaultExternalEncoding to exercise the mu_pp
// annotation branch.
type encRT struct {
	fakeRT
	enc      string
	valid    bool
	external string
}

func (e encRT) Encoding(obj Value) (string, bool) { return e.enc, e.valid }
func (e encRT) DefaultExternalEncoding() string   { return e.external }
func (e encRT) IsString(obj Value) bool           { _, ok := obj.(rStr); return ok }

func TestMuPP(t *testing.T) {
	a := newA()
	if got := a.MuPP(rInt(5)); got != "5" {
		t.Errorf("mu_pp int = %q", got)
	}
	if got := a.MuPP(rStr("hi")); got != "\"hi\"" {
		t.Errorf("mu_pp str = %q", got)
	}
	// Default-encoding string: no annotation.
	if got := a.MuPP(rStr("x")); got != "\"x\"" {
		t.Errorf("mu_pp default-enc = %q", got)
	}
}

func TestMuPPEncodingAnnotation(t *testing.T) {
	// A string whose encoding differs from default external gets the annotation.
	a := NewAssertions(encRT{enc: "ASCII-8BIT", valid: true, external: "UTF-8"})
	got := a.MuPP(rStr("x"))
	want := "# encoding: ASCII-8BIT\n#    valid: true\n\"x\""
	if got != want {
		t.Errorf("mu_pp annotated = %q, want %q", got, want)
	}
	// Invalid encoding (even if matching external) also annotates.
	a2 := NewAssertions(encRT{enc: "UTF-8", valid: false, external: "UTF-8"})
	got2 := a2.MuPP(rStr("x"))
	want2 := "# encoding: UTF-8\n#    valid: false\n\"x\""
	if got2 != want2 {
		t.Errorf("mu_pp invalid = %q, want %q", got2, want2)
	}
}

// inspRT lets a test inject an arbitrary inspect string to drive mu_pp_for_diff.
type inspRT struct {
	fakeRT
	insp     string
	isString bool
}

func (r inspRT) Inspect(obj Value) string { return r.insp }
func (r inspRT) IsString(obj Value) bool  { return r.isString }

func TestMuPPForDiff(t *testing.T) {
	cases := []struct {
		name string
		insp string
		want string
	}{
		// No newline escapes: unchanged (hex anonymized only).
		{"plain", `"abc"`, `"abc"`},
		// Single-escaped \n only → unescape to real newlines.
		{"single", `"a\nb"`, "\"a\nb\""},
		// Hex object ids anonymized.
		{"hex", `#<Foo:0x00007f9a>`, `#<Foo:0xXXXXXX>`},
		// Both single and double present → left alone (itself branch).
		{"both", `"a\nb\\nc"`, `"a\nb\\nc"`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			a := NewAssertions(inspRT{insp: c.insp})
			if got := a.MuPPForDiff(rStr("x")); got != c.want {
				t.Errorf("mu_pp_for_diff(%q) = %q, want %q", c.insp, got, c.want)
			}
		})
	}
}

func TestMuPPForDiffDoubleEscape(t *testing.T) {
	// Double-escaped "\\n" only (the string itself begins with a backslash run):
	// the gem unescapes a bit and adds newlines.
	a := NewAssertions(inspRT{insp: `\\n`})
	got := a.MuPPForDiff(rStr("x"))
	if got != "\\n\n" {
		t.Errorf("double-escape = %q, want %q", got, "\\n\n")
	}
}

func TestClassifyEscapes(t *testing.T) {
	if s, d := classifyEscapes(`a\nb`); !s || d {
		t.Errorf("single: s=%v d=%v", s, d)
	}
	if s, d := classifyEscapes(`\\n`); s || !d {
		t.Errorf("double: s=%v d=%v", s, d)
	}
	if s, d := classifyEscapes(`abc`); s || d {
		t.Errorf("none: s=%v d=%v", s, d)
	}
}

func TestEmit(t *testing.T) {
	// Matching run is replaced; a non-matching run passes through unchanged. The
	// latter is the gem proc's `: s` arm, reachable when the two escape forms
	// overlap at a token boundary within one branch.
	if got := emit(`\n`, `\n`, "\n"); got != "\n" {
		t.Errorf("emit match = %q", got)
	}
	if got := emit(`\\n`, `\n`, "\n"); got != `\\n` {
		t.Errorf("emit passthrough = %q", got)
	}
}

func TestReplaceEscapesMixedRuns(t *testing.T) {
	// A run that does not equal the single-branch target ("\\n") is left intact
	// while the target ("\n") is replaced — exercising emit's both arms via the
	// scanner. Plain text and trailing lone backslashes pass through verbatim.
	got := replaceEscapes(`a\nb\\nc\`, `\n`, "\n")
	want := "a\nb" + `\\nc\`
	if got != want {
		t.Errorf("replaceEscapes mixed = %q, want %q", got, want)
	}
}

func TestBoolStr(t *testing.T) {
	if boolStr(true) != "true" || boolStr(false) != "false" {
		t.Errorf("boolStr broken")
	}
}
