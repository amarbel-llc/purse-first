// dagnabit init-smoke STRICT wasm loader harness (purse-first#180 / FDR 0014).
//
// Runs a GOOS=js GOARCH=wasm test binary under Go's GENERIC wasm_exec.js, whose
// stub filesystem fails every operation with ENOSYS. This reproduces the
// browser filesystem strictness that CATCHES init-time filesystem hazards like
// a package init() opening /dev/null (purse-first#177) — the same class papi#62
// hit. It deliberately does NOT wire a real filesystem: Go's wasm_exec_node.js
// would supply one and open /dev/null successfully, MASKING the bug. Do not
// "simplify" this into requiring node's fs onto globalThis.fs.
//
// Invocation (constructed by `dagnabit init-smoke run`):
//   env -i PATH=<bun>:<coreutils> DAGNABIT_WASM_EXEC=<goroot>/lib/wasm/wasm_exec.js \
//     bun <this-file> <test-binary.wasm> [test flags...]
//
// The harness reads the test binary with its OWN (bun) filesystem; the wasm
// MODULE only ever sees the generic shim's stub fs.

"use strict";

const fs = require("node:fs");

const wasmExecPath = process.env.DAGNABIT_WASM_EXEC;
if (!wasmExecPath) {
  console.error("dagnabit init-smoke: DAGNABIT_WASM_EXEC is not set");
  process.exit(2);
}

const wasmPath = process.argv[2];
if (!wasmPath) {
  console.error("dagnabit init-smoke: no wasm test binary argument");
  process.exit(2);
}

// Load Go's generic wasm_exec.js. Because globalThis.fs is left undefined, the
// generic shim installs its own stub filesystem (writeSync to stdout/stderr,
// ENOSYS for everything else) — the strict behavior we want. Indirect eval runs
// it in the global scope so it can define globalThis.Go.
const wasmExecSrc = fs.readFileSync(wasmExecPath, "utf8");
(0, eval)(wasmExecSrc);

if (typeof globalThis.Go !== "function") {
  console.error(
    "dagnabit init-smoke: " + wasmExecPath + " did not define globalThis.Go",
  );
  process.exit(2);
}

const go = new globalThis.Go();

// The generic (browser) shim's exit handler only console.warns the code; it does
// NOT propagate to the host process (browsers cannot exit). Capture the code so
// a failing test (os.Exit(1)) or an init panic (exit 2) becomes our exit status,
// which is what `go test -exec` keys off of.
let exitCode = 0;
go.exit = (code) => {
  exitCode = code;
};

// argv[0] is the program name; the rest are `go test` flags forwarded by -exec.
go.argv = ["initsmoke.test"].concat(process.argv.slice(3));

const wasmBytes = fs.readFileSync(wasmPath);

WebAssembly.instantiate(wasmBytes, go.importObject)
  .then((result) => go.run(result.instance))
  .then(() => process.exit(exitCode))
  .catch((err) => {
    // An instantiation/trap failure (or a panic that surfaces as a JS throw)
    // is a run failure — surface it and fail nonzero.
    console.error(err && err.stack ? err.stack : String(err));
    process.exit(1);
  });
