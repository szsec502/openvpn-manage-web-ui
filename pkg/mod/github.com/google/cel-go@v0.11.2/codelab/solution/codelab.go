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

// This file contains code that demonstrates common CEL features.
// This code is intended for use with the CEL Codelab: http://go/cel-codelab-go
package main

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/golang/glog"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/encoding/prototext"
	"google.golang.org/protobuf/proto"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/checker/decls"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"github.com/google/cel-go/common/types/traits"
	"github.com/google/cel-go/interpreter/functions"

	exprpb "google.golang.org/genproto/googleapis/api/expr/v1alpha1"
	rpcpb "google.golang.org/genproto/googleapis/rpc/context/attribute_context"
	structpb "google.golang.org/protobuf/types/known/structpb"
	tpb "google.golang.org/protobuf/types/known/timestamppb"
)

func main() {
	exercise1()
	exercise2()
	exercise3()
	exercise4()
	exercise5()
	exercise6()
	exercise7()
	exercise8()
}

// exercise1 evaluates a simple literal expression: "Hello, World!"
//
// Compile, eval, profit!
func exercise1() {
	fmt.Println("=== Exercise 1: Hello World ===\n")
	// Create the standard environment.
	env, err := cel.NewEnv()
	if err != nil {
		glog.Exitf("env error: %v", err)
	}
	// Check that the expression compiles and returns a String.
	ast, iss := env.Parse(`"Hello, World!"`)
	// Report syntactic errors, if present.
	if iss.Err() != nil {
		glog.Exit(iss.Err())
	}
	// Type-check the expression for correctness.
	checked, iss := env.Check(ast)
	// Report semantic errors, if present.
	if iss.Err() != nil {
		glog.Exit(iss.Err())
	}
	// Check the result type is a string.
	if !proto.Equal(checked.ResultType(), decls.String) {
		glog.Exitf(
			"Got %v, wanted %v result type",
			checked.ResultType(), decls.String)
	}
	// Plan the program.
	program, err := env.Program(checked)
	if err != nil {
		glog.Exitf("program error: %v", err)
	}
	// Evaluate the program without any additional arguments.
	eval(program, cel.NoVars())
	fmt.Println()
}

// exercise2 shows how to declare and use variables in expressions.
//
// Given a `request` of type `google.rpc.context.AttributeContext.Request`
// determine whether a specific auth claim is set.
func exercise2() {
	fmt.Println("=== Exercise 2: Variables ===\n")
	// Construct a standard environment that accepts 'request' as input and uses
	// the google.rpc.context.AttributeContext.Request type.
	env, err := cel.NewEnv(
		cel.Types(&rpcpb.AttributeContext_Request{}),
		cel.Declarations(
			decls.NewVar("request",
				decls.NewObjectType("google.rpc.context.AttributeContext.Request")),
		),
	)
	if err != nil {
		glog.Exit(err)
	}
	ast := compile(env, `request.auth.claims.group == 'admin'`, decls.Bool)
	program, _ := env.Program(ast)

	// Evaluate a request object that sets the proper group claim.
	// Output: true
	claims := map[string]string{"group": "admin"}
	eval(program, request(auth("user:me@acme.co", claims), time.Now()))
	fmt.Println()
}

// exercise3 demonstrates how CEL's commutative logical operators work.
//
// Construct an expression which checks whether the `request.auth.claims.group`
// value is equal to `admin` or the `request.auth.principal` is
// `user:me@acme.co` and the `request.time` is during work hours (9:00 - 17:00)
//
// Evaluate the expression once with a request containing no claims but which
// sets the appropriate principal and occurs at 12:00 hours. Then evaluate the
// request a second time at midnight. Observe the difference in output.
func exercise3() {
	fmt.Println("=== Exercise 3: Logical AND/OR ===\n")
	env, _ := cel.NewEnv(
		cel.Types(&rpcpb.AttributeContext_Request{}),
		cel.Declarations(
			decls.NewVar("request",
				decls.NewObjectType("google.rpc.context.AttributeContext.Request"),
			),
		),
	)
	ast := compile(env,
		`request.auth.claims.group == 'admin'
				|| request.auth.principal == 'user:me@acme.co'`,
		decls.Bool)
	program, _ := env.Program(ast)

	// Evaluate once with no claims and the proper user.
	// Output: true
	eval(program, request(auth("user:me@acme.co", emptyClaims), time.Now()))

	// Evaluate again with no claims and an unexpected user.
	// Output: error, no such key
	eval(program, request(auth("other:me@acme.co", emptyClaims), time.Now()))

	fmt.Println()
}

