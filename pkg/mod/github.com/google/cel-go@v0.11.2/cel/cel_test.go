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

package cel

import (
	"context"
	"fmt"
	"io/ioutil"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"

	"github.com/google/cel-go/checker"
	"github.com/google/cel-go/checker/decls"
	"github.com/google/cel-go/common"
	"github.com/google/cel-go/common/operators"
	"github.com/google/cel-go/common/overloads"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"github.com/google/cel-go/common/types/traits"
	"github.com/google/cel-go/interpreter"
	"github.com/google/cel-go/interpreter/functions"
	"github.com/google/cel-go/parser"
	"github.com/google/cel-go/test"

	exprpb "google.golang.org/genproto/googleapis/api/expr/v1alpha1"
	descpb "google.golang.org/protobuf/types/descriptorpb"
	dynamicpb "google.golang.org/protobuf/types/dynamicpb"

	proto2pb "github.com/google/cel-go/test/proto2pb"
	proto3pb "github.com/google/cel-go/test/proto3pb"
)

func Test_ExampleWithBuiltins(t *testing.T) {
	// Variables used within this expression environment.
	decls := Declarations(
		decls.NewVar("i", decls.String),
		decls.NewVar("you", decls.String))
	env, err := NewEnv(decls)
	if err != nil {
		t.Fatalf("environment creation error: %s\n", err)
	}

	// Compile the expression.
	ast, iss := env.Compile(`"Hello " + you + "! I'm " + i + "."`)
	if iss.Err() != nil {
		t.Fatal(iss.Err())
	}

	// Create the program, and evaluate it against some input.
	prg, err := env.Program(ast)
	if err != nil {
		t.Fatalf("program creation error: %s\n", err)
	}

	// If the Eval() call were provided with cel.EvalOptions(OptTrackState) the details response
	// (2nd return) would be non-nil.
	out, _, err := prg.Eval(map[string]interface{}{
		"i":   "CEL",
		"you": "world"})
	if err != nil {
		t.Fatalf("runtime error: %s\n", err)
	}

	// Hello world! I'm CEL.
	if out.Equal(types.String("Hello world! I'm CEL.")) != types.True {
		t.Errorf(`got '%v', wanted "Hello world! I'm CEL."`, out.Value())
	}
}

