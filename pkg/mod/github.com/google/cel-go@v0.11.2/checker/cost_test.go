// Copyright 2022 Google LLC
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

package checker

import (
	"strings"
	"testing"

	"github.com/google/cel-go/checker/decls"
	"github.com/google/cel-go/common"
	"github.com/google/cel-go/common/containers"
	"github.com/google/cel-go/common/overloads"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/parser"

	proto3pb "github.com/google/cel-go/test/proto3pb"

	exprpb "google.golang.org/genproto/googleapis/api/expr/v1alpha1"
)

func TestCost(t *testing.T) {
	allTypes := decls.NewObjectType("google.expr.proto3.test.TestAllTypes")
	allList := decls.NewListType(allTypes)
	intList := decls.NewListType(decls.Int)
	nestedList := decls.NewListType(allList)

	allMap := decls.NewMapType(decls.String, allTypes)
	nestedMap := decls.NewMapType(decls.String, allMap)

	zeroCost := CostEstimate{}
	oneCost := CostEstimate{Min: 1, Max: 1}
	cases := []struct {
		name   string
		expr   string
		decls  []*exprpb.Decl
		hints  map[string]int64
		wanted CostEstimate
	}{
		{
			name:   "const",
			expr:   `"Hello World!"`,
			wanted: zeroCost,
		},
		{
			name:   "identity",
			expr:   `input`,
			decls:  []*exprpb.Decl{decls.NewVar("input", intList)},
			wanted: CostEstimate{Min: 1, Max: 1},
		},
		{
			name:   "select: map",
			expr:   `input['key']`,
			decls:  []*exprpb.Decl{decls.NewVar("input", decls.NewMapType(decls.String, decls.String))},
			wanted: CostEstimate{Min: 2, Max: 2},
		},
		{
			name:   "select: field",
			expr:   `input.single_int32`,
			decls:  []*exprpb.Decl{decls.NewVar("input", allTypes)},
			wanted: CostEstimate{Min: 2, Max: 2},
		},
		{
			name:   "select: field test only",
			expr:   `has(input.single_int32)`,
			decls:  []*exprpb.Decl{decls.NewVar("input", decls.NewObjectType("google.expr.proto3.test.TestAllTypes"))},
			wanted: zeroCost,
		},
		{
			name:   "estimated function call",
			expr:   `input.getFullYear()`,
			decls:  []*exprpb.Decl{decls.NewVar("input", decls.Timestamp)},
			wanted: CostEstimate{Min: 8, Max: 8},
		},
		{
			name:   "create list",
			expr:   `[1, 2, 3]`,
			wanted: CostEstimate{Min: 10, Max: 10},
		},
		{
			name:   "create struct",
			expr:   `google.expr.proto3.test.TestAllTypes{single_int32: 1, single_float: 3.14, single_string: 'str'}`,
			wanted: CostEstimate{Min: 40, Max: 40},
		},
		{
			name:   "create map",
			expr:   `{"a": 1, "b": 2, "c": 3}`,
			wanted: CostEstimate{Min: 30, Max: 30},
		},
		{
			name:   "all comprehension",
			decls:  []*exprpb.Decl{decls.NewVar("input", allList)},
			hints:  map[string]int64{"input": 100},
			expr:   `input.all(x, true)`,
			wanted: CostEstimate{Min: 2, Max: 302},
		},
		{
			name:   "nested all comprehension",
			decls:  []*exprpb.Decl{decls.NewVar("input", nestedList)},
			hints:  map[string]int64{"input": 50, "input.@items": 10},
			expr:   `input.all(x, x.all(y, true))`,
			wanted: CostEstimate{Min: 2, Max: 1752},
		},
		{
			name:   "all comprehension on literal",
			expr:   `[1, 2, 3].all(x, true)`,
			wanted: CostEstimate{Min: 20, Max: 20},
		},
		{
			name:   "variable cost function",
			decls:  []*exprpb.Decl{decls.NewVar("input", decls.String)},
			hints:  map[string]int64{"input": 500},
			expr:   `input.matches('[0-9]')`,
			wanted: CostEstimate{Min: 3, Max: 103},
		},
		{
			name:   "variable cost function with constant",
			expr:   `'123'.matches('[0-9]')`,
			wanted: CostEstimate{Min: 2, Max: 2},
		},
		{
			name:   "or",
			expr:   `true || false`,
			wanted: zeroCost,
		},
		{
			name:   "and",
			expr:   `true && false`,
			wanted: zeroCost,
		},
		{
			name:   "lt",
			expr:   `1 < 2`,
			wanted: oneCost,
		},
		{
			name:   "lte",
			expr:   `1 <= 2`,
			wanted: oneCost,
		},
		{
			name:   "eq",
			expr:   `1 == 2`,
			wanted: oneCost,
		},
		{
			name:   "gt",
			expr:   `2 > 1`,
			wanted: oneCost,
		},
		{
			name:   "gte",
			expr:   `2 >= 1`,
			wanted: oneCost,
		},
		{
			name:   "in",
			expr:   `2 in [1, 2, 3]`,
			wanted: CostEstimate{Min: 13, Max: 13},
		},
		{
			name:   "plus",
			expr:   `1 + 1`,
			wanted: oneCost,
		},
		{
			name:   "minus",
			expr:   `1 - 1`,
			wanted: oneCost,
		},
		{
			name:   "/",
			expr:   `1 / 1`,
			wanted: oneCost,
		},
		{
			name:   "/",
			expr:   `1 * 1`,
			wanted: oneCost,
		},
		{
			name:   "%",
			expr:   `1 % 1`,
			wanted: oneCost,
		},
		{
			name:   "ternary",
			expr:   `true ? 1 : 2`,
			wanted: zeroCost,
		},
		{
			name:   "string size",
			expr:   `size("123")`,
			wanted: oneCost,
		},
		{
			name:   "bytes to string conversion",
			decls:  []*exprpb.Decl{decls.NewVar("input", decls.Bytes)},
			hints:  map[string]int64{"input": 500},
			expr:   `string(input)`,
			wanted: CostEstimate{Min: 1, Max: 51},
		},
		{
			name:   "string to bytes conversion",
			decls:  []*exprpb.Decl{decls.NewVar("input", decls.String)},
			hints:  map[string]int64{"input": 500},
			expr:   `bytes(input)`,
			wanted: CostEstimate{Min: 1, Max: 51},
		},
		{
			name:   "int to string conversion",
			expr:   `string(1)`,
			wanted: CostEstimate{Min: 1, Max: 1},
		},
		{
			name: "contains",
			expr: `input.contains(arg1)`,
			decls: []*exprpb.Decl{
				decls.NewVar("input", decls.String),
				decls.NewVar("arg1", decls.String),
			},
			hints:  map[string]int64{"input": 500, "arg1": 500},
			wanted: CostEstimate{Min: 2, Max: 2502},
		},
		{
			name: "matches",
			expr: `input.matches('\\d+a\\d+b')`,
			decls: []*exprpb.Decl{
				decls.NewVar("input", decls.String),
			},
			hints:  map[string]int64{"input": 500},
			wanted: CostEstimate{Min: 3, Max: 103},
		},
		{
			name: "startsWith",
			expr: `input.startsWith(arg1)`,
			decls: []*exprpb.Decl{
				decls.NewVar("input", decls.String),
				decls.NewVar("arg1", decls.String),
			},
			hints:  map[string]int64{"arg1": 500},
			wanted: CostEstimate{Min: 2, Max: 52},
		},
		{
			name: "endsWith",
			expr: `input.endsWith(arg1)`,
			decls: []*exprpb.Decl{
				decls.NewVar("input", decls.String),
				decls.NewVar("arg1", decls.String),
			},
			hints:  map[string]int64{"arg1": 500},
			wanted: CostEstimate{Min: 2, Max: 52},
		},
		{
			name: "size receiver",
			expr: `input.size()`,
			decls: []*exprpb.Decl{
				decls.NewVar("input", decls.String),
			},
			wanted: CostEstimate{Min: 2, Max: 2},
		},
		{
			name: "size",
			expr: `size(input)`,
			decls: []*exprpb.Decl{
				decls.NewVar("input", decls.String),
			},
			wanted: CostEstimate{Min: 2, Max: 2},
		},
		{
			name: "ternary eval",
			expr: `(x > 2 ? input1 : input2).all(y, true)`,
			decls: []*exprpb.Decl{
				decls.NewVar("x", decls.Int),
				decls.NewVar("input1", allList),
				decls.NewVar("input2", allList),
			},
			hints:  map[string]int64{"input1": 1, "input2": 1},
			wanted: CostEstimate{Min: 4, Max: 7},
		},
		{
			name: "comprehension over map",
			expr: `input.all(k, input[k].single_int32 > 3)`,
			decls: []*exprpb.Decl{
				decls.NewVar("input", allMap),
			},
			hints:  map[string]int64{"input": 10},
			wanted: CostEstimate{Min: 2, Max: 82},
		},
		{
			name: "comprehension over nested map of maps",
			expr: `input.all(k, input[k].all(x, true))`,
			decls: []*exprpb.Decl{
				decls.NewVar("input", nestedMap),
			},
			hints:  map[string]int64{"input": 5, "input.@values": 10},
			wanted: CostEstimate{Min: 2, Max: 187},
		},
		{
			name: "string size of map keys",
			expr: `input.all(k, k.contains(k))`,
			decls: []*exprpb.Decl{
				decls.NewVar("input", nestedMap),
			},
			hints:  map[string]int64{"input": 5, "input.@keys": 10},
			wanted: CostEstimate{Min: 2, Max: 32},
		},
		{
			name: "comprehension variable shadowing",
			expr: `input.all(k, input[k].all(k, true) && k.contains(k))`,
			decls: []*exprpb.Decl{
				decls.NewVar("input", nestedMap),
			},
			hints:  map[string]int64{"input": 2, "input.@values": 2, "input.@keys": 5},
			wanted: CostEstimate{Min: 2, Max: 34},
		},
		{
			name: "comprehension variable shadowing",
			expr: `input.all(k, input[k].all(k, true) && k.contains(k))`,
			decls: []*exprpb.Decl{
				decls.NewVar("input", nestedMap),
			},
			hints:  map[string]int64{"input": 2, "input.@values": 2, "input.@keys": 5},
			wanted: CostEstimate{Min: 2, Max: 34},
		},
		{
			name: "list concat",
			expr: `(list1 + list2).all(x, true)`,
			decls: []*exprpb.Decl{
				decls.NewVar("list1", decls.NewListType(decls.Int)),
				decls.NewVar("list2", decls.NewListType(decls.Int)),
			},
			hints:  map[string]int64{"list1": 10, "list2": 10},
			wanted: CostEstimate{Min: 4, Max: 64},
		},
		{
			name: "str concat",
			expr: `"abcdefg".contains(str1 + str2)`,
			decls: []*exprpb.Decl{
				decls.NewVar("str1", decls.String),
				decls.NewVar("str2", decls.String),
			},
			hints:  map[string]int64{"str1": 10, "str2": 10},
			wanted: CostEstimate{Min: 2, Max: 6},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.hints == nil {
				tc.hints = map[string]int64{}
			}
			p, err := parser.NewParser(parser.Macros(parser.AllMacros...))
			if err != nil {
				t.Fatalf("parser.NewParser() failed: %v", err)
			}
			src := common.NewStringSource(tc.expr, "<input>")
			pe, errs := p.Parse(src)
			if len(errs.GetErrors()) != 0 {
				t.Fatalf("parser.Parse(%v) failed: %v", tc.expr, errs.ToDisplayString())
			}
			reg, err := types.NewRegistry(&proto3pb.TestAllTypes{})
			if err != nil {
				t.Fatalf("types.NewRegistry(...) failed: %v", err)
			}

			e, err := NewEnv(containers.DefaultContainer, reg)
			if err != nil {
				t.Fatalf("NewEnv() failed: %v", err)
			}
			err = e.Add(StandardDeclarations()...)
			if err != nil {
				t.Fatalf("environment creation error: %v", err)
			}
			err = e.Add(tc.decls...)
			if err != nil {
				t.Fatalf("environment creation error: %s\n", err)
			}
			checked, errs := Check(pe, src, e)
			if len(errs.GetErrors()) != 0 {
				t.Fatalf("Check(%s) failed: %v", tc.expr, errs.ToDisplayString())
			}
			est := Cost(checked, testCostEstimator{hints: tc.hints})
			if est.Min != tc.wanted.Min || est.Max != tc.wanted.Max {
				t.Fatalf("Got cost interval [%v, %v], wanted [%v, %v]",
					est.Min, est.Max, tc.wanted.Min, tc.wanted.Max)
			}
		})
	}
}

type testCostEstimator struct {
	hints map[string]int64
}

func (tc testCostEstimator) EstimateSize(element AstNode) *SizeEstimate {
	if l, ok := tc.hints[strings.Join(element.Path(), ".")]; ok {
		return &SizeEstimate{Min: 0, Max: uint64(l)}
	}
	return nil
}

func (tc testCostEstimator) EstimateCallCost(function, overloadID string, target *AstNode, args []AstNode) *CallEstimate {
	switch overloadID {
	case overloads.TimestampToYear:
		return &CallEstimate{CostEstimate: CostEstimate{Min: 7, Max: 7}}
	}
	return nil
}
