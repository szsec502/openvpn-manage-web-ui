# Examples

## Simple example using builtin operators

Evaluate expression `"Hello world! I'm " + name + "."` with `CEL` passed as
name.

```go
import (
    "github.com/google/cel-go/cel"
    "github.com/google/cel-go/checker/decls"
)

    d := cel.Declarations(decls.NewVar("name", decls.String))
    env, err := cel.NewEnv(d)

    ast, iss := env.Compile(`"Hello world! I'm " + name + "."`)
    // Check iss for compilation errors.
    if iss.Err() != nil {
        log.Fatalln(iss.Err())
    }
    prg, err := env.Program(ast)
    out, _, err := prg.Eval(map[string]interface{}{
        "name":   "CEL",
    })
    fmt.Println(out)
    // Output:Hello world! I'm CEL.
```

[Source code](simple_test.go)

## Custom function on string type

Evaluate expression `i.greet(you)` with:

```
    i       -> CEL
    you     -> world
    greet   -> "Hello %s! Nice to meet you, I'm %s."
```

First we need to declare two string variables and `greet` function.
`NewInstanceOverload` must be used if we want to declare function which will
operate on a type. First element of slice passed as `argTypes` into
`NewInstanceOverload` is declaration of instance type. Next elements are
parameters of function.

```go
    decls.NewVar("i", decls.String),
    decls.NewVar("you", decls.String),
    decls.NewFunction("greet",
        decls.NewInstanceOverload("string_greet_string",
            []*exprpb.Type{decls.String, decls.String},
            decls.String))
    ... // Create env and compile
```

Let's implement `greet` function and pass it to `program`. We will be using
`Binary`, because `greet` function uses 2 parameters (1st instance, 2nd
function parameter).

```go
    greetFunc := &functions.Overload{
        Operator: "string_greet_string",
        Binary: func(lhs ref.Val, rhs ref.Val) ref.Val {
            return types.String(
                fmt.Sprintf("Hello %s! Nice to meet you, I'm %s.\n", rhs, lhs))
            }}
    prg, err := env.Program(c, cel.Functions(greetFunc))

    out, _, err := prg.Eval(map[string]interface{}{
        "i": "CEL",
        "you": "world",
    })
    fmt.Println(out)
    // Output:Hello world! Nice to meet you, I'm CEL.
```
[Source code](custom_instance_function_test.go)

## Define custom global function

Evaluate expression `shake_hands(i,you)` with:

```
    i           -> CEL
    you         -> world
    shake_hands -> "%s and %s are shaking hands."
```

In order to declare global function we need to use `NewOverload`:

```go
    decls.NewVar("i", decls.String),
    decls.NewVar("you", decls.String),
    decls.NewFunction("shake_hands",
        decls.NewOverload("shake_hands_string_string",
            []*exprpb.Type{decls.String, decls.String},
            decls.String))
    ... // Create env and compile.

    shakeFunc := &functions.Overload{
        Operator: "shake_hands_string_string",
        Binary: func(lhs ref.Val, rhs ref.Val) ref.Val {
            return types.String(
                fmt.Sprintf("%s and %s are shaking hands.\n", lhs, rhs))
            }}
    prg, err := env.Program(c, cel.Functions(shakeFunc))

    out, _, err := prg.Eval(map[string]interface{}{
        "i": "CEL",
        "you": "world",
    })
    fmt.Println(out)
    // Output:CEL and world are shaking hands.
```

[Source code](custom_global_function_test.go)

For more examples of how to use CEL, see
[cel_test.go](https://github.com/google/cel-go/tree/master/cel/cel_test.go).
