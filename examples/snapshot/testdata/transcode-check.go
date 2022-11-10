package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/wasi_snapshot_preview1"
)

func main() {
	// Choose the context to use for function calls.
	ctx := context.Background()

	// Read a WebAssembly binary containing an exported "fac" function.
	wasm, err := os.ReadFile("transcoding.wasm")
	if err != nil {
		log.Panicln(err)
	}

	// Create a new WebAssembly Runtime.
	r := wazero.NewRuntimeWithConfig(wazero.NewRuntimeConfig().WithWasmCore2())
	defer r.Close(ctx) // This closes everything this Runtime created.

	if _, err = wasi_snapshot_preview1.Instantiate(ctx, r); err != nil {
		log.Panicln(err)
	}

	config := wazero.NewModuleConfig().
		WithSysWalltime().
		WithSysNanotime(). // instead of fake time
		WithStdout(os.Stdout).
		WithStderr(os.Stderr).
		WithArgs("file", "video.mpeg", "video2.mpeg").
		WithStartFunctions().
		WithFS(os.DirFS("."))

	code, err := r.CompileModule(ctx, wasm, wazero.NewCompileConfig())

	// Instantiate the module and return its exported functions
	module, err := r.InstantiateModule(ctx, code, config)
	if err != nil {
		log.Println("cannot instantiate module")
		log.Panicln(err)
	}

	fmt.Println(module.ExportedFunction("_start").Call(ctx))
}
