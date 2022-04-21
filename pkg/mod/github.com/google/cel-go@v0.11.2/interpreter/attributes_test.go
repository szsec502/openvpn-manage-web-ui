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

package interpreter

import (
	"reflect"
	"testing"

	"github.com/google/cel-go/checker"
	"github.com/google/cel-go/checker/decls"
	"github.com/google/cel-go/common"
	"github.com/google/cel-go/common/containers"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"github.com/google/cel-go/parser"

	exprpb "google.golang.org/genproto/googleapis/api/expr/v1alpha1"
	anypb "google.golang.org/protobuf/types/known/anypb"

	proto3pb "github.com/google/cel-go/test/proto3pb"
)

func TestAttributesAbsoluteAttr(t *testing.T) {
	reg := newTestRegistry(t)
	cont, err := containers.NewContainer(containers.Name("acme.ns"))
	if err != nil {
		t.Fatal(err)
	}
	attrs := NewAttributeFactory(cont, reg, reg)
	vars, _ := NewActivation(map[string]interface{}{
		"acme.a": map[string]interface{}{
			"b": map[uint]interface{}{
				4: map[bool]string{
					false: "success",
				},
			},
		},
	})

	// acme.a.b[4][false]
	attr := attrs.AbsoluteAttribute(1, "acme.a")
	qualB, _ := attrs.NewQualifier(nil, 2, "b")
	qual4, _ := attrs.NewQualifier(nil, 3, uint64(4))
	qualFalse, _ := attrs.NewQualifier(nil, 4, false)
	attr.AddQualifier(qualB)
	attr.AddQualifier(qual4)
	attr.AddQualifier(qualFalse)
	out, err := attr.Resolve(vars)
	if err != nil {
		t.Fatal(err)
	}
	if out != types.String("success") {
		t.Errorf("Got %v (%T), wanted success", out, out)
	}
	wantedMin, wantedMax := int64(1), int64(1)
	if min, max := estimateCost(attr); min != wantedMin || max != wantedMax {
		t.Errorf("Got cost interval [%v, %v], wanted [%v, %v]", min, max, wantedMin, wantedMax)
	}
}

func TestAttributesAbsoluteAttr_Type(t *testing.T) {
	reg := newTestRegistry(t)
	attrs := NewAttributeFactory(containers.DefaultContainer, reg, reg)

	// int
	attr := attrs.AbsoluteAttribute(1, "int")
	out, err := attr.Resolve(EmptyActivation())
	if err != nil {
		t.Fatal(err)
	}
	if out != types.IntType {
		t.Errorf("Got %v, wanted success", out)
	}
	wantedMin, wantedMax := int64(1), int64(1)
	if min, max := estimateCost(attr); min != wantedMin || max != wantedMax {
		t.Errorf("Got cost interval [%v, %v], wanted [%v, %v]", min, max, wantedMin, wantedMax)
	}
}

func TestAttributesRelativeAttr(t *testing.T) {
	reg := newTestRegistry(t)
	attrs := NewAttributeFactory(containers.DefaultContainer, reg, reg)
	data := map[string]interface{}{
		"a": map[int]interface{}{
			-1: []int32{2, 42},
		},
		"b": 1,
	}
	vars, _ := NewActivation(data)

	// The relative attribute under test is applied to a map literal:
	// {
	//   a: {-1: [2, 42], b: 1}
	//   b: 1
	// }
	//
	// The expression being evaluated is: <map-literal>.a[-1][b] -> 42
	op := NewConstValue(1, reg.NativeToValue(data))
	attr := attrs.RelativeAttribute(1, op)
	qualA, _ := attrs.NewQualifier(nil, 2, "a")
	qualNeg1, _ := attrs.NewQualifier(nil, 3, int64(-1))
	attr.AddQualifier(qualA)
	attr.AddQualifier(qualNeg1)
	attr.AddQualifier(attrs.AbsoluteAttribute(4, "b"))
	out, err := attr.Resolve(vars)
	if err != nil {
		t.Fatal(err)
	}
	if out != types.Int(42) {
		t.Errorf("Got %v (%T), wanted 42", out, out)
	}
	wantedMin, wantedMax := int64(1), int64(1)
	if min, max := estimateCost(attr); min != wantedMin || max != wantedMax {
		t.Errorf("Got cost interval [%v, %v], wanted [%v, %v]", min, max, wantedMin, wantedMax)
	}
}

