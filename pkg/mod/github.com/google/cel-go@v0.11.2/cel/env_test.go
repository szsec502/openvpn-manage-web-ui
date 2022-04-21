// Copyright 2021 Google LLC
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
	"reflect"
	"testing"

	"github.com/google/cel-go/checker/decls"
	"github.com/google/cel-go/common"
	"github.com/google/cel-go/common/operators"
	"github.com/google/cel-go/common/types"

	exprpb "google.golang.org/genproto/googleapis/api/expr/v1alpha1"
)

func TestIssuesNil(t *testing.T) {
	var iss *Issues
	iss = iss.Append(iss)
	if iss.Err() != nil {
		t.Errorf("iss.Err() got %v, wanted nil given nil issue set", iss.Err())
	}
	if len(iss.Errors()) != 0 {
		t.Errorf("iss.Errors() got %v, wanted empty value", iss.Errors())
	}
	if iss.String() != "" {
		t.Errorf("iss.String() returned %v, wanted empty value", iss.String())
	}
}

func TestIssuesEmpty(t *testing.T) {
	iss := NewIssues(common.NewErrors(nil))
	if iss.Err() != nil {
		t.Errorf("iss.Err() got %v, wanted nil given nil issue set", iss.Err())
	}
	if len(iss.Errors()) != 0 {
		t.Errorf("iss.Errors() got %v, wanted empty value", iss.Errors())
	}
	if iss.String() != "" {
		t.Errorf("iss.String() returned %v, wanted empty value", iss.String())
	}
	var iss2 *Issues
	iss3 := iss.Append(iss2)
	iss4 := iss3.Append(nil)
	if !reflect.DeepEqual(iss4, iss) {
		t.Error("Append() with a nil value resulted in the creation of a new issue set")
	}
}

func TestIssues(t *testing.T) {
	e, err := NewEnv()
	if err != nil {
		t.Fatalf("NewEnv() failed: %v", err)
	}
	_, iss := e.Compile("-")
	_, iss2 := e.Compile("b")
	iss = iss.Append(iss2)
	if len(iss.Errors()) != 3 {
		t.Errorf("iss.Errors() got %v, wanted 3 errors", iss.Errors())
	}

	wantIss := `ERROR: <input>:1:1: undeclared reference to 'b' (in container '')
 | -
 | ^
ERROR: <input>:1:2: Syntax error: no viable alternative at input '-'
 | -
 | .^
ERROR: <input>:1:2: Syntax error: mismatched input '<EOF>' expecting {'[', '{', '(', '.', '-', '!', 'true', 'false', 'null', NUM_FLOAT, NUM_INT, NUM_UINT, STRING, BYTES, IDENTIFIER}
 | -
 | .^`
	if iss.String() != wantIss {
		t.Errorf("iss.String() returned %v, wanted %v", iss.String(), wantIss)
	}
}

func BenchmarkNewCustomEnvLazy(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := NewCustomEnv(StdLib(), EagerlyValidateDeclarations(false))
		if err != nil {
			b.Fatalf("NewCustomEnv() failed: %v", err)
		}
	}
}

func BenchmarkNewCustomEnvEager(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, err := NewCustomEnv(StdLib(), EagerlyValidateDeclarations(true))
		if err != nil {
			b.Fatalf("NewCustomEnv() failed: %v", err)
		}
	}
}

func BenchmarkNewEnvLazy(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, err := NewEnv()
		if err != nil {
			b.Fatalf("NewEnv() failed: %v", err)
		}
	}
}

func BenchmarkNewEnvEager(b *testing.B) {
	for i := 0; i < b.N; i++ {
		env, err := NewEnv()
		if err != nil {
			b.Fatalf("NewEnv() failed: %v", err)
		}
		_, iss := env.Compile("123")
		if iss.Err() != nil {
			b.Fatalf("env.Compile(123) failed: %v", iss.Err())
		}
	}
}

func BenchmarkEnvExtendEager(b *testing.B) {
	env, err := NewEnv()
	if err != nil {
		b.Fatalf("NewEnv() failed: %v", err)
	}
	for i := 0; i < b.N; i++ {
		ext, err := env.Extend()
		if err != nil {
			b.Fatalf("env.Extend() failed: %v", err)
		}
		_, iss := ext.Compile("123")
		if iss.Err() != nil {
			b.Fatalf("env.Compile(123) failed: %v", iss.Err())
		}
	}
}

func BenchmarkEnvExtendEagerTypes(b *testing.B) {
	env, err := NewEnv(EagerlyValidateDeclarations(true))
	if err != nil {
		b.Fatalf("NewEnv() failed: %v", err)
	}
	for i := 0; i < b.N; i++ {
		ext, err := env.Extend(CustomTypeProvider(types.NewEmptyRegistry()))
		if err != nil {
			b.Fatalf("env.Extend() failed: %v", err)
		}
		_, iss := ext.Compile("123")
		if iss.Err() != nil {
			b.Fatalf("env.Compile(123) failed: %v", iss.Err())
		}
	}
}

func BenchmarkEnvExtendEagerDecls(b *testing.B) {
	env, err := NewEnv(EagerlyValidateDeclarations(true))
	if err != nil {
		b.Fatalf("NewEnv() failed: %v", err)
	}
	for i := 0; i < b.N; i++ {
		ext, err := env.Extend(
			Declarations(
				decls.NewVar("test_var", decls.String),
				decls.NewFunction(
					operators.In,
					decls.NewOverload("string_in_string", []*exprpb.Type{
						decls.String, decls.String,
					}, decls.Bool)),
			),
		)
		if err != nil {
			b.Fatalf("env.Extend() failed: %v", err)
		}
		_, iss := ext.Compile("123")
		if iss.Err() != nil {
			b.Fatalf("env.Compile(123) failed: %v", iss.Err())
		}
	}
}
