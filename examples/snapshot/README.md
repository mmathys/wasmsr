## Sample commands

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