func TestAttributesRelativeAttr_OneOf(t *testing.T) {
	reg := newTestRegistry(t)
	cont, err := containers.NewContainer(containers.Name("acme.ns"))
	if err != nil {
		t.Fatal(err)
	}
	attrs := NewAttributeFactory(cont, reg, reg)
	data := map[string]interface{}{
		"a": map[int]interface{}{
			-1: []int32{2, 42},
		},
		"acme.b": 1,
	}
	vars, _ := NewActivation(data)

	// The relative attribute under test is applied to a map literal:
	// {
	//   a: {-1: [2, 42], b: 1}
	//   b: 1
	// }
	//
	// The expression being evaluated is: <map-literal>.a[-1][b] -> 42
	//
	// However, since the test is validating what happens with maybe attributes
	// the attribute resolution must also consider the following variations:
	// - <map-literal>.a[-1][acme.ns.b]
	// - <map-literal>.a[-1][acme.b]
	//
	// The correct behavior should yield the value of the last alternative.
	op := NewConstValue(1, reg.NativeToValue(data))
	attr := attrs.RelativeAttribute(1, op)
	qualA, _ := attrs.NewQualifier(nil, 2, "a")
	qualNeg1, _ := attrs.NewQualifier(nil, 3, int64(-1))
	attr.AddQualifier(qualA)
	attr.AddQualifier(qualNeg1)
	attr.AddQualifier(attrs.MaybeAttribute(4, "b"))
	out, err := attr.Resolve(vars)
	if err != nil {
		t.Fatal(err)
	}
	if out != types.Int(42) {
		t.Errorf("Got %v (%T), wanted 42", out, out)
	}
	wantedMin, wantedMax := int64(1), int64(1)
	if min, max := estimateCost(attr); min != wantedMin || max != wantedMax {
		t.Errorf("Got cost interval [%v, %v], wanted [%v, %v]", min, max, wantedMin, wantedMax)
	}
}

func TestAttributesRelativeAttr_Conditional(t *testing.T) {
	reg := newTestRegistry(t)
	attrs := NewAttributeFactory(containers.DefaultContainer, reg, reg)
	data := map[string]interface{}{
		"a": map[int]interface{}{
			-1: []int32{2, 42},
		},
		"b": []int{0, 1},
		"c": []interface{}{1, 0},
	}
	vars, _ := NewActivation(data)

	// The relative attribute under test is applied to a map literal:
	// {
	//   a: {-1: [2, 42], b: 1}
	//   b: [0, 1],
	//   c: {1, 0},
	// }
	//
	// The expression being evaluated is:
	// <map-literal>.a[-1][(false ? b : c)[0]] -> 42
	//
	// Effectively the same as saying <map-literal>.a[-1][c[0]]
	cond := NewConstValue(2, types.False)
	condAttr := attrs.ConditionalAttribute(4, cond,
		attrs.AbsoluteAttribute(5, "b"),
		attrs.AbsoluteAttribute(6, "c"))
	qual0, _ := attrs.NewQualifier(nil, 7, 0)
	condAttr.AddQualifier(qual0)

	obj := NewConstValue(1, reg.NativeToValue(data))
	attr := attrs.RelativeAttribute(1, obj)
	qualA, _ := attrs.NewQualifier(nil, 2, "a")
	qualNeg1, _ := attrs.NewQualifier(nil, 3, int64(-1))
	attr.AddQualifier(qualA)
	attr.AddQualifier(qualNeg1)
	attr.AddQualifier(condAttr)
	out, err := attr.Resolve(vars)
	if err != nil {
		t.Fatal(err)
	}
	if out != types.Int(42) {
		t.Errorf("Got %v (%T), wanted 42", out, out)
	}
	wantedMin, wantedMax := int64(1), int64(1)
	if min, max := estimateCost(attr); min != wantedMin || max != wantedMax {
		t.Errorf("Got cost interval [%v, %v], wanted [%v, %v]", min, max, wantedMin, wantedMax)
	}
}

