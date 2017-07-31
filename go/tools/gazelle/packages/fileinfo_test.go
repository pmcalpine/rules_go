/* Copyright 2017 The Bazel Authors. All rights reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

   http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package packages

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/pmcalpine/rules_go/go/tools/gazelle/config"
)

func TestGoFileInfo(t *testing.T) {
	c := &config.Config{}
	dir := "."
	for _, tc := range []struct {
		desc, name, source string
		want               fileInfo
	}{
		{
			"empty file",
			"foo.go",
			"package foo\n",
			fileInfo{
				packageName: "foo",
			},
		},
		{
			"xtest file",
			"foo_test.go",
			"package foo_test\n",
			fileInfo{
				packageName: "foo",
				isTest:      true,
				isXTest:     true,
			},
		},
		{
			"xtest suffix on non-test",
			"foo_xtest.go",
			"package foo_test\n",
			fileInfo{
				packageName: "foo_test",
				isTest:      false,
				isXTest:     false,
			},
		},
		{
			"single import",
			"foo.go",
			`package foo

import "github.com/foo/bar"
`,
			fileInfo{
				packageName: "foo",
				imports:     []string{"github.com/foo/bar"},
			},
		},
		{
			"multiple imports",
			"foo.go",
			`package foo

import (
	"github.com/foo/bar"
	x "github.com/local/project/y"
)
`,
			fileInfo{
				packageName: "foo",
				imports:     []string{"github.com/foo/bar", "github.com/local/project/y"},
			},
		},
		{
			"standard imports not included",
			"foo.go",
			`package foo

import "fmt"
`,
			fileInfo{
				packageName: "foo",
			},
		},
		{
			"cgo",
			"foo.go",
			`package foo

import "C"
`,
			fileInfo{
				packageName: "foo",
				isCgo:       true,
			},
		},
		{
			"build tags",
			"foo.go",
			`// +build linux darwin

// +build !ignore

package foo
`,
			fileInfo{
				packageName: "foo",
				tags:        []string{"linux darwin", "!ignore"},
			},
		},
		{
			"build tags without blank line",
			"route.go",
			`// Copyright 2017

// +build darwin dragonfly freebsd netbsd openbsd

// Package route provides basic functions for the manipulation of
// packet routing facilities on BSD variants.
package route
`,
			fileInfo{
				packageName: "route",
				tags:        []string{"darwin dragonfly freebsd netbsd openbsd"},
			},
		},
	} {
		if err := ioutil.WriteFile(tc.name, []byte(tc.source), 0600); err != nil {
			t.Fatal(err)
		}
		defer os.Remove(tc.name)

		got, err := goFileInfo(c, dir, tc.name)
		if err != nil {
			t.Fatal(err)
		}

		// Clear fields we don't care about for testing.
		got = fileInfo{
			packageName: got.packageName,
			isTest:      got.isTest,
			isXTest:     got.isXTest,
			imports:     got.imports,
			isCgo:       got.isCgo,
			tags:        got.tags,
		}

		if !reflect.DeepEqual(got, tc.want) {
			t.Errorf("case %q: got %#v; want %#v", tc.desc, got, tc.want)
		}
	}
}

func TestGoFileInfoFailures(t *testing.T) {
	c := &config.Config{}
	dir := "."
	for _, tc := range []struct {
		desc, name, source, wantError string
	}{
		{
			"parse error",
			"foo.go",
			"pakcage foo",
			"expected 'package'",
		},
		{
			"cgo error",
			"foo.go",
			`package foo

// #cgo !
import "C"
`,
			"invalid #cgo line",
		},
		{
			"cgo in test",
			"foo_test.go",
			`package foo

import "C"
`,
			"use of cgo in test not supported",
		},
	} {
		if err := ioutil.WriteFile(tc.name, []byte(tc.source), 0600); err != nil {
			t.Fatal(err)
		}
		defer os.Remove(tc.name)

		var errorText string
		if _, err := goFileInfo(c, dir, tc.name); err != nil {
			errorText = err.Error()
		}

		if tc.wantError == "" && errorText != "" || tc.wantError != "" && !strings.Contains(errorText, tc.wantError) {
			t.Errorf("case %q: got error %q; want error containing %q", tc.desc, errorText, tc.wantError)
		}
	}
}

func TestOtherFileInfo(t *testing.T) {
	dir := "."
	for _, tc := range []struct {
		desc, name, source string
		wantTags           []string
	}{
		{
			"empty file",
			"foo.c",
			"",
			nil,
		},
		{
			"tags file",
			"foo.c",
			`// +build foo bar
// +build baz,!ignore

`,
			[]string{"foo bar", "baz,!ignore"},
		},
	} {
		if err := ioutil.WriteFile(tc.name, []byte(tc.source), 0600); err != nil {
			t.Fatal(err)
		}
		defer os.Remove(tc.name)

		got, err := otherFileInfo(dir, tc.name)
		if err != nil {
			t.Fatal(err)
		}

		// Only check that we can extract tags. Everything else is covered
		// by other tests.
		if !reflect.DeepEqual(got.tags, tc.wantTags) {
			t.Errorf("case %q: got %#v; want %#v", got.tags, tc.wantTags)
		}
	}
}

func TestOtherFileInfoFailures(t *testing.T) {
	dir := "."
	for _, tc := range []struct {
		desc, name, source, wantError string
	}{
		{
			"ignored file",
			"foo.txt",
			"",
			"",
		},
		{
			"unsupported file",
			"foo.m",
			"",
			"file extension not yet supported",
		},
	} {
		if err := ioutil.WriteFile(tc.name, []byte(tc.source), 0600); err != nil {
			t.Fatal(err)
		}
		defer os.Remove(tc.name)

		var errorText string
		if _, err := otherFileInfo(dir, tc.name); err != nil {
			errorText = err.Error()
		}

		if tc.wantError == "" && errorText != "" || tc.wantError != "" && !strings.Contains(errorText, tc.wantError) {
			t.Errorf("case %q: got error %q; want error containing %q", tc.desc, errorText, tc.wantError)
		}
	}
}

func TestFileNameInfo(t *testing.T) {
	for _, tc := range []struct {
		desc, name string
		want       fileInfo
	}{
		{
			"simple go file",
			"simple.go",
			fileInfo{
				ext:      ".go",
				category: goExt,
			},
		},
		{
			"simple go test",
			"foo_test.go",
			fileInfo{
				ext:      ".go",
				category: goExt,
				isTest:   true,
			},
		},
		{
			"test source",
			"test.go",
			fileInfo{
				ext:      ".go",
				category: goExt,
				isTest:   false,
			},
		},
		{
			"_test source",
			"_test.go",
			fileInfo{
				ext:      ".go",
				category: goExt,
				isTest:   true,
			},
		},
		{
			"source with goos",
			"foo_linux.go",
			fileInfo{
				ext:      ".go",
				category: goExt,
				goos:     "linux",
			},
		},
		{
			"source with goarch",
			"foo_amd64.go",
			fileInfo{
				ext:      ".go",
				category: goExt,
				goarch:   "amd64",
			},
		},
		{
			"source with goos then goarch",
			"foo_linux_amd64.go",
			fileInfo{
				ext:      ".go",
				category: goExt,
				goos:     "linux",
				goarch:   "amd64",
			},
		},
		{
			"source with goarch then goos",
			"foo_amd64_linux.go",
			fileInfo{
				ext:      ".go",
				category: goExt,
				goos:     "linux",
			},
		},
		{
			"test with goos and goarch",
			"foo_linux_amd64_test.go",
			fileInfo{
				ext:      ".go",
				category: goExt,
				goos:     "linux",
				goarch:   "amd64",
				isTest:   true,
			},
		},
		{
			"test then goos",
			"foo_test_linux.go",
			fileInfo{
				ext:      ".go",
				category: goExt,
				goos:     "linux",
			},
		},
		{
			"goos source",
			"linux.go",
			fileInfo{
				ext:      ".go",
				category: goExt,
				goos:     "",
			},
		},
		{
			"goarch source",
			"amd64.go",
			fileInfo{
				ext:      ".go",
				category: goExt,
				goarch:   "",
			},
		},
		{
			"goos test",
			"linux_test.go",
			fileInfo{
				ext:      ".go",
				category: goExt,
				goos:     "",
				isTest:   true,
			},
		},
		{
			"c file",
			"foo_test.cxx",
			fileInfo{
				ext:      ".cxx",
				category: cExt,
				isTest:   false,
			},
		},
		{
			"c os test file",
			"foo_linux_test.c",
			fileInfo{
				ext:      ".c",
				category: cExt,
				isTest:   false,
				goos:     "linux",
			},
		},
		{
			"h file",
			"foo_linux.h",
			fileInfo{
				ext:      ".h",
				category: hExt,
				goos:     "linux",
			},
		},
		{
			"go asm file",
			"foo_amd64.s",
			fileInfo{
				ext:      ".s",
				category: sExt,
				goarch:   "amd64",
			},
		},
		{
			"c asm file",
			"foo.S",
			fileInfo{
				ext:      ".S",
				category: csExt,
			},
		},
		{
			"unsupported file",
			"foo.m",
			fileInfo{
				ext:      ".m",
				category: unsupportedExt,
			},
		},
		{
			"ignored test file",
			"foo_test.py",
			fileInfo{
				ext:     ".py",
				isTest:  false,
				isXTest: false,
			},
		},
		{
			"ignored xtest file",
			"foo_xtest.py",
			fileInfo{
				ext:     ".py",
				isTest:  false,
				isXTest: false,
			},
		},
		{
			"ignored file",
			"foo.txt",
			fileInfo{
				ext:      ".txt",
				category: ignoredExt,
			},
		},
	} {
		tc.want.name = tc.name
		tc.want.dir = "dir"
		tc.want.path = filepath.Join("dir", tc.name)

		if got := fileNameInfo("dir", tc.name); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("case %q: got %#v; want %#v", tc.desc, got, tc.want)
		}
	}
}

func TestCgo(t *testing.T) {
	c := &config.Config{}
	dir := "."
	for _, tc := range []struct {
		desc, source string
		want         fileInfo
	}{
		{
			"not cgo",
			"package foo\n",
			fileInfo{isCgo: false},
		},
		{
			"empty cgo",
			`package foo

import "C"
`,
			fileInfo{isCgo: true},
		},
		{
			"simple flags",
			`package foo

/*
#cgo CFLAGS: -O0
	#cgo CPPFLAGS: -O1
#cgo   CXXFLAGS:   -O2
#cgo LDFLAGS: -O3 -O4
*/
import "C"
`,
			fileInfo{
				isCgo: true,
				copts: []taggedOpts{
					{opts: "-O0"},
					{opts: "-O1"},
					{opts: "-O2"},
				},
				clinkopts: []taggedOpts{
					{opts: "-O3 -O4"},
				},
			},
		},
		{
			"cflags with conditions",
			`package foo

