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

package cel_test

import (
	"fmt"
	"log"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/checker/decls"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"github.com/google/cel-go/interpreter/functions"

	exprpb "google.golang.org/genproto/googleapis/api/expr/v1alpha1"
)

func Example() {
	// Create the CEL environment with declarations for the input attributes and
	// the desired extension functions. In many cases the desired functionality will
	// be present in a built-in function.
	decls := cel.Declarations(
		// Identifiers used within this expression.
		decls.NewVar("i", decls.String),
		decls.NewVar("you", decls.String),
		// Function to generate a greeting from one person to another.
		//    i.greet(you)
		decls.NewFunction("greet",
			decls.NewInstanceOverload("string_greet_string",
				[]*exprpb.Type{decls.String, decls.String},
				decls.String)))
	e, err := cel.NewEnv(decls)
	if err != nil {
		log.Fatalf("environment creation error: %s\n", err)
	}

	// Compile the expression.
	ast, iss := e.Compile("i.greet(you)")
	if iss.Err() != nil {
		log.Fatalln(iss.Err())
	}

	// Create the program.
	funcs := cel.Functions(
		&functions.Overload{
			Operator: "string_greet_string",
			Binary: func(lhs ref.Val, rhs ref.Val) ref.Val {
				return types.String(
					fmt.Sprintf("Hello %s! Nice to meet you, I'm %s.\n", rhs, lhs))
			}})
	prg, err := e.Program(ast, funcs)
	if err != nil {
		log.Fatalf("program creation error: %s\n", err)
	}

	// Evaluate the program against some inputs. Note: the details return is not used.
	out, _, err := prg.Eval(map[string]interface{}{
		// Native values are converted to CEL values under the covers.
		"i": "CEL",
		// Values may also be lazily supplied.
		"you": func() ref.Val { return types.String("world") },
	})
	if err != nil {
		log.Fatalf("runtime error: %s\n", err)
	}

	fmt.Println(out)
	// Output:Hello world! Nice to meet you, I'm CEL.
}

func Example_globalOverload() {
	// The GlobalOverload example demonstrates how to define global overload function.
	// Create the CEL environment with declarations for the input attributes and
	// the desired extension functions. In many cases the desired functionality will
	// be present in a built-in function.
	decls := cel.Declarations(
		// Identifiers used within this expression.
		decls.NewVar("i", decls.String),
		decls.NewVar("you", decls.String),
		// Function to generate shake_hands between two people.
		//    shake_hands(i,you)
		decls.NewFunction("shake_hands",
			decls.NewOverload("shake_hands_string_string",
				[]*exprpb.Type{decls.String, decls.String},
				decls.String)))
	e, err := cel.NewEnv(decls)
	if err != nil {
		log.Fatalf("environment creation error: %s\n", err)
	}

	// Compile the expression.
	ast, iss := e.Compile(`shake_hands(i,you)`)
	if iss.Err() != nil {
		log.Fatalln(iss.Err())
	}

	// Create the program.
	funcs := cel.Functions(
		&functions.Overload{
			Operator: "shake_hands_string_string",
			Binary: func(lhs ref.Val, rhs ref.Val) ref.Val {
				s1, ok := lhs.(types.String)
				if !ok {
					return types.ValOrErr(lhs, "unexpected type '%v' passed to shake_hands", lhs.Type())
				}
				s2, ok := rhs.(types.String)
				if !ok {
					return types.ValOrErr(rhs, "unexpected type '%v' passed to shake_hands", rhs.Type())
				}
				return types.String(
					fmt.Sprintf("%s and %s are shaking hands.\n", s1, s2))
			}})
	prg, err := e.Program(ast, funcs)
	if err != nil {
		log.Fatalf("program creation error: %s\n", err)
	}

	// Evaluate the program against some inputs. Note: the details return is not used.
	out, _, err := prg.Eval(map[string]interface{}{
		"i":   "CEL",
		"you": func() ref.Val { return types.String("world") },
	})
	if err != nil {
		log.Fatalf("runtime error: %s\n", err)
	}

	fmt.Println(out)
	// Output:CEL and world are shaking hands.
}