func TestAttributesRelativeAttr_Relative(t *testing.T) {
	cont, err := containers.NewContainer(containers.Name("acme.ns"))
	if err != nil {
		t.Fatal(err)
	}
	reg := newTestRegistry(t)
	attrs := NewAttributeFactory(cont, reg, reg)
	data := map[string]interface{}{
		"a": map[int]interface{}{
			-1: map[string]interface{}{
				"first":  uint(1),
				"second": uint(2),
				"third":  uint(3),
			},
		},
		"b": uint32(2),
	}
	vars, _ := NewActivation(data)

	// The environment declares the following variables:
	// {
	//   a: {
	//     -1: {
	//       "first": 1u,
	//       "second": 2u,
	//       "third": 3u,
	//     }
	//   },
	//   b: 2u,
	// }
	//
	// The map of input variables is also re-used as a map-literal <obj> in the expression.
	//
	// The relative object under test is the following map literal.
	// <mp> {
	//   1u: "first",
	//   2u: "second",
	//   3u: "third",
	// }
	//
	// The expression under test is:
	//   <obj>.a[-1][<mp>[b]]
	//
	// This is equivalent to:
	//   <obj>.a[-1]["second"] -> 2u
	obj := NewConstValue(1, reg.NativeToValue(data))
	mp := NewConstValue(1, reg.NativeToValue(map[uint32]interface{}{
		1: "first",
		2: "second",
		3: "third",
	}))
	relAttr := attrs.RelativeAttribute(4, mp)
	qualB, _ := attrs.NewQualifier(nil, 5, attrs.AbsoluteAttribute(5, "b"))
	relAttr.AddQualifier(qualB)
	attr := attrs.RelativeAttribute(1, obj)
	qualA, _ := attrs.NewQualifier(nil, 2, "a")
	qualNeg1, _ := attrs.NewQualifier(nil, 3, int64(-1))
	attr.AddQualifier(qualA)
	attr.AddQualifier(qualNeg1)
	attr.AddQualifier(relAttr)

	out, err := attr.Resolve(vars)
	if err != nil {
		t.Fatal(err)
	}
	if out != types.Uint(2) {
		t.Errorf("Got %v (%T), wanted 2", out, out)
	}
	wantedMin, wantedMax := int64(1), int64(1)
	if min, max := estimateCost(attr); min != wantedMin || max != wantedMax {
		t.Errorf("Got cost interval [%v, %v], wanted [%v, %v]", min, max, wantedMin, wantedMax)
	}
}

func TestAttributesOneofAttr(t *testing.T) {
	reg := newTestRegistry(t)
	cont, err := containers.NewContainer(containers.Name("acme.ns"))
	if err != nil {
		t.Fatal(err)
	}
	attrs := NewAttributeFactory(cont, reg, reg)
	data := map[string]interface{}{
		"a": map[string]interface{}{
			"b": []int32{2, 42},
		},
		"acme.a.b":    1,
		"acme.ns.a.b": "found",
	}
	vars, _ := NewActivation(data)

	// a.b -> should resolve to acme.ns.a.b per namespace resolution rules.
	attr := attrs.MaybeAttribute(1, "a")
	qualB, _ := attrs.NewQualifier(nil, 2, "b")
	attr.AddQualifier(qualB)
	out, err := attr.Resolve(vars)
	if err != nil {
		t.Fatal(err)
	}
	if out != "found" {
		t.Errorf("Got %v, wanted 'found'", out)
	}
	wantedMin, wantedMax := int64(1), int64(1)
	if min, max := estimateCost(attr); min != wantedMin || max != wantedMax {
		t.Errorf("Got cost interval [%v, %v], wanted [%v, %v]", min, max, wantedMin, wantedMax)
	}
}

