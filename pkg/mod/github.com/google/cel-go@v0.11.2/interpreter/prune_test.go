// Copyright 2018 Google LLC
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

package interpreter

import (
	"testing"

	"github.com/google/cel-go/common"
	"github.com/google/cel-go/common/containers"
	"github.com/google/cel-go/parser"
	"github.com/google/cel-go/test"
)

type testInfo struct {
	in   interface{}
	expr string
	out  string
}

var testCases = []testInfo{
	{
		in: map[string]interface{}{
			"msg": map[string]string{"foo": "bar"},
		},
		expr: `msg`,
		out:  `{"foo": "bar"}`,
	},
	{
		expr: `true && false`,
		out:  `false`,
	},
	{
		in:   unknownActivation("x"),
		expr: `(true || false) && x`,
		out:  `x`,
	},
	{
		in:   unknownActivation("x"),
		expr: `(false || false) && x`,
		out:  `false`,
	},
	{
		in:   unknownActivation("a"),
		expr: `a && [1, 1u, 1.0].exists(x, type(x) == uint)`,
		out:  `a`,
	},
	{
		expr: `{'hello': 'world'.size()}`,
		out:  `{"hello": 5}`,
	},
	{
		expr: `[b'bytes-string']`,
		out:  `[b"\142\171\164\145\163\055\163\164\162\151\156\147"]`,
	},
	{
		expr: `[b'bytes'] + [b'-' + b'string']`,
		out:  `[b"\142\171\164\145\163", b"\055\163\164\162\151\156\147"]`,
	},
	{
		expr: `1u + 3u`,
		out:  `4u`,
	},
	{
		expr: `2 < 3`,
		out:  `true`,
	},
	{
		in:   unknownActivation(),
		expr: `test == null`,
		out:  `test == null`,
	},
	{
		in:   unknownActivation(),
		expr: `test == null || true`,
		out:  `true`,
	},
	{
		in:   unknownActivation(),
		expr: `test == null && false`,
		out:  `false`,
	},
	{
		in:   unknownActivation("b", "c"),
		expr: `true ? b < 1.2 : c == ['hello']`,
		out:  `b < 1.2`,
	},
	{
		in:   unknownActivation(),
		expr: `[1+3, 2+2, 3+1, four]`,
		out:  `[4, 4, 4, four]`,
	},
	{
		in:   unknownActivation(),
		expr: `test == {'a': 1, 'field': 2}.field`,
		out:  `test == 2`,
	},
	{
		in:   unknownActivation(),
		expr: `test in {'a': 1, 'field': [2, 3]}.field`,
		out:  `test in [2, 3]`,
	},
	{
		in:   unknownActivation(),
		expr: `test == {'field': [1 + 2, 2 + 3]}`,
		out:  `test == {"field": [3, 5]}`,
	},
	{
		in:   unknownActivation(),
		expr: `test in {'a': 1, 'field': [test, 3]}.field`,
		out:  `test in {"a": 1, "field": [test, 3]}.field`,
	},
	// TODO(issues/) the output test relies on tracking macro expansions back to their original
	// call patterns.
	/* {
		in:   unknownActivation(),
		expr: `[1+3, 2+2, 3+1, four].exists(x, x == four)`,
		out:  `[4, 4, 4, four].exists(x, x == four)`,
	}, */
}

func TestPrune(t *testing.T) {
	for i, tst := range testCases {
		ast, iss := parser.Parse(common.NewStringSource(tst.expr, "<input>"))
		if len(iss.GetErrors()) > 0 {
			t.Fatalf(iss.ToDisplayString())
		}
		state := NewEvalState()
		reg := newTestRegistry(t)
		attrs := NewPartialAttributeFactory(containers.DefaultContainer, reg, reg)
		interp := NewStandardInterpreter(containers.DefaultContainer, reg, reg, attrs)

		interpretable, _ := interp.NewUncheckedInterpretable(
			ast.Expr,
			ExhaustiveEval(), Observe(EvalStateObserver(state)))
		interpretable.Eval(testActivation(t, tst.in))
		newExpr := PruneAst(ast.Expr, state)
		actual, err := parser.Unparse(newExpr, nil)
		if err != nil {
			t.Error(err)
		}
		if !test.Compare(actual, tst.out) {
			t.Errorf("prune[%d], diff: %s", i, test.DiffMessage("structure", actual, tst.out))
		}
	}
}

func unknownActivation(vars ...string) PartialActivation {
	pats := make([]*AttributePattern, len(vars), len(vars))
	for i, v := range vars {
		pats[i] = NewAttributePattern(v)
	}
	a, _ := NewPartialActivation(map[string]interface{}{}, pats...)
	return a
}

func testActivation(t *testing.T, in interface{}) Activation {
	t.Helper()
	if in == nil {
		return EmptyActivation()
	}
	a, err := NewActivation(in)
	if err != nil {
		t.Fatalf("NewActivation(%v) failed: %v", in, err)
	}
	return a
}
