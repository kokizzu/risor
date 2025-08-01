package risor

import (
	"context"
	"fmt"

	"github.com/risor-io/risor/compiler"
	"github.com/risor-io/risor/object"
	"github.com/risor-io/risor/parser"
	"github.com/risor-io/risor/vm"
)

//go:generate go run ./cmd/risor-modgen

// Eval evaluates the given source code and returns the result.
func Eval(ctx context.Context, source string, options ...Option) (object.Object, error) {
	cfg := NewConfig(options...)

	// Parse the source code to create the AST
	var parserOpts []parser.Option
	if cfg.filename != "" {
		parserOpts = append(parserOpts, parser.WithFilename(cfg.filename))
	}
	ast, err := parser.Parse(ctx, source, parserOpts...)
	if err != nil {
		return nil, err
	}

	// Compile the AST to bytecode, appending these new instructions after any
	// instructions that were previously compiled
	main, err := compiler.Compile(ast, cfg.CompilerOpts()...)
	if err != nil {
		return nil, err
	}

	// Use the specified VM if provided
	if cfg.vm != nil {
		return vm.RunCodeOnVM(ctx, cfg.vm, main, cfg.VMOpts()...)
	}

	// Eval the bytecode in a new VM then return the top-of-stack (TOS) value
	return vm.Run(ctx, main, cfg.VMOpts()...)
}

// EvalCode evaluates the precompiled code and returns the result.
func EvalCode(ctx context.Context, main *compiler.Code, options ...Option) (object.Object, error) {
	cfg := NewConfig(options...)

	// Use the specified VM if provided
	if cfg.vm != nil {
		return vm.RunCodeOnVM(ctx, cfg.vm, main, cfg.VMOpts()...)
	}

	// Eval the bytecode in a VM then return the top-of-stack (TOS) value
	return vm.Run(ctx, main, cfg.VMOpts()...)
}

// Call evaluates the precompiled code and then calls the named function.
// The supplied arguments are passed in the function call. The result of
// the function call is returned.
func Call(
	ctx context.Context,
	main *compiler.Code,
	functionName string,
	args []object.Object,
	options ...Option,
) (object.Object, error) {
	cfg := NewConfig(options...)

	// Determine whether to use an existing VM or create a new one
	var err error
	var machine *vm.VirtualMachine
	if cfg.vm != nil {
		machine = cfg.vm
	} else {
		machine, err = vm.NewEmpty()
		if err != nil {
			return nil, err
		}
	}

	// Run the code to evaluate globals etc.
	if err := machine.RunCode(ctx, main, cfg.VMOpts()...); err != nil {
		return nil, err
	}

	// Get the requested function
	obj, err := machine.Get(functionName)
	if err != nil {
		return nil, err
	}
	fn, ok := obj.(*object.Function)
	if !ok {
		return nil, fmt.Errorf("object is not a function (got: %s)", obj.Type())
	}

	// Call the function
	return machine.Call(ctx, fn, args)
}