// exercise4 demonstrates how to extend CEL with custom functions.
//
// Declare a `contains` member function on map types that returns a boolean
// indicating whether the map contains the key-value pair.
func exercise4() {
	fmt.Println("=== Exercise 4: Customization ===\n")
	// Determine whether an optional claim is set to the proper value. The custom
	// map.contains(key, value) function is used as an alternative to:
	//   key in map && map[key] == value

	// Useful components of the type-signature for 'contains'.
	typeParamA := decls.NewTypeParamType("A")
	typeParamB := decls.NewTypeParamType("B")
	mapAB := decls.NewMapType(typeParamA, typeParamB)

	// Env declaration.
	env, _ := cel.NewEnv(
		cel.Types(&rpcpb.AttributeContext_Request{}),
		cel.Declarations(
			// Declare the request.
			decls.NewVar("request",
				decls.NewObjectType("google.rpc.context.AttributeContext.Request"),
			),
			// Declare the custom contains function.
			decls.NewFunction("contains",
				decls.NewParameterizedInstanceOverload(
					"map_contains_key_value",
					[]*exprpb.Type{mapAB, typeParamA, typeParamB},
					decls.Bool,
					[]string{"A", "B"},
				),
			),
		),
	)
	ast := compile(env,
		`request.auth.claims.contains('group', 'admin')`,
		decls.Bool)

	// Construct the program plan and provide the 'contains' function impl.
	// Output: false
	program, err := env.Program(ast,
		cel.Functions(
			&functions.Overload{
				Operator: "map_contains_key_value",
				Function: mapContainsKeyValue,
			}),
	)
	if err != nil {
		glog.Exit(err)
	}

	eval(program, request(auth("user:me@acme.co", emptyClaims), time.Now()))
	claims := map[string]string{"group": "admin"}
	eval(program, request(auth("user:me@acme.co", claims), time.Now()))
	fmt.Println()
}

// exercise5 covers how to build complex objects as CEL literals.
//
// Given the input `now`, construct a JWT with an expiry of 5 minutes.
func exercise5() {
	fmt.Println("=== Exercise 5: Building JSON ===\n")
	// Note the quoted keys in the CEL map literal. For proto messages the
	// field names are unquoted as they represent well-defined identifiers.
	env, _ := cel.NewEnv(
		cel.Declarations(
			decls.NewVar("now", decls.Timestamp),
		),
	)
	ast := compile(env, `
		{'aud': 'my-project',
		 'exp': now + duration('300s'),
		 'extra_claims': {
		 	'group': 'admin'
		 },
		 'iat': now,
		 'iss': 'auth.acme.com:12350',
		 'nbf': now,
		 'sub': 'serviceAccount:delegate@acme.co'
		 }`,
		decls.NewMapType(decls.String, decls.Dyn))

	program, _ := env.Program(ast)
	out, _, _ := eval(
		program,
		map[string]interface{}{
			"now": &tpb.Timestamp{Seconds: time.Now().Unix()},
		},
	)
	// The output of the program is a CEL map type, but it can be converted
	// to a JSON representation using the `ConvertToNative` method.
	fmt.Printf("------ type conversion ------\n%v\n", valueToJSON(out))
	fmt.Println()
}

// exercise6 describes how to build proto message types within CEL.
//
// Given an input `jwt` and time `now` construct a
// `google.rpc.context.AttributeContext.Request` with the `time` and `auth`
// fields populated according to the go/api-attributes specification.
func exercise6() {
	fmt.Println("=== Exercise 6: Building Protos ===\n")

	// Construct an environment and indicate that the container for all references
	// within the expression is `google.rpc.context.AttributeContext`.
	requestType := &rpcpb.AttributeContext_Request{}
	env, _ := cel.NewEnv(
		cel.Container("google.rpc.context.AttributeContext"),
		cel.Types(requestType),
		cel.Declarations(
			decls.NewVar("jwt", decls.NewMapType(decls.String, decls.Dyn)),
			decls.NewVar("now", decls.Timestamp),
		),
	)

	// Compile the `Request` message construction expression and validate that the
	// resulting expression type matches the fully qualified message name.
	//
	// Note: the field names within the proto message types are not quoted as they
	// are well-defined names composed of valid identifier characters. Also, note
	// that when building nested proto objects, the message name needs to prefix the
	// object construction.
	ast := compile(env, `
		Request{
			auth: Auth{
				principal: jwt.iss + '/' + jwt.sub,
				audiences: [jwt.aud],
				presenter: 'azp' in jwt ? jwt.azp : "",
				claims: jwt
			},
			time: now
		}`,
		decls.NewObjectType("google.rpc.context.AttributeContext.Request"))
	program, _ := env.Program(ast)

	// Construct the message. The result is a ref.Val that returns a dynamic proto message.
	out, _, _ := eval(
		program,
		map[string]interface{}{
			"jwt": map[string]interface{}{
				"sub": "serviceAccount:delegate@acme.co",
				"aud": "my-project",
				"iss": "auth.acme.com:12350",
				"extra_claims": map[string]string{
					"group": "admin",
				},
			},
			"now": &tpb.Timestamp{Seconds: time.Now().Unix()},
		},
	)
	// Unwrap the CEL value to a proto. Make sure to use the `ConvertToNative` to convert
	// the dynamic proto message to the concrete type expected.
	req, err := out.ConvertToNative(reflect.TypeOf(requestType))
	if err != nil {
		glog.Exit(err)
	}
	bytes, err := prototext.Marshal(req.(proto.Message))
	if err != nil {
		glog.Exitf("failed to marshal proto to text: %v", req)
	}
	fmt.Printf("------ type unwrap ------\n%v\n", string(bytes))
	fmt.Println()
}

