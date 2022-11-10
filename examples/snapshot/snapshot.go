package main

import (
	"context"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"os"
	"sync/atomic"
	"time"

	"github.com/eiannone/keyboard"
	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/internal/proto"
	"github.com/tetratelabs/wazero/internal/wasm"
	"github.com/tetratelabs/wazero/internal/wasmruntime"
	"github.com/tetratelabs/wazero/wasi_snapshot_preview1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	pb "google.golang.org/protobuf/proto"
)

const (
	fibPath             = "./testdata/fib.wasm"
	fibFunction         = "fib"
	catPath             = "./testdata/cat.wasm"
	catFunction         = "entry"
	transcodingPath     = "./testdata/transcoding.wasm"
	transcodingFunction = "_start"
	serverPath          = "./testdata/server.wasm"
	serverFunction      = "request"
	maxSize             = 1 << (10 * 3) // 1 GB
)

var (
	program = flag.String("program", "fib", "the program to run. fib | cat | transcoding")

	trap     = flag.Bool("trap", false, "trap after snapshot")
	quiet    = flag.Bool("quiet", false, "be quiet")
	noIndent = flag.Bool("no-indent", false, "do not indent trace")

	// mode
	mode = flag.String("mode", "local", "The execution mode. Possible values: local  remote")

	// flags for local
	export       = flag.Bool("export", false, "export snapshot, then stop.")
	fromSnapshot = flag.String("from-snapshot", "", "continue from snapshot file")

	// flags for remote
	port  = flag.Int("port", 50051, "The server port")
	peer  = flag.Int("peer", 50052, "The peer port")
	start = flag.Bool("start", false, "Start the execution")

	// Snapshot creation: all instructions, interactive, or timeout.
	snapshotAll         = flag.Bool("all", false, "Snapshot after every instruction.")
	snapshotInteractive = flag.Bool("interactive", false, "Use keyboard for snapshotting")
	snapshotTimeout     = flag.Int("snapshotS", 0, "time to automatically create a snapshot, in seconds")

	// benchmarking
	benchmark       = flag.Bool("bench", false, "Benchmark snapshot")
	timeStart       time.Time
	timeStartExport time.Time

	path     string
	function string

	client *proto.ExecutorClient = nil
)

type server struct {
	proto.UnimplementedExecutorServer
}

func main() {
	flag.Parse()

	if *program == "fib" {
		path = fibPath
		function = fibFunction
	} else if *program == "cat" {
		path = catPath
		function = catFunction
	} else if *program == "transcoding" {
		path = transcodingPath
		function = transcodingFunction
	} else if *program == "server" {
		path = serverPath
		function = serverFunction
	} else {
		log.Fatalln("invalid program")
	}

	// Choose the context to use for function calls.
	ctx := context.Background()

	snapshot := &proto.Snapshot{}
	ctx = context.WithValue(ctx, "snapshot", snapshot)

	ctx = readFlags(ctx)

	if *snapshotInteractive {
		go monitorKeys()
	}

	if *mode == "local" {
		for {
			execute(ctx)
		}
	} else {
		// mode == remote
		if *start {
			go execute(ctx)
		}

		// launch rpc server
		lis, err := net.Listen("tcp", fmt.Sprintf(":%d", *port))
		if err != nil {
			log.Fatalf("failed to listen: %v", err)
		}
		s := grpc.NewServer(grpc.MaxRecvMsgSize(maxSize))
		proto.RegisterExecutorServer(s, &server{})
		log.Printf("server listening at %v", lis.Addr())
		if err := s.Serve(lis); err != nil {
			log.Fatalf("failed to serve: %v", err)
		}
	}

}
func (s *server) Execute(ctx context.Context, in *proto.ExecuteRequest) (*proto.ExecuteResponse, error) {
	fmt.Println("got execute request")
	ctx = readFlags(ctx)
	ctx = context.WithValue(ctx, "snapshot", in.Snapshot)
	go execute(ctx)
	return &proto.ExecuteResponse{Ok: true}, nil
}

func (s *server) Finish(ctx context.Context, in *proto.Empty) (*proto.Empty, error) {
	go func() {
		time.Sleep(500 * time.Millisecond)
		os.Exit(0)
	}()
	return &proto.Empty{}, nil
}

func readFlags(ctx context.Context) context.Context {
	// check snapshot params
	if *snapshotInteractive && *snapshotTimeout > 0 {
		log.Fatalln("-interactive and -snapshotS cannot be set at the same time")
	}

	// check mode
	if *mode != "local" && *mode != "remote" {
		log.Fatalln("-mode must be either local or remote")
	}

	// disallow port, peer, start in local mode
	if *mode == "local" && *start {
		log.Fatalln("set mode to remote to use start arg")
	}

	ctx = context.WithValue(ctx, "trap", *trap)
	ctx = context.WithValue(ctx, "quiet", *quiet)
	ctx = context.WithValue(ctx, "no-indent", *noIndent)
	ctx = context.WithValue(ctx, "export", *export)
	ctx = context.WithValue(ctx, "port", *port)
	ctx = context.WithValue(ctx, "peer", *peer)
	ctx = context.WithValue(ctx, "send", *start)
	ctx = context.WithValue(ctx, "mode", *mode)
	ctx = context.WithValue(ctx, "all", *snapshotAll)
	ctx = context.WithValue(ctx, "benchmark", *benchmark)
	ctx = context.WithValue(ctx, "timeStart", &timeStart)
	ctx = context.WithValue(ctx, "timeStartExport", &timeStartExport)

	// read snapshot from file if flag is given
	if *fromSnapshot != "" {
		in, err := ioutil.ReadFile(*fromSnapshot)
		if err != nil {
			log.Fatalln("Error reading file:", err)
		}

		snapshotPb := &proto.Snapshot{}
		if err := pb.Unmarshal(in, snapshotPb); err != nil {
			log.Fatalln("Failed to parse snapshot:", err)
		}

		snapshot := ctx.Value("snapshot").(*proto.Snapshot)
		*snapshot = *snapshotPb
	}

	return ctx
}

