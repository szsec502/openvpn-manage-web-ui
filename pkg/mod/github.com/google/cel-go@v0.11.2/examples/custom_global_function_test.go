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

package examples

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

func ExampleCustomGlobalFunction() {
	d := cel.Declarations(
		decls.NewVar("i", decls.String),
		decls.NewVar("you", decls.String),
		decls.NewFunction("shake_hands",
			decls.NewOverload("shake_hands_string_string",
				[]*exprpb.Type{decls.String, decls.String},
				decls.String)))
	env, err := cel.NewEnv(d)
	if err != nil {
		log.Fatalf("environment creation error: %v\n", err)
	}
	// Check iss for error in both Parse and Check.
	ast, iss := env.Compile(`shake_hands(i,you)`)
	if iss.Err() != nil {
		log.Fatalln(iss.Err())
	}
	shakeFunc := &functions.Overload{
		Operator: "shake_hands_string_string",
		Binary: func(lhs ref.Val, rhs ref.Val) ref.Val {
			return types.String(
				fmt.Sprintf("%s and %s are shaking hands.\n", lhs, rhs))
		}}
	prg, err := env.Program(ast, cel.Functions(shakeFunc))
	if err != nil {
		log.Fatalf("Program creation error: %v\n", err)
	}

	out, _, err := prg.Eval(map[string]interface{}{
		"i":   "CEL",
		"you": "world",
	})
	if err != nil {
		log.Fatalf("Evaluation error: %v\n", err)
	}

	fmt.Println(out)
	// Output:CEL and world are shaking hands.
}