// exercise7 introduces macros for dealing with repeated fields and maps.
//
// Determine whether the `jwt.extra_claims` has at least one key that starts
// with the `group` prefix, and ensure that all group-like keys have list
// values containing only strings that end with '@acme.co`.
func exercise7() {
	fmt.Println("=== Exercise 7: Macros ===\n")
	env, _ := cel.NewEnv(
		cel.Declarations(decls.NewVar("jwt", decls.Dyn)),
	)
	ast := compile(env,
		`jwt.extra_claims.exists(c, c.startsWith('group'))
				&& jwt.extra_claims
							.filter(c, c.startsWith('group'))
							.all(c, jwt.extra_claims[c]
											 	 .all(g, g.endsWith('@acme.co')))`,
		decls.Bool)
	program, _ := env.Program(ast)

	// Evaluate a complex-ish JWT with two groups that satisfy the criteria.
	// Output: true.
	eval(program,
		map[string]interface{}{
			"jwt": map[string]interface{}{
				"sub": "serviceAccount:delegate@acme.co",
				"aud": "my-project",
				"iss": "auth.acme.com:12350",
				"extra_claims": map[string][]string{
					"group1": {"admin@acme.co", "analyst@acme.co"},
					"labels": {"metadata", "prod", "pii"},
					"groupN": {"forever@acme.co"},
				},
			},
		})

	fmt.Println()
}

// exercise8 covers some useful features of CEL-Go which can be used to
// improve performance and better understand evaluation behavior.
//
// Turn on the optimization, exhaustive eval, and state tracking
// `cel.ProgramOption` flags to see the impact on evaluation behavior.
func exercise8() {
	fmt.Println("=== Exercise 8: Tuning ===\n")
	// Declare the `x` and 'y' variables as input into the expression.
	env, _ := cel.NewEnv(
		cel.Declarations(
			decls.NewVar("x", decls.Int),
			decls.NewVar("y", decls.Uint),
		),
	)
	ast := compile(env,
		`x in [1, 2, 3, 4, 5] && type(y) == uint`,
		decls.Bool)
	// Turn on optimization.
	trueVars := map[string]interface{}{"x": int64(4), "y": uint64(2)}
	program, _ := env.Program(ast, cel.EvalOptions(cel.OptOptimize))
	// Try benchmarking this evaluation with the optimization flag on and off.
	eval(program, trueVars)

	// Turn on exhaustive eval to see what the evaluation state looks like.
	// The input is structure to show a false on the first branch, and true
	// on the second.
	falseVars := map[string]interface{}{"x": int64(6), "y": uint64(2)}
	program, _ = env.Program(ast, cel.EvalOptions(cel.OptExhaustiveEval))
	eval(program, falseVars)

	// Turn on optimization and state tracking to see the typical eval
	// behavior, but with partial input.
	xVar := map[string]interface{}{"x": int64(3)}
	partialVars, _ := cel.PartialVars(xVar, cel.AttributePattern("y"))
	program, _ = env.Program(ast,
		cel.EvalOptions(cel.OptPartialEval, cel.OptOptimize, cel.OptTrackState))
	_, details, _ := eval(program, partialVars)

	// Convert the unknown parts of the expression to a new AST and format it back
	// to a human-readable expression.
	residualAst, _ := env.ResidualAst(ast, details)
	residual, _ := cel.AstToString(residualAst)
	fmt.Printf("------ residual ------\n%s\n", residual)

	fmt.Println()
}

// Functions to assist with CEL execution.

// compile will parse and check an expression `expr` against a given
// environment `env` and determine whether the resulting type of the expression
// matches the `exprType` provided as input.
func compile(env *cel.Env, expr string, exprType *exprpb.Type) *cel.Ast {
	ast, iss := env.Compile(expr)
	if iss.Err() != nil {
		glog.Exit(iss.Err())
	}
	if !proto.Equal(ast.ResultType(), exprType) {
		glog.Exitf(
			"Got %v, wanted %v result type",
			ast.ResultType(),
			exprType)
	}
	fmt.Printf("%s\n\n", strings.ReplaceAll(expr, "\t", " "))
	return ast
}

