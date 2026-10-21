async function run() {
    let code = 0;
    try {
        go = new Go();
        go.argv = goargs;
        // wasm_exec.js only logs a non-zero exit; keep it for the caller
        go.exit = (c) => {
            code = c;
            if (c !== 0) {
                console.warn("exit code:", c);
            }
        };

        result = await WebAssembly.instantiateStreaming(fetch("binary.wasm"), go.importObject);
        await go.run(result.instance);
    } catch (error) {
        console.error(error.message);
        return 1;
    }
    return code;
}