func TestAbbrevsCompiled(t *testing.T) {
	// Test whether abbreviations successfully resolve at type-check time (compile time).
	env, err := NewEnv(
		Abbrevs("qualified.identifier.name"),
		Declarations(
			decls.NewVar("qualified.identifier.name.first", decls.String),
		),
	)
	if err != nil {
		t.Fatal(err)
	}
	ast, iss := env.Compile(`"hello "+ name.first`) // abbreviation resolved here.
	if iss.Err() != nil {
		t.Fatal(iss.Err())
	}
	prg, err := env.Program(ast)
	if err != nil {
		t.Fatal(err)
	}
	out, _, err := prg.Eval(
		map[string]interface{}{
			"qualified.identifier.name.first": "Jim",
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if out.Value() != "hello Jim" {
		t.Errorf("got %v, wanted 'hello Jim'", out)
	}
}

func TestAbbrevsParsed(t *testing.T) {
	// Test whether abbreviations are resolved properly at evaluation time.
	env, err := NewEnv(
		Abbrevs("qualified.identifier.name"),
	)
	if err != nil {
		t.Fatal(err)
	}
	ast, iss := env.Parse(`"hello " + name.first`)
	if iss.Err() != nil {
		t.Fatal(iss.Err())
	}
	prg, err := env.Program(ast) // abbreviation resolved here.
	if err != nil {
		t.Fatal(err)
	}
	out, _, err := prg.Eval(
		map[string]interface{}{
			"qualified.identifier.name": map[string]string{
				"first": "Jim",
			},
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if out.Value() != "hello Jim" {
		t.Errorf("got %v, wanted 'hello Jim'", out)
	}
}

func TestAbbrevs_Disambiguation(t *testing.T) {
	env, err := NewEnv(
		Abbrevs("external.Expr"),
		Container("google.api.expr.v1alpha1"),
		Types(&exprpb.Expr{}),
		Declarations(
			decls.NewVar("test", decls.Bool),
			decls.NewVar("external.Expr", decls.String),
		),
	)
	if err != nil {
		t.Fatal(err)
	}
	// This expression will return either a string or a protobuf Expr value depending on the value
	// of the 'test' argument. The fully qualified type name is used indicate that the protobuf
	// typed 'Expr' should be used rather than the abbreviatation for 'external.Expr'.
	ast, iss := env.Compile(`test ? dyn(Expr) : google.api.expr.v1alpha1.Expr{id: 1}`)
	if iss.Err() != nil {
		t.Fatal(iss.Err())
	}
	prg, err := env.Program(ast)
	if err != nil {
		t.Fatal(err)
	}
	out, _, err := prg.Eval(
		map[string]interface{}{
			"test":          true,
			"external.Expr": "string expr",
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if out.Value() != "string expr" {
		t.Errorf("got %v, wanted 'string expr'", out)
	}
	out, _, err = prg.Eval(
		map[string]interface{}{
			"test":          false,
			"external.Expr": "wrong expr",
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	want := &exprpb.Expr{Id: 1}
	got, _ := out.ConvertToNative(reflect.TypeOf(want))
	if !proto.Equal(got.(*exprpb.Expr), want) {
		t.Errorf("got %v, wanted '%v'", out, want)
	}
}

func TestCustomEnvError(t *testing.T) {
	e, err := NewCustomEnv(StdLib(), StdLib())
	if err != nil {
		t.Fatal(err)
	}
	_, iss := e.Compile("a.b.c == true")
	if iss.Err() == nil {
		t.Error("got successful compile, expected error for duplicate function declarations.")
	}
}

func TestCustomEnv(t *testing.T) {
	e, _ := NewCustomEnv(
		Declarations(decls.NewVar("a.b.c", decls.Bool)))

	t.Run("err", func(t *testing.T) {
		_, iss := e.Compile("a.b.c == true")
		if iss.Err() == nil {
			t.Error("got successful compile, expected error for missing operator '_==_'")
		}
	})

	t.Run("ok", func(t *testing.T) {
		ast, iss := e.Compile("a.b.c")
		if iss.Err() != nil {
			t.Fatal(iss.Err())
		}
		prg, _ := e.Program(ast)
		out, _, _ := prg.Eval(map[string]interface{}{"a.b.c": true})
		if out != types.True {
			t.Errorf("got '%v', wanted 'true'", out.Value())
		}
	})
}

func TestHomogeneousAggregateLiterals(t *testing.T) {
	e, err := NewCustomEnv(
		Declarations(
			decls.NewVar("name", decls.String),
			decls.NewFunction(
				operators.In,
				decls.NewOverload(overloads.InList, []*exprpb.Type{
					decls.String, decls.NewListType(decls.String),
				}, decls.Bool),
				decls.NewOverload(overloads.InMap, []*exprpb.Type{
					decls.String, decls.NewMapType(decls.String, decls.Bool),
				}, decls.Bool))),
		HomogeneousAggregateLiterals())
	if err != nil {
		t.Fatalf("NewCustomEnv() failed: %v", err)
	}

	funcs := Functions(&functions.Overload{
		Operator: operators.In,
		Binary: func(lhs ref.Val, rhs ref.Val) ref.Val {
			if rhs.Type().HasTrait(traits.ContainerType) {
				return rhs.(traits.Container).Contains(lhs)
			}
			return types.ValOrErr(rhs, "no such overload")
		},
	})

	tests := []struct {
		name string
		expr string
		iss  string
		vars map[string]interface{}
		out  ref.Val
	}{
		{
			name: "err_list",
			expr: `name in ['hello', 0]`,
			iss: `
			ERROR: <input>:1:19: expected type 'string' but found 'int'
             | name in ['hello', 0]
             | ..................^`,
		},
		{
			name: "err_map_key",
			expr: `name in {'hello':'world', 1:'!'}`,
			iss: `
			ERROR: <input>:1:6: found no matching overload for '@in' applied to '(string, map(!error!, string))'
			 | name in {'hello':'world', 1:'!'}
			 | .....^
			ERROR: <input>:1:27: expected type 'string' but found 'int'
             | name in {'hello':'world', 1:'!'}
             | ..........................^`,
		},
		{
			name: "err_map_value",
			expr: `name in {'hello':'world', 'goodbye':true}`,
			iss: `
			ERROR: <input>:1:37: expected type 'string' but found 'bool'
             | name in {'hello':'world', 'goodbye':true}
             | ....................................^`,
		},
		{
			name: "ok_list",
			expr: `name in ['hello', 'world']`,
			vars: map[string]interface{}{"name": "world"},
			out:  types.True,
		},
		{
			name: "ok_map",
			expr: `name in {'hello': false, 'world': true}`,
			vars: map[string]interface{}{"name": "world"},
			out:  types.True,
		},
	}
	for _, tst := range tests {
		tc := tst
		t.Run(tc.name, func(t *testing.T) {
			ast, iss := e.Compile(tc.expr)
			if tc.iss != "" {
				if iss.Err() == nil {
					t.Fatalf("e.Compile(%v) returned ast, expected error: %v", tc.expr, tc.iss)
				}
				if !test.Compare(iss.Err().Error(), tc.iss) {
					t.Fatalf("e.Compile(%v) returned %v, expected error: %v", tc.expr, iss.Err(), tc.iss)
				}
				return
			}
			if iss.Err() != nil {
				t.Fatalf("e.Compile(%v) failed: %v", tc.expr, iss.Err())
			}
			prg, err := e.Program(ast, funcs)
			if err != nil {
				t.Fatalf("e.Program() failed: %v", err)
			}
			out, _, err := prg.Eval(tc.vars)
			if err != nil {
				t.Fatalf("prg.Eval(%v) errored: %v", tc.vars, err)
			}
			if out != tc.out {
				t.Errorf("program eval got %v, wanted %v", out, tc.out)
			}
		})
	}
}

func TestCrossTypeNumericComparisons(t *testing.T) {
	tests := []struct {
		name string
		expr string
		iss  string
		opt  EnvOption
		out  ref.Val
	}{
		// Statically typed expressions need to opt in to cross-type numeric comparisons
		{
			name: "double_less_than_int_err",
			expr: `1.0 < 2`,
			opt:  CrossTypeNumericComparisons(false),
			iss: `
			ERROR: <input>:1:5: found no matching overload for '_<_' applied to '(double, int)'
             | 1.0 < 2
             | ....^`,
		},
		{
			name: "double_less_than_int_success",
			expr: `1.0 < 2`,
			opt:  CrossTypeNumericComparisons(true),
			out:  types.True,
		},
		// Dynamic data already benefits from cross-type numeric comparisons
		{
			name: "dyn_less_than_int_success",
			expr: `dyn(1.0) < 2`,
			opt:  CrossTypeNumericComparisons(false),
			out:  types.True,
		},
		{
			name: "dyn_less_than_int_success",
			expr: `dyn(1.0) < 2`,
			opt:  CrossTypeNumericComparisons(true),
			out:  types.True,
		},
	}
	for _, tst := range tests {
		tc := tst
		t.Run(tc.name, func(t *testing.T) {
			e, err := NewEnv(tc.opt)
			if err != nil {
				t.Fatalf("NewEnv() failed: %v", err)
			}
			ast, iss := e.Compile(tc.expr)
			if tc.iss != "" {
				if iss.Err() == nil {
					t.Fatalf("e.Compile(%v) returned ast, expected error: %v", tc.expr, tc.iss)
				}
				if !test.Compare(iss.Err().Error(), tc.iss) {
					t.Fatalf("e.Compile(%v) returned %v, expected error: %v", tc.expr, iss.Err(), tc.iss)
				}
				return
			}
			if iss.Err() != nil {
				t.Fatalf("e.Compile(%v) failed: %v", tc.expr, iss.Err())
			}
			prg, err := e.Program(ast)
			if err != nil {
				t.Fatalf("e.Program() failed: %v", err)
			}
			out, _, err := prg.Eval(NoVars())
			if err != nil {
				t.Fatalf("prg.Eval() errored: %v", err)
			}
			if out != tc.out {
				t.Errorf("program eval got %v, wanted %v", out, tc.out)
			}
		})
	}
}

func TestCustomTypes(t *testing.T) {
	exprType := decls.NewObjectType("google.api.expr.v1alpha1.Expr")
	reg := types.NewEmptyRegistry()
	e, _ := NewEnv(
		CustomTypeAdapter(reg),
		CustomTypeProvider(reg),
		Container("google.api.expr.v1alpha1"),
		Types(
			&exprpb.Expr{},
			types.BoolType,
			types.IntType,
			types.StringType),
		Declarations(
			decls.NewVar("expr", exprType)))

	ast, _ := e.Compile(`
		expr == Expr{id: 2,
			call_expr: Expr.Call{
				function: "_==_",
				args: [
					Expr{id: 1, ident_expr: Expr.Ident{ name: "a" }},
					Expr{id: 3, ident_expr: Expr.Ident{ name: "b" }}]
			}}`)
	if !proto.Equal(ast.ResultType(), decls.Bool) {
		t.Fatalf("got %v, wanted type bool", ast.ResultType())
	}
	prg, _ := e.Program(ast)
	vars := map[string]interface{}{"expr": &exprpb.Expr{
		Id: 2,
		ExprKind: &exprpb.Expr_CallExpr{
			CallExpr: &exprpb.Expr_Call{
				Function: "_==_",
				Args: []*exprpb.Expr{
					{
						Id: 1,
						ExprKind: &exprpb.Expr_IdentExpr{
							IdentExpr: &exprpb.Expr_Ident{Name: "a"},
						},
					},
					{
						Id: 3,
						ExprKind: &exprpb.Expr_IdentExpr{
							IdentExpr: &exprpb.Expr_Ident{Name: "b"},
						},
					},
				},
			},
		},
	}}
	out, _, _ := prg.Eval(vars)
	if out != types.True {
		t.Errorf("got '%v', wanted 'true'", out.Value())
	}
}

func TestTypeIsolation(t *testing.T) {
	b, err := ioutil.ReadFile("testdata/team.fds")
	if err != nil {
		t.Fatal("can't read fds file: ", err)
	}
	var fds descpb.FileDescriptorSet
	if err = proto.Unmarshal(b, &fds); err != nil {
		t.Fatal("can't unmarshal descriptor data: ", err)
	}

	e, err := NewEnv(
		TypeDescs(&fds),
		Declarations(
			decls.NewVar("myteam",
				decls.NewObjectType("cel.testdata.Team"))))
	if err != nil {
		t.Fatalf("NewEnv() failed: %v", err)
	}

	src := "myteam.members[0].name == 'Cyclops'"
	_, iss := e.Compile(src)
	if iss.Err() != nil {
		t.Error(iss.Err())
	}

	// Ensure that isolated types don't leak through.
	e2, _ := NewEnv(
		Declarations(
			decls.NewVar("myteam",
				decls.NewObjectType("cel.testdata.Team"))))
	_, iss = e2.Compile(src)
	if iss == nil || iss.Err() == nil {
		t.Errorf("wanted compile failure for unknown message.")
	}
}

func TestDynamicProto(t *testing.T) {
	b, err := ioutil.ReadFile("testdata/team.fds")
	if err != nil {
		t.Fatalf("ioutil.ReadFile() failed: %v", err)
	}
	var fds descpb.FileDescriptorSet
	if err = proto.Unmarshal(b, &fds); err != nil {
		t.Fatalf("proto.Unmarshal() failed: %v", err)
	}
	files := (&fds).GetFile()
	fileCopy := make([]interface{}, len(files))
	for i := 0; i < len(files); i++ {
		fileCopy[i] = files[i]
	}
	pbFiles, err := protodesc.NewFiles(&fds)
	if err != nil {
		t.Fatalf("protodesc.NewFiles() failed: %v", err)
	}
	e, err := NewEnv(
		Container("cel"),
		// The following is identical to registering the FileDescriptorSet;
		// however, it tests a different code path which aggregates individual
		// FileDescriptorProto values together.
		TypeDescs(fileCopy...),
		// Additionally, demonstrate that double registration of files doesn't
		// cause any problems.
		TypeDescs(pbFiles),
	)
	if err != nil {
		t.Fatalf("NewEnv() failed: %v", err)
	}
	src := `testdata.Team{name: 'X-Men', members: [
		testdata.Mutant{name: 'Jean Grey', level: 20},
		testdata.Mutant{name: 'Cyclops', level: 7},
		testdata.Mutant{name: 'Storm', level: 7},
		testdata.Mutant{name: 'Wolverine', level: 11}
	]}`
	ast, iss := e.Compile(src)
	if iss.Err() != nil {
		t.Fatalf("env.Compile(%s) failed: %v", src, iss.Err())
	}
	prg, err := e.Program(ast, EvalOptions(OptOptimize))
	if err != nil {
		t.Fatalf("env.Program() failed: %v", err)
	}
	out, _, err := prg.Eval(NoVars())
	if err != nil {
		t.Fatalf("program.Eval() failed: %v", err)
	}
	obj, ok := out.(traits.Indexer)
	if !ok {
		t.Fatalf("unable to convert output to object: %v", out)
	}
	if obj.Get(types.String("name")).Equal(types.String("X-Men")) == types.False {
		t.Fatalf("got field 'name' %v, wanted X-Men", obj.Get(types.String("name")))
	}
}

func TestDynamicProto_Input(t *testing.T) {
	b, err := ioutil.ReadFile("testdata/team.fds")
	if err != nil {
		t.Fatalf("ioutil.ReadFile() failed: %v", err)
	}
	var fds descpb.FileDescriptorSet
	if err = proto.Unmarshal(b, &fds); err != nil {
		t.Fatalf("proto.Unmarshal() failed: %v", err)
	}
	files := (&fds).GetFile()
	fileCopy := make([]interface{}, len(files))
	for i := 0; i < len(files); i++ {
		fileCopy[i] = files[i]
	}
	pbFiles, err := protodesc.NewFiles(&fds)
	if err != nil {
		t.Fatalf("protodesc.NewFiles() failed: %v", err)
	}
	desc, err := pbFiles.FindDescriptorByName("cel.testdata.Mutant")
	if err != nil {
		t.Fatalf("pbFiles.FindDescriptorByName() could not find Mutant: %v", err)
	}
	msgDesc, ok := desc.(protoreflect.MessageDescriptor)
	if !ok {
		t.Fatalf("desc not convertible to MessageDescriptor: %T", desc)
	}
	wolverine := dynamicpb.NewMessage(msgDesc)
	wolverine.ProtoReflect().Set(msgDesc.Fields().ByName("name"), protoreflect.ValueOfString("Wolverine"))
	e, err := NewEnv(
		// The following is identical to registering the FileDescriptorSet;
		// however, it tests a different code path which aggregates individual
		// FileDescriptorProto values together.
		TypeDescs(fileCopy...),
		Declarations(decls.NewVar("mutant", decls.NewObjectType("cel.testdata.Mutant"))),
	)
	if err != nil {
		t.Fatalf("NewEnv() failed: %v", err)
	}
	src := `has(mutant.name) && mutant.name == 'Wolverine'`
	ast, iss := e.Compile(src)
	if iss.Err() != nil {
		t.Fatalf("env.Compile(%s) failed: %v", src, iss.Err())
	}
	prg, err := e.Program(ast, EvalOptions(OptOptimize))
	if err != nil {
		t.Fatalf("env.Program() failed: %v", err)
	}
	out, _, err := prg.Eval(map[string]interface{}{
		"mutant": wolverine,
	})
	if err != nil {
		t.Fatalf("program.Eval() failed: %v", err)
	}
	obj, ok := out.(types.Bool)
	if !ok {
		t.Fatalf("unable to convert output to object: %v", out)
	}
	if obj != types.True {
		t.Errorf("got %v, wanted true", out)
	}
}

func TestGlobalVars(t *testing.T) {
	mapStrDyn := decls.NewMapType(decls.String, decls.Dyn)
	e, _ := NewEnv(
		Declarations(
			decls.NewVar("attrs", mapStrDyn),
			decls.NewVar("default", decls.Dyn),
			decls.NewFunction(
				"get",
				decls.NewInstanceOverload(
					"get_map",
					[]*exprpb.Type{mapStrDyn, decls.String, decls.Dyn},
					decls.Dyn))))
	ast, _ := e.Compile(`attrs.get("first", attrs.get("second", default))`)

	// Create the program.
	funcs := Functions(
		&functions.Overload{
			Operator: "get",
			Function: func(args ...ref.Val) ref.Val {
				if len(args) != 3 {
					return types.NewErr("invalid arguments to 'get'")
				}
				attrs, ok := args[0].(traits.Mapper)
				if !ok {
					return types.NewErr(
						"invalid operand of type '%v' to obj.get(key, def)",
						args[0].Type())
				}
				key, ok := args[1].(types.String)
				if !ok {
					return types.NewErr(
						"invalid key of type '%v' to obj.get(key, def)",
						args[1].Type())
				}
				defVal := args[2]
				if attrs.Contains(key) == types.True {
					return attrs.Get(key)
				}
				return defVal
			}})

	// Global variables can be configured as a ProgramOption and optionally overridden on Eval.
	prg, _ := e.Program(ast, funcs, Globals(map[string]interface{}{
		"default": "third",
	}))

	t.Run("global_default", func(t *testing.T) {
		vars := map[string]interface{}{
			"attrs": map[string]interface{}{}}
		out, _, _ := prg.Eval(vars)
		if out.Equal(types.String("third")) != types.True {
			t.Errorf("got '%v', expected 'third'.", out.Value())
		}
	})

	t.Run("attrs_alt", func(t *testing.T) {
		vars := map[string]interface{}{
			"attrs": map[string]interface{}{"second": "yep"}}
		out, _, _ := prg.Eval(vars)
		if out.Equal(types.String("yep")) != types.True {
			t.Errorf("got '%v', expected 'yep'.", out.Value())
		}
	})

	t.Run("local_default", func(t *testing.T) {
		vars := map[string]interface{}{
			"attrs":   map[string]interface{}{},
			"default": "fourth"}
		out, _, _ := prg.Eval(vars)
		if out.Equal(types.String("fourth")) != types.True {
			t.Errorf("got '%v', expected 'fourth'.", out.Value())
		}
	})
}

func TestClearMacros(t *testing.T) {
	e, err := NewEnv(ClearMacros())
	if err != nil {
		t.Fatalf("NewEnv(ClearMacros()) failed: %v", err)
	}
	ast, iss := e.Parse("has(a.b)")
	if iss.Err() != nil {
		t.Fatalf("Parse(`has(a.b)`) failed: %v", iss.Err())
	}
	pe, err := AstToParsedExpr(ast)
	if err != nil {
		t.Fatalf("AstToParsedExpr(ast) failed: %v", err)
	}
	want := &exprpb.Expr{
		Id: 1,
		ExprKind: &exprpb.Expr_CallExpr{
			CallExpr: &exprpb.Expr_Call{
				Function: "has",
				Args: []*exprpb.Expr{
					{
						Id: 3,
						ExprKind: &exprpb.Expr_SelectExpr{
							SelectExpr: &exprpb.Expr_Select{
								Operand: &exprpb.Expr{
									Id: 2,
									ExprKind: &exprpb.Expr_IdentExpr{
										IdentExpr: &exprpb.Expr_Ident{Name: "a"},
									},
								},
								Field: "b",
							},
						},
					},
				},
			},
		},
	}
	if !proto.Equal(pe.GetExpr(), want) {
		t.Errorf("Parse() produced AST with macro replacement rather than call. got %v, wanted %v", pe, want)
	}
}

func TestCustomMacro(t *testing.T) {
	joinMacro := parser.NewReceiverMacro("join", 1,
		func(eh parser.ExprHelper,
			target *exprpb.Expr,
			args []*exprpb.Expr) (*exprpb.Expr, *common.Error) {
			delim := args[0]
			iterIdent := eh.Ident("__iter__")
			accuIdent := eh.Ident("__result__")
			init := eh.LiteralString("")
			condition := eh.LiteralBool(true)
			step := eh.GlobalCall(
				operators.Conditional,
				eh.GlobalCall(operators.Greater,
					eh.ReceiverCall("size", accuIdent),
					eh.LiteralInt(0)),
				eh.GlobalCall(
					operators.Add,
					eh.GlobalCall(
						operators.Add,
						accuIdent,
						delim),
					iterIdent),
				iterIdent)
			return eh.Fold(
				"__iter__",
				target,
				"__result__",
				init,
				condition,
				step,
				accuIdent), nil
		})
	e, _ := NewEnv(
		Macros(joinMacro),
	)
	ast, iss := e.Compile(`['hello', 'cel', 'friend'].join(',')`)
	if iss.Err() != nil {
		t.Fatal(iss.Err())
	}
	prg, err := e.Program(ast, EvalOptions(OptExhaustiveEval))
	if err != nil {
		t.Fatalf("program creation error: %s\n", err)
	}
	out, _, err := prg.Eval(NoVars())
	if err != nil {
		t.Fatal(err)
	}
	if out.Equal(types.String("hello,cel,friend")) != types.True {
		t.Errorf("got %v, wanted 'hello,cel,friend'", out)
	}
}

func TestAstIsChecked(t *testing.T) {
	e, err := NewEnv()
	if err != nil {
		t.Fatalf("NewEnv() failed: %v", err)
	}
	ast, iss := e.Compile("true")
	if iss.Err() != nil {
		t.Fatalf("e.Compile('true') failed: %v", iss.Err())
	}
	if !ast.IsChecked() {
		t.Error("got ast.IsChecked() 'false', wanted 'true'.")
	}
	ce, err := AstToCheckedExpr(ast)
	if err != nil {
		t.Fatalf("AstToCheckedExpr(%v) failed: %v", ast, err)
	}
	ast2 := CheckedExprToAst(ce)
	if !ast2.IsChecked() {
		t.Error("got ast2.IsChecked() 'false', wanted 'true'")
	}
	if !proto.Equal(ast.Expr(), ast2.Expr()) {
		t.Errorf("AST exprs did not roundtrip properly: ast1: %v, ast2: %v", ast, ast2)
	}
}

func TestEvalOptions(t *testing.T) {
	e, _ := NewEnv(
		Declarations(
			decls.NewVar("k", decls.String),
			decls.NewVar("v", decls.Bool)))
	ast, _ := e.Compile(`{k: true}[k] || v != false`)

	prg, err := e.Program(ast, EvalOptions(OptExhaustiveEval))
	if err != nil {
		t.Fatalf("program creation error: %s\n", err)
	}
	out, details, err := prg.Eval(
		map[string]interface{}{
			"k": "key",
			"v": true})
	if err != nil {
		t.Fatalf("runtime error: %s\n", err)
	}
	if out != types.True {
		t.Errorf("got '%v', expected 'true'", out.Value())
	}

	// Test to see whether 'v != false' was resolved to a value.
	// With short-circuiting it normally wouldn't be.
	s := details.State()
	lhsVal, found := s.Value(ast.Expr().GetCallExpr().GetArgs()[0].Id)
	if !found {
		t.Error("got not found, wanted evaluation of left hand side expression.")
		return
	}
	if lhsVal != types.True {
		t.Errorf("got '%v', expected 'true'", lhsVal)
	}
	rhsVal, found := s.Value(ast.Expr().GetCallExpr().GetArgs()[1].Id)
	if !found {
		t.Error("got not found, wanted evaluation of right hand side expression.")
		return
	}
	if rhsVal != types.True {
		t.Errorf("got '%v', expected 'true'", rhsVal)
	}
}

func TestContextEval(t *testing.T) {
	env, err := NewEnv(
		Declarations(
			decls.NewVar("items", decls.NewListType(decls.Int)),
		),
	)
	if err != nil {
		t.Fatalf("NewEnv() failed: %v", err)
	}
	ast, iss := env.Compile("items.map(i, i * 2).filter(i, i >= 50).size()")
	if iss.Err() != nil {
		t.Fatalf("env.Compile(expr) failed: %v", iss.Err())
	}
	prg, err := env.Program(ast, EvalOptions(OptOptimize|OptTrackState), InterruptCheckFrequency(100))
	if err != nil {
		t.Fatalf("env.Program() failed: %v", err)
	}

	ctx := context.TODO()
	items := make([]int64, 1000)
	for i := int64(0); i < 1000; i++ {
		items[i] = i
	}
	out, _, err := prg.ContextEval(ctx, map[string]interface{}{"items": items})
	if err != nil {
		t.Fatalf("prg.ContextEval() failed: %v", err)
	}
	if out != types.Int(975) {
		t.Errorf("prg.ContextEval() got %v, wanted 75", out)
	}

	evalCtx, cancel := context.WithTimeout(ctx, time.Microsecond)
	defer cancel()

	out, _, err = prg.ContextEval(evalCtx, map[string]interface{}{"items": items})
	if err == nil {
		t.Errorf("Got result %v, wanted timeout error", out)
	}
	if err != nil && err.Error() != "operation interrupted" {
		t.Errorf("Got %v, wanted operation interrupted error", err)
	}
}

func BenchmarkContextEval(b *testing.B) {
	env, err := NewEnv(
		Declarations(
			decls.NewVar("items", decls.NewListType(decls.Int)),
		),
	)
	if err != nil {
		b.Fatalf("NewEnv() failed: %v", err)
	}
	ast, iss := env.Compile("items.map(i, i * 2).filter(i, i >= 50).size()")
	if iss.Err() != nil {
		b.Fatalf("env.Compile(expr) failed: %v", iss.Err())
	}
	prg, err := env.Program(ast, EvalOptions(OptOptimize), InterruptCheckFrequency(200))
	if err != nil {
		b.Fatalf("env.Program() failed: %v", err)
	}

	ctx := context.TODO()
	items := make([]int64, 100)
	for i := int64(0); i < 100; i++ {
		items[i] = i
	}
	for i := 0; i < b.N; i++ {
		out, _, err := prg.ContextEval(ctx, map[string]interface{}{"items": items})
		if err != nil {
			b.Fatalf("prg.ContextEval() failed: %v", err)
		}
		if out != types.Int(75) {
			b.Errorf("prg.ContextEval() got %v, wanted 75", out)
		}
	}
}

func TestEvalRecover(t *testing.T) {
	e, err := NewEnv(
		Declarations(
			decls.NewFunction("panic",
				decls.NewOverload("panic", []*exprpb.Type{}, decls.Bool)),
		))
	if err != nil {
		t.Fatalf("NewEnv() failed: %v", err)
	}
	funcs := Functions(&functions.Overload{
		Operator: "panic",
		Function: func(args ...ref.Val) ref.Val {
			panic("watch me recover")
		},
	})
	// Test standard evaluation.
	pAst, _ := e.Parse("panic()")
	prgm, _ := e.Program(pAst, funcs)
	_, _, err = prgm.Eval(map[string]interface{}{})
	if err.Error() != "internal error: watch me recover" {
		t.Errorf("got '%v', wanted 'internal error: watch me recover'", err)
	}
	// Test the factory-based evaluation.
	prgm, _ = e.Program(pAst, funcs, EvalOptions(OptTrackState))
	_, _, err = prgm.Eval(map[string]interface{}{})
	if err.Error() != "internal error: watch me recover" {
		t.Errorf("got '%v', wanted 'internal error: watch me recover'", err)
	}
}

func TestResidualAst(t *testing.T) {
	e, _ := NewEnv(
		Declarations(
			decls.NewVar("x", decls.Int),
			decls.NewVar("y", decls.Int),
		),
	)
	unkVars := e.UnknownVars()
	ast, _ := e.Parse(`x < 10 && (y == 0 || 'hello' != 'goodbye')`)
	prg, _ := e.Program(ast,
		EvalOptions(OptTrackState, OptPartialEval),
	)
	out, det, err := prg.Eval(unkVars)
	if !types.IsUnknown(out) {
		t.Fatalf("got %v, expected unknown", out)
	}
	if err != nil {
		t.Fatal(err)
	}
	residual, err := e.ResidualAst(ast, det)
	if err != nil {
		t.Fatal(err)
	}
	expr, err := AstToString(residual)
	if err != nil {
		t.Fatal(err)
	}
	if expr != "x < 10" {
		t.Errorf("got expr: %s, wanted x < 10", expr)
	}
}

func TestResidualAst_Complex(t *testing.T) {
	e, _ := NewEnv(
		Declarations(
			decls.NewVar("resource.name", decls.String),
			decls.NewVar("request.time", decls.Timestamp),
			decls.NewVar("request.auth.claims",
				decls.NewMapType(decls.String, decls.String)),
		),
	)
	unkVars, _ := PartialVars(
		map[string]interface{}{
			"resource.name": "bucket/my-bucket/objects/private",
			"request.auth.claims": map[string]string{
				"email_verified": "true",
			},
		},
		AttributePattern("request.auth.claims").QualString("email"),
	)
	ast, iss := e.Compile(
		`resource.name.startsWith("bucket/my-bucket") &&
		 bool(request.auth.claims.email_verified) == true &&
		 request.auth.claims.email == "wiley@acme.co"`)
	if iss.Err() != nil {
		t.Fatal(iss.Err())
	}
	prg, _ := e.Program(ast,
		EvalOptions(OptTrackState, OptPartialEval),
	)
	out, det, err := prg.Eval(unkVars)
	if !types.IsUnknown(out) {
		t.Fatalf("got %v, expected unknown", out)
	}
	if err != nil {
		t.Fatal(err)
	}
	residual, err := e.ResidualAst(ast, det)
	if err != nil {
		t.Fatal(err)
	}
	expr, err := AstToString(residual)
	if err != nil {
		t.Fatal(err)
	}
	if expr != `request.auth.claims.email == "wiley@acme.co"` {
		t.Errorf("got expr: %s, wanted request.auth.claims.email == \"wiley@acme.co\"", expr)
	}
}

func Benchmark_EvalOptions(b *testing.B) {
	e, _ := NewEnv(
		Declarations(
			decls.NewVar("ai", decls.Int),
			decls.NewVar("ar", decls.NewMapType(decls.String, decls.String)),
		),
	)
	ast, _ := e.Compile("ai == 20 || ar['foo'] == 'bar'")
	vars := map[string]interface{}{
		"ai": 2,
		"ar": map[string]string{
			"foo": "bar",
		},
	}

	opts := map[string]EvalOption{
		"track-state":     OptTrackState,
		"exhaustive-eval": OptExhaustiveEval,
		"optimize":        OptOptimize,
	}
	for k, opt := range opts {
		b.Run(k, func(bb *testing.B) {
			prg, _ := e.Program(ast, EvalOptions(opt))
			b.ResetTimer()
			b.ReportAllocs()
			for i := 0; i < bb.N; i++ {
				_, _, err := prg.Eval(vars)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func TestEnvExtension(t *testing.T) {
	e, _ := NewEnv(
		Container("google.api.expr.v1alpha1"),
		Types(&exprpb.Expr{}),
		Declarations(
			decls.NewVar("expr",
				decls.NewObjectType("google.api.expr.v1alpha1.Expr")),
		),
	)
	e2, _ := e.Extend(
		CustomTypeAdapter(types.DefaultTypeAdapter),
		Types(&proto3pb.TestAllTypes{}),
	)
	if e == e2 {
		t.Error("got object equality, wanted separate objects")
	}
	if e.TypeAdapter() == e2.TypeAdapter() {
		t.Error("got the same type adapter, wanted isolated instances.")
	}
	if e.TypeProvider() == e2.TypeProvider() {
		t.Error("got the same type provider, wanted isolated instances.")
	}
	e3, _ := e2.Extend()
	if e2.TypeAdapter() != e3.TypeAdapter() {
		t.Error("got different type adapters, wanted immutable adapter reference")
	}
	if e2.TypeProvider() == e3.TypeProvider() {
		t.Error("got the same type provider, wanted isolated instances.")
	}
}

func TestEnvExtensionIsolation(t *testing.T) {
	baseEnv, err := NewEnv(
		Container("google.expr"),
		Declarations(
			decls.NewVar("age", decls.Int),
			decls.NewVar("gender", decls.String),
			decls.NewVar("country", decls.String),
		),
	)
	if err != nil {
		t.Fatal(err)
	}
	env1, err := baseEnv.Extend(
		Types(&proto2pb.TestAllTypes{}),
		Declarations(decls.NewVar("name", decls.String)))
	if err != nil {
		t.Fatal(err)
	}
	env2, err := baseEnv.Extend(
		Types(&proto3pb.TestAllTypes{}),
		Declarations(decls.NewVar("group", decls.String)))
	if err != nil {
		t.Fatal(err)
	}
	_, issues := env2.Compile(`size(group) > 10
		&& !has(proto3.test.TestAllTypes{}.single_int32)`)
	if issues.Err() != nil {
		t.Fatal(issues.Err())
	}
	_, issues = env2.Compile(`size(name) > 10`)
	if issues.Err() == nil {
		t.Fatal("env2 contains 'name', but should not")
	}
	_, issues = env2.Compile(`!has(proto2.test.TestAllTypes{}.single_int32)`)
	if issues.Err() == nil {
		t.Fatal("env2 contains 'proto2.test.TestAllTypes', but should not")
	}

	_, issues = env1.Compile(`size(name) > 10
		&& !has(proto2.test.TestAllTypes{}.single_int32)`)
	if issues.Err() != nil {
		t.Fatal(issues.Err())
	}
	_, issues = env1.Compile("size(group) > 10")
	if issues.Err() == nil {
		t.Fatal("env1 contains 'group', but should not")
	}
	_, issues = env1.Compile(`!has(proto3.test.TestAllTypes{}.single_int32)`)
	if issues.Err() == nil {
		t.Fatal("env1 contains 'proto3.test.TestAllTypes', but should not")
	}
}

func TestParseError(t *testing.T) {
	e, err := NewEnv()
	if err != nil {
		t.Fatalf("NewEnv() failed: %v", err)
	}
	_, iss := e.Parse("invalid & logical_and")
	if iss.Err() == nil {
		t.Fatal("e.Parse('invalid & logical_and') did not error")
	}
}

func TestParseWithMacroTracking(t *testing.T) {
	e, err := NewEnv(EnableMacroCallTracking())
	if err != nil {
		t.Fatalf("NewEnv(EnableMacroCallTracking()) failed: %v", err)
	}
	ast, iss := e.Parse("has(a.b) && a.b.exists(c, c < 10)")
	if iss.Err() != nil {
		t.Fatalf("e.Parse() failed: %v", iss.Err())
	}
	pe, err := AstToParsedExpr(ast)
	if err != nil {
		t.Fatalf("AstToParsedExpr(%v) failed: %v", ast, err)
	}
	macroCalls := pe.GetSourceInfo().GetMacroCalls()
	if len(macroCalls) != 2 {
		t.Errorf("got %d macro calls, wanted 2", len(macroCalls))
	}
	callsFound := map[string]bool{"has": false, "exists": false}
	for _, expr := range macroCalls {
		f := expr.GetCallExpr().GetFunction()
		_, found := callsFound[f]
		if !found {
			t.Errorf("Unexpected macro call: %v", expr)
		}
		callsFound[f] = true
	}
	callsWanted := map[string]bool{"has": true, "exists": true}
	if !reflect.DeepEqual(callsFound, callsWanted) {
		t.Errorf("Tracked calls %v, but wanted %v", callsFound, callsWanted)
	}
}

func TestParseAndCheckConcurrently(t *testing.T) {
	e, err := NewEnv(
		Container("google.api.expr.v1alpha1"),
		Types(&exprpb.Expr{}),
		Declarations(
			decls.NewVar("expr",
				decls.NewObjectType("google.api.expr.v1alpha1.Expr")),
		),
	)
	if err != nil {
		t.Fatalf("NewEnv() failed: %v", err)
	}

	parseAndCheck := func(expr string) {
		_, iss := e.Compile(expr)
		if iss.Err() != nil {
			t.Fatalf("e.Compile('%s') failed: %v", expr, iss.Err())
		}
	}

	const concurrency = 10
	wgDone := sync.WaitGroup{}
	wgDone.Add(concurrency)

	for i := 0; i < concurrency; i++ {
		go func(i int) {
			defer wgDone.Done()
			parseAndCheck(fmt.Sprintf("expr.id + %d", i))
		}(i)
	}
	wgDone.Wait()
}

func TestCustomInterpreterDecorator(t *testing.T) {
	var lastInstruction interpreter.Interpretable
	optimizeArith := func(i interpreter.Interpretable) (interpreter.Interpretable, error) {
		lastInstruction = i
		// Only optimize the instruction if it is a call.
		call, ok := i.(interpreter.InterpretableCall)
		if !ok {
			return i, nil
		}
		// Only optimize the math functions when they have constant arguments.
		switch call.Function() {
		case operators.Add,
			operators.Subtract,
			operators.Multiply,
			operators.Divide:
			// These are all binary operators so they should have to arguments
			args := call.Args()
			_, lhsIsConst := args[0].(interpreter.InterpretableConst)
			_, rhsIsConst := args[1].(interpreter.InterpretableConst)
			// When the values are constant then the call can be evaluated with
			// an empty activation and the value returns as a constant.
			if !lhsIsConst || !rhsIsConst {
				return i, nil
			}
			val := call.Eval(interpreter.EmptyActivation())
			if types.IsError(val) {
				return nil, val.(*types.Err)
			}
			return interpreter.NewConstValue(call.ID(), val), nil
		default:
			return i, nil
		}
	}

	env, _ := NewEnv(Declarations(decls.NewVar("foo", decls.Int)))
	ast, _ := env.Compile(`foo == -1 + 2 * 3 / 3`)
	_, err := env.Program(ast,
		EvalOptions(OptPartialEval),
		CustomDecorator(optimizeArith))
	if err != nil {
		t.Fatal(err)
	}
	call, ok := lastInstruction.(interpreter.InterpretableCall)
	if !ok {
		t.Errorf("got %v, expected call", lastInstruction)
	}
	args := call.Args()
	lhs := args[0]
	lastAttr, ok := lhs.(interpreter.InterpretableAttribute)
	if !ok {
		t.Errorf("got %v, wanted attribute", lhs)
	}
	absAttr := lastAttr.Attr().(interpreter.NamespacedAttribute)
	varNames := absAttr.CandidateVariableNames()
	if len(varNames) != 1 || varNames[0] != "foo" {
		t.Errorf("got variables %v, wanted foo", varNames)
	}
	rhs := args[1]
	lastConst, ok := rhs.(interpreter.InterpretableConst)
	if !ok {
		t.Errorf("got %v, wanted constant", rhs)
	}
	// This is the last number produced by the optimization.
	if lastConst.Value().Equal(types.IntOne) == types.False {
		t.Errorf("got %v as the last observed constant, wanted 1", lastConst)
	}
}

// TODO: ideally testCostEstimator and testRuntimeCostEstimator would be shared in a test fixtures package
type testCostEstimator struct {
	hints map[string]int64
}

func (tc testCostEstimator) EstimateSize(element checker.AstNode) *checker.SizeEstimate {
	if l, ok := tc.hints[strings.Join(element.Path(), ".")]; ok {
		return &checker.SizeEstimate{Min: 0, Max: uint64(l)}
	}
	return nil
}

func (tc testCostEstimator) EstimateCallCost(function, overloadID string, target *checker.AstNode, args []checker.AstNode) *checker.CallEstimate {
	switch overloadID {
	case overloads.TimestampToYear:
		return &checker.CallEstimate{CostEstimate: checker.CostEstimate{Min: 7, Max: 7}}
	}
	return nil
}

type testRuntimeCostEstimator struct {
}

var timeToYearCost uint64 = 7

func (e testRuntimeCostEstimator) CallCost(function, overloadID string, args []ref.Val, result ref.Val) *uint64 {
	argsSize := make([]uint64, len(args))
	for i, arg := range args {
		reflectV := reflect.ValueOf(arg.Value())
		switch reflectV.Kind() {
		// Note that the CEL bytes type is implemented with Go byte slices, therefore also supported by the following
		// code.
		case reflect.String, reflect.Array, reflect.Slice, reflect.Map:
			argsSize[i] = uint64(reflectV.Len())
		default:
			argsSize[i] = 1
		}
	}

	switch overloadID {
	case overloads.TimestampToYear:
		return &timeToYearCost
	default:
		return nil
	}
}

// TestEstimateCostAndRuntimeCost sanity checks that the cost systems are usable from the program API.
func TestEstimateCostAndRuntimeCost(t *testing.T) {
	intList := decls.NewListType(decls.Int)
	zeroCost := checker.CostEstimate{}
	cases := []struct {
		name  string
		expr  string
		decls []*exprpb.Decl
		hints map[string]int64
		want  checker.CostEstimate
		in    interface{}
	}{
		{
			name: "const",
			expr: `"Hello World!"`,
			want: zeroCost,
			in:   map[string]interface{}{},
		},
		{
			name:  "identity",
			expr:  `input`,
			decls: []*exprpb.Decl{decls.NewVar("input", intList)},
			want:  checker.CostEstimate{Min: 1, Max: 1},
			in:    map[string]interface{}{"input": []int{1, 2}},
		},
		{
			name: "str concat",
			expr: `"abcdefg".contains(str1 + str2)`,
			decls: []*exprpb.Decl{
				decls.NewVar("str1", decls.String),
				decls.NewVar("str2", decls.String),
			},
			hints: map[string]int64{"str1": 10, "str2": 10},
			want:  checker.CostEstimate{Min: 2, Max: 6},
			in:    map[string]interface{}{"str1": "val1111111", "str2": "val2222222"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.hints == nil {
				tc.hints = map[string]int64{}
			}
			e, err := NewEnv(
				Declarations(tc.decls...),
				Types(&proto3pb.TestAllTypes{}))
			if err != nil {
				t.Fatalf("NewEnv(opts ...EnvOption) failed to create an environment: %s\n", err)
			}
			ast, iss := e.Compile(tc.expr)
			if iss.Err() != nil {
				t.Fatal(iss.Err())
			}
			est, err := e.EstimateCost(ast, testCostEstimator{hints: tc.hints})
			if err != nil {
				t.Fatalf("Env.EstimateCost(ast *Ast, estimator checker.CostEstimator) failed to estimate cost: %s\n", err)
			}
			if est.Min != tc.want.Min || est.Max != tc.want.Max {
				t.Fatalf("Env.EstimateCost(ast *Ast, estimator checker.CostEstimator) failed to return the right cost interval. Got [%v, %v], wanted [%v, %v]",
					est.Min, est.Max, tc.want.Min, tc.want.Max)
			}

			checkedAst, iss := e.Check(ast)
			if iss.Err() != nil {
				t.Fatalf(`Env.Check(ast *Ast) failed to check expression: %v`, iss.Err())
			}
			// Evaluate expression.
			program, err := e.Program(checkedAst, CostTracking(testRuntimeCostEstimator{}))
			if err != nil {
				t.Fatalf(`Env.Program(ast *Ast, opts ...ProgramOption) failed to construct program: %v`, err)
			}
			_, details, err := program.Eval(tc.in)
			if err != nil {
				t.Fatalf(`Program.Eval(vars interface{}) failed to evaluate expression: %v`, err)
			}
			actualCost := details.ActualCost()
			if actualCost == nil {
				t.Errorf(`EvalDetails.ActualCost() got nil for "%s" cost, wanted %d`, tc.expr, actualCost)
			}

			if est.Min > *actualCost || est.Max < *actualCost {
				t.Errorf("EvalDetails.ActualCost() failed to return a runtime cost %d is the range of estimate cost [%d, %d]", *actualCost,
					est.Min, est.Max)
			}
		})
	}
}

func TestResidualAst_AttributeQualifiers(t *testing.T) {
	e, _ := NewEnv(
		Declarations(
			decls.NewVar("x", decls.NewMapType(decls.String, decls.Dyn)),
			decls.NewVar("y", decls.NewListType(decls.Int)),
			decls.NewVar("u", decls.Int),
		),
	)
	ast, _ := e.Parse(`x.abc == u && x["abc"] == u && x[x.string] == u && y[0] == u && y[x.zero] == u && (true ? x : y).abc == u && (false ? y : x).abc == u`)
	prg, _ := e.Program(ast,
		EvalOptions(OptTrackState, OptPartialEval),
	)
	vars, _ := PartialVars(map[string]interface{}{
		"x": map[string]interface{}{
			"zero":   0,
			"abc":    123,
			"string": "abc",
		},
		"y": []int{123},
	}, AttributePattern("u"))
	out, det, err := prg.ContextEval(context.TODO(), vars)
	if !types.IsUnknown(out) {
		t.Fatalf("got %v, expected unknown", out)
	}
	if err != nil {
		t.Fatal(err)
	}
	residual, err := e.ResidualAst(ast, det)
	if err != nil {
		t.Fatal(err)
	}
	expr, err := AstToString(residual)
	if err != nil {
		t.Fatal(err)
	}
	const want = "123 == u && 123 == u && 123 == u && 123 == u && 123 == u && 123 == u && 123 == u"
	if expr != want {
		t.Errorf("got expr: %s, wanted %s", expr, want)
	}
}

func TestResidualAst_Modified(t *testing.T) {
	e, _ := NewEnv(
		Declarations(
			decls.NewVar("x", decls.NewMapType(decls.String, decls.Int)),
			decls.NewVar("y", decls.Int),
		),
	)
	ast, _ := e.Parse("x == y")
	prg, _ := e.Program(ast,
		EvalOptions(OptTrackState, OptPartialEval),
	)
	for _, x := range []int{123, 456} {
		vars, _ := PartialVars(map[string]interface{}{
			"x": x,
		}, AttributePattern("y"))
		out, det, err := prg.Eval(vars)
		if !types.IsUnknown(out) {
			t.Fatalf("got %v, expected unknown", out)
		}
		if err != nil {
			t.Fatal(err)
		}
		residual, err := e.ResidualAst(ast, det)
		if err != nil {
			t.Fatal(err)
		}
		orig, err := AstToString(ast)
		if err != nil {
			t.Fatal(err)
		}
		if orig != "x == y" {
			t.Errorf("parsed ast: got expr: %s, wanted x == y", orig)
		}
		expr, err := AstToString(residual)
		if err != nil {
			t.Fatal(err)
		}
		want := fmt.Sprintf("%d == y", x)
		if expr != want {
			t.Errorf("residual ast: got expr: %s, wanted %s", expr, want)
		}
	}
}

func TestDeclareContextProto(t *testing.T) {
	descriptor := new(proto3pb.TestAllTypes).ProtoReflect().Descriptor()
	option := DeclareContextProto(descriptor)
	env, err := NewEnv(option)
	if err != nil {
		t.Fatalf("NewEnv(DeclareContextProto(%v)) failed: %s", descriptor, err)
	}
	expression := `single_int64 == 1 && single_double == 1.0 && single_bool == true && single_string == '' && single_nested_message == google.expr.proto3.test.TestAllTypes.NestedMessage{}
	&& single_nested_enum == google.expr.proto3.test.TestAllTypes.NestedEnum.FOO && single_duration == duration('5s') && single_timestamp == timestamp('1972-01-01T10:00:20.021-05:00')
	&& single_any == google.protobuf.Any{} && repeated_int32 == [1,2] && map_string_string == {'': ''} && map_int64_nested_type == {0 : google.expr.proto3.test.NestedTestAllTypes{}}`
	_, iss := env.Compile(expression)
	if iss.Err() != nil {
		t.Fatalf("env.Compile(%s) failed: %s", expression, iss.Err())
	}
}

func TestRegexOptimizer(t *testing.T) {
	var stringTests = []struct {
		expr          string
		optimizeRegex bool
		progErr       string
		err           string
		parseOnly     bool
	}{
		{expr: `"123 abc 456".matches('[0-9]*')`},
		{expr: `"123 abc 456".matches('[0-9]' + '*')`},
		{expr: `"123 abc 456".matches('[0-9]*')`, optimizeRegex: true},
		{expr: `"123 abc 456".matches('[0-9]' + '*')`, optimizeRegex: true},
		{
			// Verify that a regex compilation error for an optimized regex is
			// reported at program creation time.
			expr: `"123 abc 456".matches(')[0-9]*')`, optimizeRegex: true,
			progErr: "error parsing regexp: unexpected ): `)[0-9]*`",
		},
		{
			expr: `"123 abc 456".matches(')[0-9]*')`,
			err:  "error parsing regexp: unexpected ): `)[0-9]*`",
		},
	}

	env, err := NewEnv()
	if err != nil {
		t.Fatal(err)
	}
	for i, tst := range stringTests {
		tc := tst
		t.Run(fmt.Sprintf("[%d]", i), func(tt *testing.T) {
			var asts []*Ast
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
				var opts []ProgramOption
				if tc.optimizeRegex {
					opts = append(opts, EvalOptions(OptOptimize))
				}
				prg, progErr := env.Program(ast, opts...)
				if tc.progErr != "" {
					if progErr == nil {
						tt.Fatalf("wanted error %s for expr: %s", tc.progErr, tc.expr)
					}
					if tc.progErr != progErr.Error() {
						tt.Errorf("got error %v, wanted error %s for expr: %s", progErr, tc.progErr, tc.expr)
					}
					continue
				} else if progErr != nil {
					tt.Fatal(progErr)
				}
				out, _, err := prg.Eval(NoVars())
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
