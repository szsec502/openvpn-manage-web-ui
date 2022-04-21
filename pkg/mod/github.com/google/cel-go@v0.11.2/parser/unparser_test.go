// Copyright 2019 Google LLC
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

package parser

import (
	"errors"
	"strings"
	"testing"

	"github.com/google/cel-go/common"
	"google.golang.org/protobuf/proto"

	exprpb "google.golang.org/genproto/googleapis/api/expr/v1alpha1"
)

func TestUnparse(t *testing.T) {
	tests := []struct {
		name               string
		in                 string
		out                interface{}
		requiresMacroCalls bool
	}{
		{name: "call_add", in: `a + b - c`},
		{name: "call_and", in: `a && b && c && d && e`},
		{name: "call_and_or", in: `a || b && (c || d) && e`},
		{name: "call_cond", in: `a ? b : c`},
		{name: "call_cond_nested_inner", in: `a ? (c ? d : e) : b`},
		{name: "call_cond_nested_outer", in: `a ? b : c ? d : e`, out: `a ? b : (c ? d : e)`},
		{name: "call_index", in: `a[1]["b"]`},
		{name: "call_index_eq", in: `x["a"].single_int32 == 23`},
		{name: "call_mul", in: `a * (b / c) % 0`},
		{name: "call_mul_add", in: `a + b * c`},
		{name: "call_mul_add_nested", in: `(a + b) * c / (d - e)`},
		{name: "call_mul_nested", in: `a * b / c % 0`},
		{name: "call_not", in: `!true`},
		{name: "call_neg", in: `-num`},
		{name: "call_or", in: `a || b || c || d || e`},
		{name: "call_neg_mult", in: `-(1 * 2)`},
		{name: "call_neg_add", in: `-(1 + 2)`},
		{name: "call_operator_precedence", in: `1 - (2 == -1)`},
		{name: "calc_distr_paren", in: `(1 + 2) * 3`},
		{name: "calc_distr_noparen", in: `1 + 2 * 3`},
		{name: "cond_tern_simple", in: `(x > 5) ? (x - 5) : 0`},
		{name: "cond_tern_neg_expr", in: `-((x > 5) ? (x - 5) : 0)`},
		{name: "cond_tern_neg_term", in: `-x ? (x - 5) : 0`},
		{name: "func_global", in: `size(a ? (b ? c : d) : e)`},
		{name: "func_member", in: `a.hello("world")`},
		{name: "func_no_arg", in: `zero()`},
		{name: "func_one_arg", in: `one("a")`},
		{name: "func_two_args", in: `and(d, 32u)`},
		{name: "func_var_args", in: `max(a, b, 100)`},
		{name: "func_neq", in: `x != "a"`},
		{name: "func_in", in: `a in b`},
		{name: "list_empty", in: `[]`},
		{name: "list_one", in: `[1]`},
		{name: "list_many", in: `["hello, world", "goodbye, world", "sure, why not?"]`},
		{name: "lit_bytes", in: `b"\303\203\302\277"`},
		{name: "lit_double", in: `-42.101`},
		{name: "lit_false", in: `false`},
		{name: "lit_int", in: `-405069`},
		{name: "lit_null", in: `null`},
		{name: "lit_string", in: `"hello:\t'world'"`},
		{name: "lit_string_quote", in: `"hello:\"world\""`},
		{name: "lit_true", in: `true`},
		{name: "lit_uint", in: `42u`},
		{name: "ident", in: `my_ident`},
		{name: "macro_has", in: `has(hello.world)`},
		{name: "map_empty", in: `{}`},
		{name: "map_lit_key", in: `{"a": a.b.c, b"\142": bytes(a.b.c)}`},
		{name: "map_expr_key", in: `{a: a, b: a.b, c: a.b.c, a ? b : c: false, a || b: true}`},
		{name: "msg_empty", in: `v1alpha1.Expr{}`},
		{name: "msg_fields", in: `v1alpha1.Expr{id: 1, call_expr: v1alpha1.Call_Expr{function: "name"}}`},
		{name: "select", in: `a.b.c`},
		{name: "idx_idx_sel", in: `a[b][c].name`},
		{name: "sel_expr_target", in: `(a + b).name`},
		{name: "sel_cond_target", in: `(a ? b : c).name`},
		{name: "idx_cond_target", in: `(a ? b : c)[0]`},
		{name: "cond_conj", in: `(a1 && a2) ? b : c`},
		{name: "cond_disj_conj", in: `a ? (b1 || b2) : (c1 && c2)`},
		{name: "call_cond_target", in: `(a ? b : c).method(d)`},
		{name: "cond_flat", in: `false && !true || false`},
		{name: "cond_paren", in: `false && (!true || false)`},
		{name: "cond_cond", in: `(false && !true || false) ? 2 : 3`},
		{name: "cond_binop", in: `(x < 5) ? x : 5`},
		{name: "cond_binop_binop", in: `(x > 5) ? (x - 5) : 0`},
		{name: "cond_cond_binop", in: `(x > 5) ? ((x > 10) ? (x - 10) : 5) : 0`},

		// Equivalent expressions form unparse which do not match the originals.
		{name: "call_add_equiv", in: `a+b-c`, out: `a + b - c`},
		{name: "call_cond_equiv", in: `a ? b          : c`, out: `a ? b : c`},
		{name: "call_index_equiv", in: `a[  1  ]["b"]`, out: `a[1]["b"]`},
		{name: "call_or_and_equiv", in: `(false && !true) || false`, out: `false && !true || false`},
		{name: "call_not_not_equiv", in: `!!true`, out: `true`},
		{name: "call_cond_equiv", in: `(a || b ? c : d).e`, out: `((a || b) ? c : d).e`},
		{name: "lit_quote_bytes_equiv", in: `b'aaa"bbb'`, out: `b"\141\141\141\042\142\142\142"`},
		{name: "select_equiv", in: `a . b . c`, out: `a.b.c`},

		// These expressions require macro call tracking to be enabled.
		{
			name:               "comp_all",
			in:                 `[1, 2, 3].all(x, x > 0)`,
			requiresMacroCalls: true,
		},
		{
			name:               "comp_exists",
			in:                 `[1, 2, 3].exists(x, x > 0)`,
			requiresMacroCalls: true,
		},
		{
			name:               "comp_map",
			in:                 `[1, 2, 3].map(x, x >= 2, x * 4)`,
			requiresMacroCalls: true,
		},
		{
			name:               "comp_exists_one",
			in:                 `[1, 2, 3].exists_one(x, x >= 2)`,
			requiresMacroCalls: true,
		},
		{
			name:               "comp_nested",
			in:                 `[[1], [2], [3]].map(x, x.filter(y, y > 1))`,
			requiresMacroCalls: true,
		},
		{
			name:               "comp_chained",
			in:                 `[1, 2, 3].map(x, x >= 2, x * 4).filter(x, x <= 10)`,
			requiresMacroCalls: true,
		},
	}

	for _, tst := range tests {
		tc := tst
		t.Run(tc.name, func(t *testing.T) {
			prsr, err := NewParser(
				Macros(AllMacros...),
				PopulateMacroCalls(tc.requiresMacroCalls),
			)
			if err != nil {
				t.Fatalf("NewParser() failed: %v", err)
			}
			p, iss := prsr.Parse(common.NewTextSource(tc.in))
			if len(iss.GetErrors()) > 0 {
				t.Fatalf("parser.Parse(%s) failed: %v", tc.in, iss.ToDisplayString())
			}
			out, err := Unparse(p.GetExpr(), p.GetSourceInfo())
			if err != nil {
				t.Fatalf("Unparse(%s) failed: %v", tc.in, err)
			}
			var want interface{} = tc.in
			if tc.out != nil {
				want = tc.out
			}
			if out != want {
				t.Errorf("Unparse() got '%s', wanted '%s'", out, want)
			}
			p2, iss := prsr.Parse(common.NewTextSource(out))
			if len(iss.GetErrors()) > 0 {
				t.Fatalf("parser.Parse(%s) roundtrip failed: %v", tc.in, iss.ToDisplayString())
			}
			before := p.GetExpr()
			after := p2.GetExpr()
			if !proto.Equal(before, after) {
				t.Errorf("Roundtrip Parse() differs from original. Got '%v', wanted '%v'", before, after)
			}
		})
	}
}