/*
#cgo foo bar,!baz CFLAGS: -O0
*/
import "C"
`,
			fileInfo{
				isCgo: true,
				copts: []taggedOpts{
					{tags: "foo bar,!baz", opts: "-O0"},
				},
			},
		},
		{
			"slashslash comments",
			`package foo

// #cgo CFLAGS: -O0
// #cgo CFLAGS: -O1
import "C"
`,
			fileInfo{
				isCgo: true,
				copts: []taggedOpts{
					{opts: "-O0"},
					{opts: "-O1"},
				},
			},
		},
		{
			"comment above single import group",
			`package foo

/*
#cgo CFLAGS: -O0
*/
import ("C")
`,
			fileInfo{
				isCgo: true,
				copts: []taggedOpts{
					{opts: "-O0"},
				},
			},
		},
	} {
		path := "TestCgo.go"
		if err := ioutil.WriteFile(path, []byte(tc.source), 0600); err != nil {
			t.Fatal(err)
		}
		defer os.Remove(path)

		got, err := goFileInfo(c, dir, path)
		if err != nil {
			t.Fatal(err)
		}

		// Clear fields we don't care about for testing.
		got = fileInfo{isCgo: got.isCgo, copts: got.copts, clinkopts: got.clinkopts}

		if !reflect.DeepEqual(got, tc.want) {
			t.Errorf("case %q: got %#v; want %#v", tc.desc, got, tc.want)
		}
	}
}

func TestCgoFailures(t *testing.T) {
	c := &config.Config{}
	dir := "."
	for _, tc := range []struct {
		desc, source, wantError string
	}{
		{
			"bad go file",
			"pakcage foo",
			"expected 'package'",
		},
		{
			"unknown cgo verb",
			`package foo