// eval will evaluate a given program `prg` against a set of variables `vars`
// and return the output, eval details (optional), or error that results from
// evaluation.
func eval(prg cel.Program,
	vars interface{}) (out ref.Val, det *cel.EvalDetails, err error) {
	varMap, isMap := vars.(map[string]interface{})
	fmt.Println("------ input ------")
	if !isMap {
		fmt.Printf("(%T)\n", vars)
	} else {
		for k, v := range varMap {
			switch val := v.(type) {
			case proto.Message:
				bytes, err := prototext.Marshal(val)
				if err != nil {
					glog.Exitf("failed to marshal proto to text: %v", val)
				}
				fmt.Printf("%s = %s", k, string(bytes))
			case map[string]interface{}:
				b, _ := json.MarshalIndent(v, "", "  ")
				fmt.Printf("%s = %v\n", k, string(b))
			case uint64:
				fmt.Printf("%s = %vu\n", k, v)
			default:
				fmt.Printf("%s = %v\n", k, v)
			}
		}
	}
	fmt.Println()
	out, det, err = prg.Eval(vars)
	report(out, det, err)
	fmt.Println()
	return
}

// report prints out the result of evaluation in human-friendly terms.
func report(result ref.Val, details *cel.EvalDetails, err error) {
	fmt.Println("------ result ------")
	if err != nil {
		fmt.Printf("error: %s\n", err)
	} else {
		fmt.Printf("value: %v (%T)\n", result, result)
	}
	if details != nil {
		fmt.Printf("\n------ eval states ------\n")
		state := details.State()
		stateIDs := state.IDs()
		ids := make([]int, len(stateIDs), len(stateIDs))
		for i, id := range stateIDs {
			ids[i] = int(id)
		}
		sort.Ints(ids)
		for _, id := range ids {
			v, found := state.Value(int64(id))
			if !found {
				continue
			}
			fmt.Printf("%d: %v (%T)\n", id, v, v)
		}
	}
}

// mapContainsKeyValue implements the custom function:
//   `map.contains(key, value) bool`.
func mapContainsKeyValue(args ...ref.Val) ref.Val {
	// Check the argument input count.
	if len(args) != 3 {
		return types.NewErr("no such overload")
	}
	obj := args[0]
	m, isMap := obj.(traits.Mapper)
	// Ensure the argument is a CEL map type, otherwise error.
	// The type-checking is a best effort check to ensure that the types provided
	// to functions match the ones specified; however, it is always possible that
	// the implementation does not match the declaration. Always check arguments
	// types whenever there is a possibility that your function will deal with
	// dynamic content.
	if !isMap {
		// The helper ValOrErr ensures that errors on input are propagated.
		return types.ValOrErr(obj, "no such overload")
	}

	// CEL has many interfaces for dealing with different type abstractions.
	// The traits.Mapper interface unifies field presence testing on proto
	// messages and maps.
	key := args[1]
	v, found := m.Find(key)
	// If not found and the value was non-nil, the value is an error per the
	// `Find` contract. Propagate it accordingly.
	if !found {
		if v != nil {
			return types.ValOrErr(v, "unsupported key type")
		}
		// Return CEL False if the key was not found.
		return types.False
	}
	// Otherwise whether the value at the key equals the value provided.
	return v.Equal(args[2])
}

// Functions for constructing CEL inputs.

// auth constructs a `google.rpc.context.AttributeContext.Auth` message.
func auth(user string, claims map[string]string) *rpcpb.AttributeContext_Auth {
	claimFields := make(map[string]*structpb.Value)
	for k, v := range claims {
		claimFields[k] = structpb.NewStringValue(v)
	}
	return &rpcpb.AttributeContext_Auth{
		Principal: user,
		Claims:    &structpb.Struct{Fields: claimFields},
	}
}

// request constructs a `google.rpc.context.AttributeContext.Request` message.
func request(auth *rpcpb.AttributeContext_Auth, t time.Time) map[string]interface{} {
	req := &rpcpb.AttributeContext_Request{
		Auth: auth,
		Time: &tpb.Timestamp{Seconds: t.Unix()},
	}
	return map[string]interface{}{"request": req}
}

// valueToJSON converts the CEL type to a protobuf JSON representation and
// marshals the result to a string.
func valueToJSON(val ref.Val) string {
	v, err := val.ConvertToNative(reflect.TypeOf(&structpb.Value{}))
	if err != nil {
		glog.Exit(err)
	}
	marshaller := protojson.MarshalOptions{Indent: "    "}
	bytes, err := marshaller.Marshal(v.(proto.Message))
	if err != nil {
		glog.Exit(err)
	}
	return string(bytes)
}

var (
	emptyClaims = make(map[string]string)
)