func TestUnparseErrors(t *testing.T) {
	tests := []struct {
		name string
		in   *exprpb.Expr
		err  error
	}{
		{name: "empty_expr", in: &exprpb.Expr{}, err: errors.New("unsupported expression")},
		{
			name: "bad_constant",
			in: &exprpb.Expr{
				ExprKind: &exprpb.Expr_ConstExpr{
					ConstExpr: &exprpb.Constant{},
				},
			},
			err: errors.New("unsupported constant"),
		},
		{
			name: "bad_args",
			in: &exprpb.Expr{
				ExprKind: &exprpb.Expr_CallExpr{
					CallExpr: &exprpb.Expr_Call{
						Function: "_&&_",
						Args:     []*exprpb.Expr{{}, {}},
					},
				},
			},
			err: errors.New("unsupported expression"),
		},
		{
			name: "bad_struct",
			in: &exprpb.Expr{
				ExprKind: &exprpb.Expr_StructExpr{
					StructExpr: &exprpb.Expr_CreateStruct{
						MessageName: "Msg",
						Entries: []*exprpb.Expr_CreateStruct_Entry{
							{KeyKind: &exprpb.Expr_CreateStruct_Entry_FieldKey{FieldKey: "field"}},
						},
					},
				},
			},
			err: errors.New("unsupported expression"),
		},
		{
			name: "bad_map",
			in: &exprpb.Expr{
				ExprKind: &exprpb.Expr_StructExpr{
					StructExpr: &exprpb.Expr_CreateStruct{
						Entries: []*exprpb.Expr_CreateStruct_Entry{
							{KeyKind: &exprpb.Expr_CreateStruct_Entry_FieldKey{FieldKey: "field"}},
						},
					},
				},
			},
			err: errors.New("unsupported expression"),
		},
		{
			name: "bad_index",
			in: &exprpb.Expr{
				ExprKind: &exprpb.Expr_CallExpr{
					CallExpr: &exprpb.Expr_Call{
						Function: "_[_]",
						Args:     []*exprpb.Expr{{}, {}},
					},
				},
			},
			err: errors.New("unsupported expression"),
		},
	}

	for _, tst := range tests {
		tc := tst
		t.Run(tc.name, func(t *testing.T) {
			out, err := Unparse(tc.in, &exprpb.SourceInfo{})
			if err == nil {
				t.Fatalf("Unparse(%v) got %v, wanted error %v", tc.in, out, tc.err)
			}
			if !strings.Contains(err.Error(), tc.err.Error()) {
				t.Errorf("Unparse(%v) got unexpected error: %v, wanted %v", tc.in, err, tc.err)
			}
		})
	}
}
