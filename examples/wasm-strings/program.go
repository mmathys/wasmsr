package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/internal/wasm"
	"github.com/tetratelabs/wazero/wasi_snapshot_preview1"
)

func main() {
	// Choose the context to use for function calls.
	ctx := context.Background()

	wasmBin, err := os.ReadFile("./testdata/program.wasm")
	if err != nil {
		log.Panicln(err)
	}

	// Create a new WebAssembly Runtime.
	r := wazero.NewRuntimeWithConfig(wazero.NewRuntimeConfig().WithWasmCore2())
	//defer r.Close(ctx) // This closes everything this Runtime created.

	// Instantiate WASI, which implements system I/O such as console output.
	if _, err = wasi_snapshot_preview1.Instantiate(ctx, r); err != nil {
		log.Panicln(err)
	}

	config := wazero.NewModuleConfig().
		// By default, I/O streams are discarded and there's no file system.
		WithStdout(os.Stdout).WithStderr(os.Stderr).WithArgs("file", "passed arg").WithStartFunctions()

	code, err := r.CompileModule(ctx, wasmBin, wazero.NewCompileConfig())
	if err != nil {
		log.Panicln(err)
	}

	// Instantiate the module and return its exported functions
	module, err := r.InstantiateModule(ctx, code, config)
	if err != nil {
		log.Println("instantiate panic")
		log.Panicln(err)
	}

	str := "hello\x00"
	strLen := uint64(len(str))

	malloc := module.ExportedFunction("malloc")
	results, err := malloc.Call(ctx, strLen)
	if err != nil {
		log.Println("malloc panic")
		log.Panic(err)
	}
	inputPtr := uint32(results[0])
	fmt.Printf("pointer: %v\n", inputPtr)

	if !module.Memory().Write(ctx, inputPtr, []byte(str)) {
		log.Panicf("Memory.Write(%d, %d) out of range of memory size %d",
			inputPtr, strLen, module.Memory().Size(ctx))
	}

	startFunction := module.ExportedFunction("_start").(*wasm.FunctionInstance)

	//res, err := startFunction.Call(ctx, uint64(inputPtr))
	res, err := startFunction.Call(ctx)
	if err != nil {
		log.Panic(err)
	}
	log.Printf("result: %v\n", res)
}
