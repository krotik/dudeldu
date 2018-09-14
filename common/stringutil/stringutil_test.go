/*
 * Public Domain Software
 *
 * I (Matthias Ladkau) am the author of the source code in this file.
 * I have placed the source code in this file in the public domain.
 *
 * For further information see: http://creativecommons.org/publicdomain/zero/1.0/
 */

package stringutil

import (
	"fmt"
	"regexp"
	"testing"
)

func TestPluralCompareByteArray(t *testing.T) {
	if fmt.Sprintf("There are 2 test%s", Plural(2)) != "There are 2 tests" {
		t.Error("2 items should have an 's'")
	}
	if fmt.Sprintf("There is 1 test%s", Plural(1)) != "There is 1 test" {
		t.Error("1 item should have no 's'")
	}
}
func TestStripCStyleComments(t *testing.T) {

	test := `
// Comment1
This is a test
/* A
comment
// Comment2
  */ bla
`

	if out := string(StripCStyleComments([]byte(test))); out != `
This is a test
 bla
` {
		t.Error("Unexpected return:", out)
		return
	}
}

func TestGlobToRegex(t *testing.T) {
	globMatch(t, true, "*", "^$", "foo", "bar")
	globMatch(t, true, "?", "?", "^", "[", "]", "$")
	globMatch(t, true, "foo*", "foo", "food", "fool")
	globMatch(t, true, "f*d", "fud", "food")
	globMatch(t, true, "*d", "good", "bad")
	globMatch(t, true, "\\*\\?\\[\\{\\\\", "*?[{\\")
	globMatch(t, true, "[]^-]", "]", "-", "^")
	globMatch(t, true, "]", "]")
	globMatch(t, true, "^.$()|+", "^.$()|+")
	globMatch(t, true, "[^^]", ".", "$", "[", "]")
	globMatch(t, false, "[^^]", "^")
	globMatch(t, true, "[!!-]", "^", "?")
	globMatch(t, false, "[!!-]", "!", "-")
	globMatch(t, true, "{[12]*,[45]*,[78]*}", "1", "2!", "4", "42", "7", "7$")
	globMatch(t, false, "{[12]*,[45]*,[78]*}", "3", "6", "9ÃŸ")
	globMatch(t, true, "}", "}")
	globMatch(t, true, "abc,", "abc,")

	globMatch(t, true, "myfile[^9]", "myfile1")
	globMatch(t, true, "myfile[!9]", "myfile1")
	globMatch(t, false, "myfile[^9]", "myfile9")
	globMatch(t, false, "myfile[!9]", "myfile9")

	globMatch(t, true, "*.*", "tester/bla.txt")
	globMatch(t, false, "*.tmp", "tester/bla.txt")

	testdata := []string{"foo*test", "f?t", "*d", "all"}
	expected := []string{"foo", "f", "", "all"}

	for i, str := range testdata {
		res := GlobStartingLiterals(str)

		if res != expected[i] {
			t.Error("Unexpected starting literal for glob:", res, "str:",
				str, "expected:", expected[i])
		}
	}

	testdata = []string{"[", "{", "\\", "*.*\\", "[["}
	expected = []string{"Unclosed character class at 1 of [",
		"Unclosed group at 1 of {",
		"Missing escaped character at 1 of \\",
		"Missing escaped character at 4 of *.*\\",
		"Unclosed character class at 1 of [["}

	for i, str := range testdata {
		_, err := GlobToRegex(str)

		if err.Error() != expected[i] {
			t.Error("Unexpected error for glob:", err, "str:",
				str, "expected error:", expected[i])
		}
	}

	str, err := GlobToRegex("[][]")

	if err != nil {
		t.Error("Unecpected glob parsing error:", err)
	}
	if str != "[][]" {
		t.Error("Unexpected glob parsing result:", str)
	}
}

func globMatch(t *testing.T, expectedResult bool, glob string, testStrings ...string) {
	re, err := GlobToRegex(glob)
	if err != nil {
		t.Error("Glob parsing error:", err)
	}
	for _, testString := range testStrings {
		res, err := regexp.MatchString(re, testString)
		if err != nil {
			t.Error("Regexp", re, "parsing error:", err, "from glob", glob)
		}
		if res != expectedResult {
			t.Error("Unexpected evaluation result. Glob:", glob, "testString:",
				testString, "expectedResult:", expectedResult)
		}
	}
}