func TestAttributesConditionalAttr_TrueBranch(t *testing.T) {
	reg := newTestRegistry(t)
	attrs := NewAttributeFactory(containers.DefaultContainer, reg, reg)
	data := map[string]interface{}{
		"a": map[int]interface{}{
			-1: []int32{2, 42},
		},
		"b": map[string]interface{}{
			"c": map[int32]interface{}{
				-1: []uint{2, 42},
			},
		},
	}
	vars, _ := NewActivation(data)

	// (true ? a : b.c)[-1][1]
	tv := attrs.AbsoluteAttribute(2, "a")
	fv := attrs.MaybeAttribute(3, "b")
	qualC, _ := attrs.NewQualifier(nil, 4, "c")
	fv.AddQualifier(qualC)
	cond := attrs.ConditionalAttribute(1, NewConstValue(0, types.True), tv, fv)
	qualNeg1, _ := attrs.NewQualifier(nil, 5, int64(-1))
	qual1, _ := attrs.NewQualifier(nil, 6, int64(1))
	cond.AddQualifier(qualNeg1)
	cond.AddQualifier(qual1)
	out, err := cond.Resolve(vars)
	if err != nil {
		t.Fatal(err)
	}
	if out != int32(42) {
		t.Errorf("Got %v (%T), wanted 42", out, out)
	}
	wantedMin, wantedMax := int64(1), int64(1)
	if min, max := estimateCost(fv); min != wantedMin || max != wantedMax {
		t.Errorf("Got cost interval [%v, %v], wanted [%v, %v]", min, max, wantedMin, wantedMax)
	}
}

func TestAttributesConditionalAttr_FalseBranch(t *testing.T) {
	reg := newTestRegistry(t)
	attrs := NewAttributeFactory(containers.DefaultContainer, reg, reg)
	data := map[string]interface{}{
		"a": map[int]interface{}{
			-1: []int32{2, 42},
		},
		"b": map[string]interface{}{
			"c": map[int32]interface{}{
				-1: []uint{2, 42},
			},
		},
	}
	vars, _ := NewActivation(data)

	// (false ? a : b.c)[-1][1]
	tv := attrs.AbsoluteAttribute(2, "a")
	fv := attrs.MaybeAttribute(3, "b")
	qualC, _ := attrs.NewQualifier(nil, 4, "c")
	fv.AddQualifier(qualC)
	cond := attrs.ConditionalAttribute(1, NewConstValue(0, types.False), tv, fv)
	qualNeg1, _ := attrs.NewQualifier(nil, 5, int64(-1))
	qual1, _ := attrs.NewQualifier(nil, 6, int64(1))
	cond.AddQualifier(qualNeg1)
	cond.AddQualifier(qual1)
	out, err := cond.Resolve(vars)
	if err != nil {
		t.Fatal(err)
	}
	if out != uint(42) {
		t.Errorf("Got %v (%T), wanted 42", out, out)
	}
	wantedMin, wantedMax := int64(1), int64(1)
	if min, max := estimateCost(fv); min != wantedMin || max != wantedMax {
		t.Errorf("Got cost interval [%v, %v], wanted [%v, %v]", min, max, wantedMin, wantedMax)
	}
}

func TestAttributesConditionalAttr_ErrorUnknown(t *testing.T) {
	reg := newTestRegistry(t)
	attrs := NewAttributeFactory(containers.DefaultContainer, reg, reg)

	// err ? a : b
	tv := attrs.AbsoluteAttribute(2, "a")
	fv := attrs.MaybeAttribute(3, "b")
	cond := attrs.ConditionalAttribute(1, NewConstValue(0, types.NewErr("test error")), tv, fv)
	out, err := cond.Resolve(EmptyActivation())
	if err == nil {
		t.Errorf("Got %v, wanted error", out)
	}
	wantedMin, wantedMax := int64(1), int64(1)
	if min, max := estimateCost(fv); min != wantedMin || max != wantedMax {
		t.Errorf("Got cost interval [%v, %v], wanted [%v, %v]", min, max, wantedMin, wantedMax)
	}

	// unk ? a : b
	condUnk := attrs.ConditionalAttribute(1, NewConstValue(0, types.Unknown{1}), tv, fv)
	out, err = condUnk.Resolve(EmptyActivation())
	if err != nil {
		t.Fatal(err)
	}
	unk, ok := out.(types.Unknown)
	if !ok || !types.IsUnknown(unk) {
		t.Errorf("Got %v, wanted unknown", out)
	}
	wantedMin, wantedMax = int64(1), int64(1)
	if min, max := estimateCost(fv); min != wantedMin || max != wantedMax {
		t.Errorf("Got cost interval [%v, %v], wanted [%v, %v]", min, max, wantedMin, wantedMax)
	}
}

