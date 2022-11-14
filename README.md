# WasmSR: WebAssembly runtime with snapshotting and restoring functionality 

WebAssembly is a way to safely run code compiled in other languages. Runtimes
execute WebAssembly Modules (Wasm), which are most often binaries with a `.wasm`
extension.

This runtime is based on wazero, a WebAssembly Core Specification 1.0 and 2.0 compliant runtime written in Go.

This video presents the FFmpeg experiment. Two Docker containers were set up,
and the video transcoding is migrated every 10 seconds between the two
containers. The logs for the Docker containers can be seen with the
prefix `host-1` and `host-2`.

https://user-images.githubusercontent.com/5797176/201670697-8ed04765-094a-4310-8568-459d61cd3da3.mp4

## Running Examples

The snapshotting example can be found in [examples/snapshot](examples/snapshot/).

Below commands are listed to execute the experiments mentioned in the thesis.

### Run the fibonacci example fib(9)

```
go run snapshot.go -quiet
```

### Print trace of the fibonacci example, instructions only

```
go run snapshot.go
```

### Print trace of the fibonacci example, instructions + execution state

```
go run snapshot.go -all
```

### Run the `cat` example (to test the WASI interface)

```
go run snapshot.go -program cat -quiet
```

### Run the `transcoding` example

```
go run snapshot.go -program transcoding -quiet
```

### Snapshot server experiment

Use keys 'r' for a request to the server and keys 's' for a snapshot.

First, run the program from an initial state:

```bash
go run snapshot.go -program server -quiet -trap -export 
# now press 'r' a few times
# finally press 's', which snapshots the program into snapshot.bin
```

Then, restore the server from the `snapshot.bin` file and see that the counter stays
the same:

```bash
go run snapshot.go -program server -quiet -trap -export -from-snapshot snapshot.bin
# press 'r' a few times and see that the counter is valid.
```


### Ping pong execution between two hosts

```bash
# peer 1
go run snapshot.go -program transcoding -mode remote -port 50051 -peer 50052 -quiet -trap -snapshotS 10 -no-indent -send

# peer 2
go run snapshot.go -program transcoding -mode remote -port 50052 -peer 50051 -quiet -trap -snapshotS 10 -no-indent
```
