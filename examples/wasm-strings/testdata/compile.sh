# for custom entry
#/usr/local/wasi-sdk/bin/clang -Wl,--no-entry -Wl,--export=malloc -o program.wasm program.c

# for POSIX arg passing style
/usr/local/wasi-sdk/bin/clang -Wl,--export=malloc -o program.wasm program.c