func BenchmarkResolverFieldQualifier(b *testing.B) {
	msg := &proto3pb.TestAllTypes{
		NestedType: &proto3pb.TestAllTypes_SingleNestedMessage{
			SingleNestedMessage: &proto3pb.TestAllTypes_NestedMessage{
				Bb: 123,
			},
		},
	}
	reg := newBenchRegistry(b, msg)
	attrs := NewAttributeFactory(containers.DefaultContainer, reg, reg)
	vars, _ := NewActivation(map[string]interface{}{
		"msg": msg,
	})
	attr := attrs.AbsoluteAttribute(1, "msg")
	opType, found := reg.FindType("google.expr.proto3.test.TestAllTypes")
	if !found {
		b.Fatal("FindType() could not find TestAllTypes")
	}
	fieldType, found := reg.FindType("google.expr.proto3.test.TestAllTypes.NestedMessage")
	if !found {
		b.Fatal("FindType() could not find NestedMessage")
	}
	attr.AddQualifier(makeQualifier(b, attrs, opType.GetType(), 2, "single_nested_message"))
	attr.AddQualifier(makeQualifier(b, attrs, fieldType.GetType(), 3, "bb"))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := attr.Resolve(vars)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func TestResolverCustomQualifier(t *testing.T) {
	reg := newTestRegistry(t)
	attrs := &custAttrFactory{
		AttributeFactory: NewAttributeFactory(containers.DefaultContainer, reg, reg),
	}
	msg := &proto3pb.TestAllTypes_NestedMessage{
		Bb: 123,
	}
	vars, _ := NewActivation(map[string]interface{}{
		"msg": msg,
	})
	attr := attrs.AbsoluteAttribute(1, "msg")
	qualBB, _ := attrs.NewQualifier(&exprpb.Type{
		TypeKind: &exprpb.Type_MessageType{
			MessageType: "google.expr.proto3.test.TestAllTypes.NestedMessage",
		},
	}, 2, "bb")
	attr.AddQualifier(qualBB)
	out, err := attr.Resolve(vars)
	if err != nil {
		t.Error(err)
	}
	if out != int32(123) {
		t.Errorf("Got %v, wanted 123", out)
	}
	wantedMin, wantedMax := int64(1), int64(1)
	if min, max := estimateCost(attr); min != wantedMin || max != wantedMax {
		t.Errorf("Got cost interval [%v, %v], wanted [%v, %v]", min, max, wantedMin, wantedMax)
	}
}

func TestAttributesMissingMsg(t *testing.T) {
	reg := newTestRegistry(t)
	attrs := NewAttributeFactory(containers.DefaultContainer, reg, reg)
	any, _ := anypb.New(&proto3pb.TestAllTypes{})
	vars, _ := NewActivation(map[string]interface{}{
		"missing_msg": any,
	})

	// missing_msg.field
	attr := attrs.AbsoluteAttribute(1, "missing_msg")
	field, _ := attrs.NewQualifier(nil, 2, "field")
	attr.AddQualifier(field)
	out, err := attr.Resolve(vars)
	if err == nil {
		t.Fatalf("got %v, wanted error", out)
	}
	if err.Error() != "unknown type: 'google.expr.proto3.test.TestAllTypes'" {
		t.Fatalf("got %v, wanted unknown type: 'google.expr.proto3.test.TestAllTypes'", err)
	}
}

func TestAttributeMissingMsg_UnknownField(t *testing.T) {
	reg := newTestRegistry(t)
	attrs := NewPartialAttributeFactory(containers.DefaultContainer, reg, reg)
	any, _ := anypb.New(&proto3pb.TestAllTypes{})
	vars, _ := NewPartialActivation(map[string]interface{}{
		"missing_msg": any,
	}, NewAttributePattern("missing_msg").QualString("field"))

	// missing_msg.field
	attr := attrs.AbsoluteAttribute(1, "missing_msg")
	field, _ := attrs.NewQualifier(nil, 2, "field")
	attr.AddQualifier(field)
	out, err := attr.Resolve(vars)
	if err != nil {
		t.Fatal(err)
	}
	_, isUnk := out.(types.Unknown)
	if !isUnk {
		t.Errorf("got %v, wanted unknown value", out)
	}
}

func TestAttributeStateTracking(t *testing.T) {
	var tests = []struct {
		expr  string
		env   []*exprpb.Decl
		in    map[string]interface{}
		out   ref.Val
		state map[int64]interface{}
	}{
		{
			expr: `[{"field": true}][0].field`,
			env:  []*exprpb.Decl{},
			in:   map[string]interface{}{},
			out:  types.True,
			state: map[int64]interface{}{
				// overall expression
				1: true,
				// [{"field": true}][0]
				6: map[ref.Val]ref.Val{types.String("field"): types.True},
				// [{"field": true}][0].field
				8: true,
			},
		},
		{
			expr: `a[1]['two']`,
			env: []*exprpb.Decl{
				decls.NewVar("a", decls.NewMapType(
					decls.Int,
					decls.NewMapType(decls.String, decls.Bool))),
			},
			in: map[string]interface{}{
				"a": map[int64]interface{}{
					1: map[string]bool{
						"two": true,
					},
				},
			},
			out: types.True,
			state: map[int64]interface{}{
				// overall expression
				1: true,
				// a[1]
				2: map[string]bool{"two": true},
				// a[1]["two"]
				4: true,
			},
		},
		{
			expr: `a[1][2][3]`,
			env: []*exprpb.Decl{
				decls.NewVar("a", decls.NewMapType(
					decls.Int,
					decls.NewMapType(decls.Dyn, decls.Dyn))),
			},
			in: map[string]interface{}{
				"a": map[int64]interface{}{
					1: map[int64]interface{}{
						1: 0,
						2: []string{"index", "middex", "outdex", "dex"},
					},
				},
			},
			out: types.String("dex"),
			state: map[int64]interface{}{
				// overall expression
				1: "dex",
				// a[1]
				2: map[int64]interface{}{
					1: 0,
					2: []string{"index", "middex", "outdex", "dex"},
				},
				// a[1][2]
				4: []string{"index", "middex", "outdex", "dex"},
				// a[1][2][3]
				6: "dex",
			},
		},
		{
			expr: `a[1][2][a[1][1]]`,
			env: []*exprpb.Decl{
				decls.NewVar("a", decls.NewMapType(
					decls.Int,
					decls.NewMapType(decls.Dyn, decls.Dyn))),
			},
			in: map[string]interface{}{
				"a": map[int64]interface{}{
					1: map[int64]interface{}{
						1: 0,
						2: []string{"index", "middex", "outdex", "dex"},
					},
				},
			},
			out: types.String("index"),
			state: map[int64]interface{}{
				// overall expression
				1: "index",
				// a[1]
				2: map[int64]interface{}{
					1: 0,
					2: []string{"index", "middex", "outdex", "dex"},
				},
				// a[1][2]
				4: []string{"index", "middex", "outdex", "dex"},
				// a[1][2][a[1][1]]
				6: "index",
				// dynamic index into a[1][2]
				// a[1]
				8: map[int64]interface{}{
					1: 0,
					2: []string{"index", "middex", "outdex", "dex"},
				},
				// a[1][1]
				10: int64(0),
			},
		},
	}
	for _, test := range tests {
		tc := test
		t.Run(tc.expr, func(t *testing.T) {
			src := common.NewTextSource(tc.expr)
			parsed, errors := parser.Parse(src)
			if len(errors.GetErrors()) != 0 {
				t.Fatalf(errors.ToDisplayString())
			}
			cont := containers.DefaultContainer
			reg := newTestRegistry(t)
			env, err := checker.NewEnv(cont, reg)
			if err != nil {
				t.Fatalf("checker.NewEnv() failed: %v", err)
			}
			env.Add(checker.StandardDeclarations()...)
			if tc.env != nil {
				env.Add(tc.env...)
			}
			checked, errors := checker.Check(parsed, src, env)
			if len(errors.GetErrors()) != 0 {
				t.Fatalf(errors.ToDisplayString())
			}
			attrs := NewAttributeFactory(cont, reg, reg)
			interp := NewStandardInterpreter(cont, reg, reg, attrs)
			// Show that program planning will now produce an error.
			st := NewEvalState()
			i, err := interp.NewInterpretable(checked, Optimize(), Observe(EvalStateObserver(st)))
			if err != nil {
				t.Fatal(err)
			}
			if err != nil {
				t.Fatal(err)
			}
			in, _ := NewActivation(tc.in)
			out := i.Eval(in)
			if tc.out.Equal(out) != types.True {
				t.Errorf("got %v, wanted %v", out.Value(), tc.out)
			}
			for id, val := range tc.state {
				stVal, found := st.Value(id)
				if !found {
					t.Errorf("state not found for %d=%v", id, val)
					continue
				}
				if !reflect.DeepEqual(stVal.Value(), val) {
					t.Errorf("got %v, wanted %v for id: %d", stVal.Value(), val, id)
				}
			}
		})
	}
}

func BenchmarkResolverCustomQualifier(b *testing.B) {
	reg := newBenchRegistry(b)
	attrs := &custAttrFactory{
		AttributeFactory: NewAttributeFactory(containers.DefaultContainer, reg, reg),
	}
	msg := &proto3pb.TestAllTypes_NestedMessage{
		Bb: 123,
	}
	vars, _ := NewActivation(map[string]interface{}{
		"msg": msg,
	})
	attr := attrs.AbsoluteAttribute(1, "msg")
	qualBB, _ := attrs.NewQualifier(&exprpb.Type{
		TypeKind: &exprpb.Type_MessageType{
			MessageType: "google.expr.proto3.test.TestAllTypes.NestedMessage",
		},
	}, 2, "bb")
	attr.AddQualifier(qualBB)
	for i := 0; i < b.N; i++ {
		attr.Resolve(vars)
	}
}

type custAttrFactory struct {
	AttributeFactory
}

func (r *custAttrFactory) NewQualifier(objType *exprpb.Type,
	qualID int64, val interface{}) (Qualifier, error) {
	if objType.GetMessageType() == "google.expr.proto3.test.TestAllTypes.NestedMessage" {
		return &nestedMsgQualifier{id: qualID, field: val.(string)}, nil
	}
	return r.AttributeFactory.NewQualifier(objType, qualID, val)
}

type nestedMsgQualifier struct {
	id    int64
	field string
}

func (q *nestedMsgQualifier) ID() int64 {
	return q.id
}

func (q *nestedMsgQualifier) Qualify(vars Activation, obj interface{}) (interface{}, error) {
	pb := obj.(*proto3pb.TestAllTypes_NestedMessage)
	return pb.GetBb(), nil
}

// Cost implements the Coster interface method. It returns zero for testing purposes.
func (q *nestedMsgQualifier) Cost() (min, max int64) {
	return 0, 0
}

func makeQualifier(b *testing.B, attrs AttributeFactory, typ *exprpb.Type, qualID int64, val interface{}) Qualifier {
	b.Helper()
	qual, err := attrs.NewQualifier(typ, qualID, val)
	if err != nil {
		b.Fatalf("attrs.NewQualifier() failed: %v", err)
	}
	return qual
}