// #cgo FFLAGS: -O0
import "C"
`,
			"invalid #cgo verb",
		},
		{
			"unsupported cgo verb",
			`package foo

// #cgo pkg-config: foo
import "C"
`,
			"not supported",
		},
		{
			"bad cgo quoting",
			`package foo

// #cgo CFLAGS: 'foo bar'
import "C"
`,
			"malformed #cgo argument",
		},
	} {
		path := "TestCgoFailures.go"
		if err := ioutil.WriteFile(path, []byte(tc.source), 0600); err != nil {
			t.Fatal(err)
		}
		defer os.Remove(path)

		var errorText string
		if _, err := goFileInfo(c, dir, path); err != nil {
			errorText = err.Error()
		}

		if tc.wantError == "" && errorText != "" || tc.wantError != "" && !strings.Contains(errorText, tc.wantError) {
			t.Errorf("case %q: got error %q; want error containing %q", tc.desc, errorText, tc.wantError)
		}
	}
}

// Copied from go/build build_test.go
var (
	expandSrcDirPath = filepath.Join(string(filepath.Separator)+"projects", "src", "add")
)

// Copied from go/build build_test.go
var expandSrcDirTests = []struct {
	input, expected string
}{
	{"-L ${SRCDIR}/libs -ladd", "-L /projects/src/add/libs -ladd"},
	{"${SRCDIR}/add_linux_386.a -pthread -lstdc++", "/projects/src/add/add_linux_386.a -pthread -lstdc++"},
	{"Nothing to expand here!", "Nothing to expand here!"},
	{"$", "$"},
	{"$$", "$$"},
	{"${", "${"},
	{"$}", "$}"},
	{"$FOO ${BAR}", "$FOO ${BAR}"},
	{"Find me the $SRCDIRECTORY.", "Find me the $SRCDIRECTORY."},
	{"$SRCDIR is missing braces", "$SRCDIR is missing braces"},
}

// Copied from go/build build_test.go
func TestExpandSrcDir(t *testing.T) {
	for _, test := range expandSrcDirTests {
		output, _ := expandSrcDir(test.input, expandSrcDirPath)
		if output != test.expected {
			t.Errorf("%q expands to %q with SRCDIR=%q when %q is expected", test.input, output, expandSrcDirPath, test.expected)
		} else {
			t.Logf("%q expands to %q with SRCDIR=%q", test.input, output, expandSrcDirPath)
		}
	}
}

// Copied from go/build build_test.go
func TestShellSafety(t *testing.T) {
	tests := []struct {
		input, srcdir, expected string
		result                  bool
	}{
		{"-I${SRCDIR}/../include", "/projects/src/issue 11868", "-I/projects/src/issue 11868/../include", true},
		{"-I${SRCDIR}", "wtf$@%", "-Iwtf$@%", true},
		{"-X${SRCDIR}/1,${SRCDIR}/2", "/projects/src/issue 11868", "-X/projects/src/issue 11868/1,/projects/src/issue 11868/2", true},
		{"-I/tmp -I/tmp", "/tmp2", "-I/tmp -I/tmp", false},
		{"-I/tmp", "/tmp/[0]", "-I/tmp", true},
		{"-I${SRCDIR}/dir", "/tmp/[0]", "-I/tmp/[0]/dir", false},
	}
	for _, test := range tests {
		output, ok := expandSrcDir(test.input, test.srcdir)
		if ok != test.result {
			t.Errorf("Expected %t while %q expands to %q with SRCDIR=%q; got %t", test.result, test.input, output, test.srcdir, ok)
		}
		if output != test.expected {
			t.Errorf("Expected %q while %q expands with SRCDIR=%q; got %q", test.expected, test.input, test.srcdir, output)
		}
	}
}

func TestIsStandard(t *testing.T) {
	for _, tc := range []struct {
		goPrefix, importpath string
		want                 bool
	}{
		{"", "fmt", true},
		{"", "encoding/json", true},
		{"", "foo/bar", true},
		{"", "foo.com/bar", false},
		{"foo", "fmt", true},
		{"foo", "encoding/json", true},
		{"foo", "foo", true},
		{"foo", "foo/bar", false},
		{"foo", "foo.com/bar", false},
		{"foo.com/bar", "fmt", true},
		{"foo.com/bar", "encoding/json", true},
		{"foo.com/bar", "foo/bar", true},
		{"foo.com/bar", "foo.com/bar", false},
	} {
		if got := isStandard(tc.goPrefix, tc.importpath); got != tc.want {
			t.Errorf("for prefix %q, importpath %q: got %#v; want %#v", tc.goPrefix, tc.importpath, got, tc.want)
		}
	}
}

func TestReadTags(t *testing.T) {
	for _, tc := range []struct {
		desc, source string
		want         []string
	}{
		{
			"empty file",
			"",
			nil,
		},
		{
			"single comment without blank line",
			"// +build foo\npackage main",
			nil,
		},
		{
			"multiple comments without blank link",
			`// +build foo