func TestLevenshteinDistance(t *testing.T) {
	testdata1 := []string{"", "a", "", "abc", "", "a", "abc", "a", "b", "ac",
		"abcdefg", "a", "ab", "example", "sturgeon", "levenshtein", "distance"}
	testdata2 := []string{"", "", "a", "", "abc", "a", "abc", "ab", "ab", "abc",
		"xabxcdxxefxgx", "b", "ac", "samples", "urgently", "frankenstein", "difference"}
	expected := []int{0, 1, 1, 3, 3, 0, 0, 1, 1, 1, 6, 1, 1,
		3, 6, 6, 5}

	for i, str1 := range testdata1 {
		res := LevenshteinDistance(str1, testdata2[i])

		if res != expected[i] {
			t.Error("Unexpected Levenshtein distance result:", res, "str1:",
				str1, "str2:", testdata2[i], "expected:", expected[i])
		}
	}
}

func TestVersionStringCompare(t *testing.T) {
	testdata1 := []string{"1", "1.1", "1.1", "2.1", "5.4.3.2.1", "1.674.2.18",
		"1.674.2", "1.674.2.5", "2.4.18.14smp", "2.4.18.15smp", "1.2.3a1",
		"2.18.15smp"}
	testdata2 := []string{"2", "2.0", "1.1", "2.0", "6.5.4.3.2", "1.674.2.5",
		"1.674.2.5", "1.674.2", "2.4.18.14smp", "2.4.18.14smp", "1.2.3b1",
		"2.4.18.14smp"}

	expected := []int{-1, -1, 0, 1, -1, 1, -1, 1, 0, 1, -1, 1}

	for i, str1 := range testdata1 {
		res := VersionStringCompare(str1, testdata2[i])

		if res != expected[i] {
			t.Error("Unexpected version string compare result:", res, "str1:",
				str1, "str2:", testdata2[i])
		}
	}
}

func TestVersionStringPartCompare(t *testing.T) {

	testdata1 := []string{"", "", "1", "1", "a", "1a", "a", "1a", "1a", "1", "12a", "12a1",
		"12a1"}
	testdata2 := []string{"", "1", "", "2", "b", "b", "2b", "2b", "1", "1b", "12b", "12a2",
		"12b1"}
	expected := []int{0, -1, 1, -1, -1, 1, -1, -1, 1, -1, -1, -1, -1}

	for i, str1 := range testdata1 {
		res := versionStringPartCompare(str1, testdata2[i])

		if res != expected[i] {
			t.Error("Unexpected version string compare result:", res, "str1:",
				str1, "str2:", testdata2[i])
		}
	}
}

func TestIsAlphaNumeric(t *testing.T) {
	testdata := []string{"test", "123test", "test1234_123", "test#", "test-"}
	expected := []bool{true, true, true, false, false}

	for i, str := range testdata {
		if IsAlphaNumeric(str) != expected[i] {
			t.Error("Unexpected result for alphanumeric test:", str)
		}
	}
}

func TestCreateDisplayString(t *testing.T) {
	testdata := []string{"this is a tEST", "_bla", "a_bla", "a__bla", "a__b_la", ""}
	expected := []string{"This Is A Test", " Bla", "A Bla", "A  Bla", "A  B La", ""}

	for i, str := range testdata {
		res := CreateDisplayString(str)
		if res != expected[i] {
			t.Error("Unexpected result for creating a display string from:", str,
				"result:", res, "expected:", expected[i])
		}
	}
}

func TestGenerateRollingString(t *testing.T) {
	testdata := []string{"_-=-_", "abc", "=", ""}
	testlen := []int{20, 4, 5, 100}
	expected := []string{"_-=-__-=-__-=-__-=-_", "abca", "=====", ""}

	for i, str := range testdata {
		res := GenerateRollingString(str, testlen[i])
		if res != expected[i] {
			t.Error("Unexpected result for creating a roling string from:", str,
				"result:", res, "expected:", expected[i])
		}
	}
}

func TestMD5HexString(t *testing.T) {
	res := MD5HexString("This is a test")
	if res != "ce114e4501d2f4e2dcea3e17b546f339" {
		t.Error("Unexpected md5 hex result", res)

	}
}
