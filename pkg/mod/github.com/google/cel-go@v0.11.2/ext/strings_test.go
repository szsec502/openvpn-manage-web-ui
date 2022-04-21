// Copyright 2020 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package ext

import (
	"fmt"
	"testing"

	"github.com/google/cel-go/cel"
)

// TODO: move these tests to a conformance test.
var stringTests = []struct {
	expr      string
	err       string
	parseOnly bool
}{
	// CharAt test.
	{expr: `'tacocat'.charAt(3) == 'o'`},
	{expr: `'tacocat'.charAt(7) == ''`},
	{expr: `'©αT'.charAt(0) == '©' && '©αT'.charAt(1) == 'α' && '©αT'.charAt(2) == 'T'`},
	// Index of search string tests.
	{expr: `'tacocat'.indexOf('') == 0`},
	{expr: `'tacocat'.indexOf('ac') == 1`},
	{expr: `'tacocat'.indexOf('none') == -1`},
	{expr: `'tacocat'.indexOf('', 3) == 3`},
	{expr: `'tacocat'.indexOf('a', 3) == 5`},
	{expr: `'tacocat'.indexOf('at', 3) == 5`},
	{expr: `'ta©o©αT'.indexOf('©') == 2`},
	{expr: `'ta©o©αT'.indexOf('©', 3) == 4`},
	{expr: `'ta©o©αT'.indexOf('©αT', 3) == 4`},
	{expr: `'ta©o©αT'.indexOf('©α', 5) == -1`},
	{expr: `'ijk'.indexOf('k') == 2`},
	{expr: `'hello wello'.indexOf('hello wello') == 0`},
	{expr: `'hello wello'.indexOf('ello', 6) == 7`},
	{expr: `'hello wello'.indexOf('elbo room!!') == -1`},
	{expr: `'hello wello'.indexOf('elbo room!!!') == -1`},
	{expr: `'tacocat'.lastIndexOf('') == 7`},
	{expr: `'tacocat'.lastIndexOf('at') == 5`},
	{expr: `'tacocat'.lastIndexOf('none') == -1`},
	{expr: `'tacocat'.lastIndexOf('', 3) == 3`},
	{expr: `'tacocat'.lastIndexOf('a', 3) == 1`},
	{expr: `'ta©o©αT'.lastIndexOf('©') == 4`},
	{expr: `'ta©o©αT'.lastIndexOf('©', 3) == 2`},
	{expr: `'ta©o©αT'.lastIndexOf('©α', 4) == 4`},
	{expr: `'hello wello'.lastIndexOf('ello', 6) == 1`},
	{expr: `'hello wello'.lastIndexOf('low') == -1`},
	{expr: `'hello wello'.lastIndexOf('elbo room!!') == -1`},
	{expr: `'hello wello'.lastIndexOf('elbo room!!!') == -1`},
	{expr: `'hello wello'.lastIndexOf('hello wello') == 0`},
	{expr: `'bananananana'.lastIndexOf('nana', 7) == 6`},
	// Lower ASCII tests.
	{expr: `'TacoCat'.lowerAscii() == 'tacocat'`},
	{expr: `'TacoCÆt'.lowerAscii() == 'tacocÆt'`},
	{expr: `'TacoCÆt Xii'.lowerAscii() == 'tacocÆt xii'`},
	// Replace tests
	{expr: `"12 days 12 hours".replace("{0}", "2") == "12 days 12 hours"`},
	{expr: `"{0} days {0} hours".replace("{0}", "2") == "2 days 2 hours"`},
	{expr: `"{0} days {0} hours".replace("{0}", "2", 1).replace("{0}", "23") == "2 days 23 hours"`},
	{expr: `"1 ©αT taco".replace("αT", "o©α") == "1 ©o©α taco"`},
	// Split tests.
	{expr: `"hello world".split(" ") == ["hello", "world"]`},
	{expr: `"hello world events!".split(" ", 0) == []`},
	{expr: `"hello world events!".split(" ", 1) == ["hello world events!"]`},
	{expr: `"o©o©o©o".split("©", -1) == ["o", "o", "o", "o"]`},
	// Substring tests.
	{expr: `"tacocat".substring(4) == "cat"`},
	{expr: `"tacocat".substring(7) == ""`},
	{expr: `"tacocat".substring(0, 4) == "taco"`},
	{expr: `"tacocat".substring(4, 4) == ""`},
	{expr: `'ta©o©αT'.substring(2, 6) == "©o©α"`},
	{expr: `'ta©o©αT'.substring(7, 7) == ""`},
	// Trim tests using the unicode standard for whitespace.
	{expr: `" \f\n\r\t\vtext  ".trim() == "text"`},
	{expr: `"\u0085\u00a0\u1680text".trim() == "text"`},
	{expr: `"text\u2000\u2001\u2002\u2003\u2004\u2004\u2006\u2007\u2008\u2009".trim() == "text"`},
	{expr: `"\u200atext\u2028\u2029\u202F\u205F\u3000".trim() == "text"`},
	// Trim test with whitespace-like characters not included.
	{expr: `"\u180etext\u200b\u200c\u200d\u2060\ufeff".trim()
				== "\u180etext\u200b\u200c\u200d\u2060\ufeff"`},
	// Upper ASCII tests.
	{expr: `'tacoCat'.upperAscii() == 'TACOCAT'`},
	{expr: `'tacoCαt'.upperAscii() == 'TACOCαT'`},
	// Join tests.
	{expr: `['x', 'y'].join() == 'xy'`},
	{expr: `['x', 'y'].join('-') == 'x-y'`},
	{expr: `[].join() == ''`},
	{expr: `[].join('-') == ''`},
	// Error test cases based on checked expression usage.
	{
		expr: `'tacocat'.charAt(30) == ''`,
		err:  "index out of range: 30",
	},
	{
		expr: `'tacocat'.indexOf('a', 30) == -1`,
		err:  "index out of range: 30",
	},
	{
		expr: `'tacocat'.lastIndexOf('a', -1) == -1`,
		err:  "index out of range: -1",
	},
	{
		expr: `'tacocat'.lastIndexOf('a', 30) == -1`,
		err:  "index out of range: 30",
	},
	{
		expr: `"tacocat".substring(40) == "cat"`,
		err:  "index out of range: 40",
	},
	{
		expr: `"tacocat".substring(-1) == "cat"`,
		err:  "index out of range: -1",
	},
	{
		expr: `"tacocat".substring(1, 50) == "cat"`,
		err:  "index out of range: 50",
	},
	{
		expr: `"tacocat".substring(49, 50) == "cat"`,
		err:  "index out of range: 49",
	},
	{
		expr: `"tacocat".substring(4, 3) == ""`,
		err:  "invalid substring range. start: 4, end: 3",
	},
	// Valid parse-only expressions which should generate runtime errors.
	{
		expr:      `42.charAt(2) == ""`,
		err:       "no such overload",
		parseOnly: true,
	},
	{
		expr:      `'hello'.charAt(true) == ""`,
		err:       "no such overload",
		parseOnly: true,
	},
	{
		expr:      `24.indexOf('2') == 0`,
		err:       "no such overload",
		parseOnly: true,
	},
	{
		expr:      `'hello'.indexOf(true) == 1`,
		err:       "no such overload",
		parseOnly: true,
	},
	{
		expr:      `42.indexOf('4', 0) == 0`,
		err:       "no such overload",
		parseOnly: true,
	},
	{
		expr:      `'42'.indexOf(4, 0) == 0`,
		err:       "no such overload",
		parseOnly: true,
	},
	{
		expr:      `'42'.indexOf('4', '0') == 0`,
		err:       "no such overload",
		parseOnly: true,
	},
	{
		expr:      `'42'.indexOf('4', 0, 1) == 0`,
		err:       "no such overload",
		parseOnly: true,
	},
	{
		expr:      `42.split("2") == ["4"]`,
		err:       "no such overload",
		parseOnly: true,
	},
	{
		expr:      `42.replace(2, 1) == "41"`,
		err:       "no such overload",
		parseOnly: true,
	},
	{
		expr:      `"42".replace(2, 1) == "41"`,
		err:       "no such overload",
		parseOnly: true,
	},
	{
		expr:      `"42".replace("2", 1) == "41"`,
		err:       "no such overload",
		parseOnly: true,
	},
	{
		expr:      `42.replace("2", "1", 1) == "41"`,
		err:       "no such overload",
		parseOnly: true,
	},
	{
		expr:      `"42".replace(2, "1", 1) == "41"`,
		err:       "no such overload",
		parseOnly: true,
	},
	{
		expr:      `"42".replace("2", 1, 1) == "41"`,
		err:       "no such overload",
		parseOnly: true,
	},
	{
		expr:      `"42".replace("2", "1", "1") == "41"`,
		err:       "no such overload",
		parseOnly: true,
	},
	{
		expr:      `"42".replace("2", "1", 1, false) == "41"`,
		err:       "no such overload",
		parseOnly: true,
	},
	{
		expr:      `42.split("") == ["4", "2"]`,
		err:       "no such overload",
		parseOnly: true,
	},
	{
		expr:      `"42".split(2) == ["4"]`,
		err:       "no such overload",
		parseOnly: true,
	},
	{
		expr:      `42.split("2", "1") == ["4"]`,
		err:       "no such overload",
		parseOnly: true,
	},
	{
		expr:      `"42".split(2, 1) == ["4"]`,
		err:       "no such overload",
		parseOnly: true,
	},
	{
		expr:      `"42".split("2", "1") == ["4"]`,
		err:       "no such overload",
		parseOnly: true,
	},
	{
		expr:      `"42".split("2", 1, 1) == ["4"]`,
		err:       "no such overload",
		parseOnly: true,
	},
	{
		expr:      `'hello'.substring(1, 2, 3) == ""`,
		err:       "no such overload",
		parseOnly: true,
	},
	{
		expr:      `30.substring(true, 3) == ""`,
		err:       "no such overload",
		parseOnly: true,
	},
	{
		expr:      `"tacocat".substring(true, 3) == ""`,
		err:       "no such overload",
		parseOnly: true,
	},
	{
		expr:      `"tacocat".substring(0, false) == ""`,
		err:       "no such overload",
		parseOnly: true,
	},
}