// +build bar
package main

`,
			[]string{"foo"},
		},
		{
			"single comment",
			"// +build foo\n\n",
			[]string{"foo"},
		},
		{
			"multiple comments",
			`// +build foo
// +build bar

package main`,
			[]string{"foo", "bar"},
		},
		{
			"multiple comments with blank",
			`// +build foo

// +build bar

package main`,
			[]string{"foo", "bar"},
		},
		{
			"comment with space",
			"  //   +build   foo   bar  \n\n",
			[]string{"foo bar"},
		},
		{
			"slash star comment",
			"/* +build foo */\n\n",
			nil,
		},
	} {
		f, err := ioutil.TempFile(".", "TestReadTags")
		if err != nil {
			t.Fatal(err)
		}
		path := f.Name()
		defer os.Remove(path)
		if err = f.Close(); err != nil {
			t.Fatal(err)
		}
		if err = ioutil.WriteFile(path, []byte(tc.source), 0600); err != nil {
			t.Fatal(err)
		}

		if got, err := readTags(path); err != nil {
			t.Fatal(err)
		} else if !reflect.DeepEqual(got, tc.want) {
			t.Errorf("case %q: got %#v; want %#v", tc.desc, got, tc.want)
		}
	}
}

func TestCheckConstraints(t *testing.T) {
	for _, tc := range []struct {
		desc string
		fi   fileInfo
		tags string
		want bool
	}{
		{
			"unconstrained",
			fileInfo{},
			"",
			true,
		},
		{
			"goos satisfied",
			fileInfo{goos: "linux"},
			"linux",
			true,
		},
		{
			"goos unsatisfied",
			fileInfo{goos: "linux"},
			"darwin",
			false,
		},
		{
			"goarch satisfied",
			fileInfo{goarch: "amd64"},
			"amd64",
			true,
		},
		{
			"goarch unsatisfied",
			fileInfo{goarch: "amd64"},
			"arm",
			false,
		},
		{
			"goos goarch satisfied",
			fileInfo{goos: "linux", goarch: "amd64"},
			"linux,amd64",
			true,
		},
		{
			"goos goarch unsatisfied",
			fileInfo{goos: "linux", goarch: "amd64"},
			"darwin,amd64",
			false,
		},
		{
			"tags all satisfied",
			fileInfo{tags: []string{"foo", "bar"}},
			"foo,bar",
			true,
		},
		{
			"tags some unsatisfied",
			fileInfo{tags: []string{"foo", "bar"}},
			"foo",
			false,
		},
		{
			"goos unsatisfied tags satisfied",
			fileInfo{goos: "linux", tags: []string{"foo"}},
			"darwin,foo",
			false,
		},
	} {
		if got := tc.fi.checkConstraints(parseTags(tc.tags)); got != tc.want {
			t.Errorf("case %q: got %#v; want %#v", tc.desc, got, tc.want)
		}
	}
}

func TestCheckTags(t *testing.T) {
	for _, tc := range []struct {
		desc, line, tags string
		want             bool
	}{
		{
			"empty tags",
			"",
			"",
			false,
		},
		{
			"ignored",
			"ignore",
			"",
			false,
		},
		{
			"single satisfied",
			"foo",
			"foo",
			true,
		},
		{
			"single unsatisfied",
			"foo",
			"bar",
			false,
		},
		{
			"NOT satisfied",
			"!foo",
			"",
			true,
		},
		{
			"NOT unsatisfied",
			"!foo",
			"foo",
			false,
		},
		{
			"double negative fails",
			"yes !!yes yes",
			"yes",
			false,
		},
		{
			"AND satisfied",
			"foo,bar",
			"foo,bar",
			true,
		},
		{
			"AND NOT satisfied",
			"foo,!bar",
			"foo",
			true,
		},
		{
			"AND unsatisfied",
			"foo,bar",
			"foo",
			false,
		},
		{
			"AND NOT unsatisfied",
			"foo,!bar",
			"foo,bar",
			false,
		},
		{
			"OR satisfied",
			"foo bar",
			"foo",
			true,
		},
		{
			"OR NOT satisfied",
			"foo !bar",
			"",
			true,
		},
		{
			"OR unsatisfied",
			"foo bar",
			"",
			false,
		},
		{
			"OR NOT unsatisfied",
			"foo !bar",
			"bar",
			false,
		},
		{
			"release tags",
			"go1.7,go1.8,go1.9,go1.91,go2.0",
			"",
			true,
		},
		{
			"release tag negated",
			"!go1.8",
			"",
			true,
		},
	} {
		if got := checkTags(tc.line, parseTags(tc.tags)); got != tc.want {
			t.Errorf("case %q: got %#v; want %#v", tc.desc, got, tc.want)
		}
	}
}

func parseTags(tags string) map[string]bool {
	tagMap := make(map[string]bool)
	for _, t := range strings.Split(tags, ",") {
		tagMap[t] = true
	}
	return tagMap
}