func initClient() {
	// Set up a connection to the server.
	conn, err := grpc.Dial(fmt.Sprintf(":%d", *peer), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("did not connect: %v", err)
	}
	c := proto.NewExecutorClient(conn)
	client = &c
}

func sendSnapshot(ctx context.Context, snapshot *proto.Snapshot) {
	if client == nil {
		initClient()
	}
	c := *client

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	r, err := c.Execute(ctx, &proto.ExecuteRequest{Snapshot: snapshot}, grpc.WaitForReady(true), grpc.MaxCallSendMsgSize(maxSize), grpc.MaxCallRecvMsgSize(maxSize))
	if err != nil {
		log.Fatalf("could not execute: %v", err)
	}
	log.Printf("Sent execute request. Ok: %t\n", r.Ok)
}

func sendFinish(ctx context.Context) {
	if client == nil {
		initClient()
	}
	c := *client
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_, err := c.Finish(ctx, &proto.Empty{}, grpc.WaitForReady(true))
	if err != nil {
		log.Fatalf("could not finish: %v", err)
	}
}

func execute(ctx context.Context) {
	snapshot := ctx.Value("snapshot").(*proto.Snapshot)

	// Read a WebAssembly binary containing an exported "entry" function.
	wasmBytes, err := os.ReadFile(path)
	if err != nil {
		log.Panicln(err)
	}

	// Create a new WebAssembly Runtime.
	r := wazero.NewRuntimeWithConfig(wazero.NewRuntimeConfigInterpreter().WithWasmCore2())

	defer r.Close(ctx) // This closes everything this Runtime created.

	// Instantiate WASI, which implements system I/O such as console output.
	if _, err = wasi_snapshot_preview1.Instantiate(ctx, r); err != nil {
		log.Panicln(err)
	}

	config := wazero.NewModuleConfig().
		// By default, I/O streams are discarded and there's no file system.
		WithStdout(os.Stdout).WithStderr(os.Stderr)

	if *program == "transcoding" {
		// for transcoding, do not initialize the module with the _start function.
		// instead, call it directly.
		config = config.WithStartFunctions()
		config = config.WithArgs("file", "video1s.mpeg", "video2.mpeg")
	}

	if *program == "cat" || *program == "transcoding" {
		config = config.WithFS(os.DirFS("."))
	}

	// If we restore from a snapshot, never initialize the module with the _start function.
	if snapshot.Valid {
		config = config.WithStartFunctions()
	}

	// Compile the WebAssembly module using the default configuration.
	code, err := r.CompileModule(ctx, wasmBytes, wazero.NewCompileConfig())
	if err != nil {
		log.Panicln(err)
	}

	// Instantiate the module and return its exported functions
	module, err := r.InstantiateModule(context.WithValue(ctx, "snapshot", nil), code, config)
	if err != nil {
		fmt.Println("panic instantiating module")
		log.Panicln(err)
	}

	wasmFunction := module.ExportedFunction(function).(*wasm.FunctionInstance)

	if *program == "server" {
		for {
			char, _, err := keyboard.GetSingleKey()
			if err != nil {
				panic(err)
			}
			if char == 'r' {
				if snapshot.Valid {
					_, err = wasmFunction.Resume(ctx, snapshot)
					snapshot.Valid = false
				} else {
					_, err = wasmFunction.Call(ctx)
				}

				if err != nil {
					log.Panicln(err)
				}
			} else if char == 's' {
				atomic.StoreUint32(&wasm.BreakpointFlag, 1)
				if snapshot.Valid {
					_, err = wasmFunction.Resume(ctx, snapshot)
				} else {
					_, err = wasmFunction.Call(ctx)
				}

				os.Exit(0)
			} else if char == '\x00' {
				fmt.Println("aborted execution")
				os.Exit(0)
			}
		}
	}

	var res, params []uint64
	// Discover fib(9) is 34
	if *program == "fib" {
		params = []uint64{9}
	}

	if *snapshotTimeout > 0 {
		go func() {
			time.Sleep(time.Duration(*snapshotTimeout) * time.Second)
			atomic.StoreUint32(&wasm.BreakpointFlag, 1)
		}()
	} else if *snapshotAll {
		atomic.StoreUint32(&wasm.BreakpointFlag, 1)
	}

	if snapshot.Valid {
		fmt.Println("resume")
		res, err = wasmFunction.Resume(ctx, snapshot)
	} else {
		res, err = wasmFunction.Call(ctx, params...)
	}

	module.Close(ctx)

	switch err {
	case wasmruntime.ErrRuntimeSnapshot:
		//log.Printf("snapshot: %v\n", snapshot)
		// send snapshot
		if *mode == "remote" {
			sendSnapshot(ctx, snapshot)
		}
	case nil:
		fmt.Printf("result: %d\n", res)
		if *mode == "remote" {
			sendFinish(ctx)
		}
		os.Exit(0)
	default:
		log.Panicln(err)
	}
}

// monitor keys. if Ctrl+S
func monitorKeys() {
	fmt.Println("press s for snapshot")
	for {
		char, _, err := keyboard.GetSingleKey()
		if err != nil {
			panic(err)
		}
		if char == 's' {
			fmt.Println("\n## Snapshot ##\n")
			atomic.StoreUint32(&wasm.BreakpointFlag, 1)
		} else if char == '\x00' {
			fmt.Println("aborted execution")
			os.Exit(0)
		}
	}
}