func TestStrings(t *testing.T) {
	env, err := cel.NewEnv(Strings())
	if err != nil {
		t.Fatal(err)
	}
	for i, tst := range stringTests {
		tc := tst
		t.Run(fmt.Sprintf("[%d]", i), func(tt *testing.T) {
			var asts []*cel.Ast
			pAst, iss := env.Parse(tc.expr)
			if iss.Err() != nil {
				tt.Fatal(iss.Err())
			}
			asts = append(asts, pAst)
			if !tc.parseOnly {
				cAst, iss := env.Check(pAst)
				if iss.Err() != nil {
					tt.Fatal(iss.Err())
				}
				asts = append(asts, cAst)
			}
			for _, ast := range asts {
				prg, err := env.Program(ast)
				if err != nil {
					tt.Fatal(err)
				}
				out, _, err := prg.Eval(cel.NoVars())
				if tc.err != "" {
					if err == nil {
						tt.Fatalf("got value %v, wanted error %s for expr: %s",
							out.Value(), tc.err, tc.expr)
					}
					if tc.err != err.Error() {
						tt.Errorf("got error %v, wanted error %s for expr: %s", err, tc.err, tc.expr)
					}
				} else if err != nil {
					tt.Fatal(err)
				} else if out.Value() != true {
					tt.Errorf("got %v, wanted true for expr: %s", out.Value(), tc.expr)
				}
			}
		})
	}
}
