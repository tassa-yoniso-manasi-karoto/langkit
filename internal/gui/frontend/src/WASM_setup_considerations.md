**The WebAssembly Frontier: Navigating Integration Complexities in Modern Web Applications with Vite and Wails v2**

**tl;dr:**

*   **Objective:** Achieve robust, fully functional WebAssembly (WASM) integration (Rust/`wasm-bindgen`) in a Wails v2 + Vite frontend, ensuring accessible browser WASM APIs (especially `WebAssembly.Memory`).
*   **Core Challenge in `wails dev`:** Wails v2 proxies Vite dev server assets but re-serves them under `wails://`, causing `import.meta.url` in JS to be `wails://`-based. When `wasm-bindgen`'s JS glue fetches its `.wasm` binary via this `wails://` path, Wails fails to provide the `Content-Type: application/wasm` header to the webview, breaking `instantiateStreaming` and subsequent API access from Rust.
*   **Recommended Solution: Clean Targeted WASM Inlining.**
    1.  Build WASM with `wasm-pack --target web`.
    2.  Use a Node.js script to create a self-contained, inlined JS glue file:
        *   Embeds Base64 WASM binary as `WASM_BINARY_BUFFER`.
        *   **Surgically modifies the original `wasm-bindgen` JS glue's `async function __wbg_init(module_or_path)` (default export) to use `WASM_BINARY_BUFFER` instead of fetching, while ensuring original `__wbg_load` and `__wbg_finalize_init` are correctly called with this buffer.**
        *   This modified JS glue is placed in `frontend/src/wasm-generated/pkg/`.
    3.  `frontend/src/lib/wasm.ts` statically imports this inlined JS glue.
    4.  `vite.config.ts` includes `vite-plugin-top-level-await` and `optimizeDeps.exclude`.
*   **Rationale:** Bypasses the `wails://` `Content-Type` issue for the `.wasm` binary. The current focus is perfecting the `__wbg_init` modification for correct `wasm-bindgen` internal state linkage.
*   **Prototyping with `wails://`:** Loading the *original* `wasm-bindgen` JS glue via `wails://` (e.g., from `/public`) *can* execute WASM code for quick prototyping if immediate browser API access from Rust isn't critical, but it **will suffer from the `Content-Type` issue and broken APIs for memory introspection.**
*   **Other Explored (Less Ideal) Paths:**
    *   Standard Vite `src/` asset handling (failed due to `wails://` `import.meta.url` issue leading to `Content-Type` problem).
    *   Complex inlining via shell scripts with global overrides (fragile, caused new linkage issues).
    *   Wails `AssetServer.Middleware` or custom Go `Handler` (theoretically plausible for `Content-Type` but doesn't fix the `wails://` origin for JS and might not overcome deeper WebKit custom scheme header issues).

---

**1. Introduction**

WebAssembly (WASM) offers significant performance benefits for web applications. However, integrating WASM modules, particularly those generated from Rust with `wasm-bindgen`, into projects using modern frontend tools like Vite and desktop frameworks like Wails v2, presents unique challenges. This document archives the investigation into these complexities, detailing attempted solutions, outcomes, and current best-practice recommendations for Langkit. The primary goal is a reliable WASM integration with full browser API functionality.

**2. The Core Challenge: Asset Serving and `Content-Type` in `wails dev`**

The central difficulty in the Wails v2 + Vite development environment (`wails dev`) lies in how frontend assets are served and how this impacts WASM loading:

*   **Wails v2 Dev Server Proxying:**
    *   When `frontend:dev:serverUrl` in `wails.json` points to Vite's HTTP dev server, Wails acts as a proxy.
    *   Requests from the Wails webview are first handled by Wails's `AssetHandler`. This handler fetches the content from Vite's HTTP URL.
    *   Crucially, Wails then re-serves this content to the webview under its internal `wails://` URL scheme (e.g., `wails://wails.localhost:34115/src/lib/your-script.js`).
    *   This results in `import.meta.url` within any JavaScript module loaded into the webview evaluating to a `wails://`-based URI.

*   **The `.wasm` Binary Fetch Problem:**
    *   If `wasm-bindgen`'s JS glue (e.g., `log_engine.js`) is loaded into this `wails://` context, and it attempts to fetch its companion `.wasm` binary (e.g., `log_engine_bg.wasm`) using a relative path (e.g., `new URL('./log_engine_bg.wasm', import.meta.url)`), the resolved URL for the `.wasm` file also becomes a `wails://` URI.
    *   Wails's `AssetHandler` proxies this `wails://` request for the `.wasm` file to Vite. Vite correctly serves the `.wasm` binary with `Content-Type: application/wasm` *back to Wails's proxy*.
    *   **The Failure Point:** When Wails's `AssetHandler` then serves this proxied `.wasm` content back to the webview under the `wails://` scheme, it **fails to propagate or set the `Content-Type: application/wasm` header.** This has been confirmed by "No response headers" in WebKit Network DevTools for the `wails://`-served `.wasm` file.

*   **Consequences:**
    *   The `wasm-bindgen` JS glue logs a warning: "`WebAssembly.instantiateStreaming` failed because your server does not serve Wasm with `application/wasm` MIME type."
    *   It falls back to `WebAssembly.instantiate(ArrayBuffer)`.
    *   This leads to an incompletely initialized WASM instance where `wasm_bindgen::memory()` in Rust cannot access the `WebAssembly.Memory` object, rendering memory-related APIs non-functional from Rust's perspective.

**3. Explored Solutions, Outcomes, and Current Strategy**

Based on these challenges, several approaches have been investigated:

*   **3.1. Standard Vite Asset Handling (No Inlining, Vite Plugins)**
    *   **Attempt:**
        1.  Place `wasm-pack --target web` output (JS glue and `.wasm` binary) into `frontend/src/wasm-generated/pkg/`.
        2.  Use a static ES `import` in `frontend/src/lib/wasm.ts` for the JS glue.
        3.  Configure `vite.config.ts` with `vite-plugin-wasm`, `vite-plugin-top-level-await`, and `optimizeDeps.exclude`.
    *   **Outcome:** Failed. Due to Wails proxying (as described in Section 2), the JS glue and subsequently the `.wasm` binary were still effectively requested via `wails://` by the webview, leading to the `Content-Type` issue for the `.wasm` file. Vite plugins could not affect the final `wails://` response headers from Wails to the webview.

*   **3.2. Manual WASM Inlining with "Failsafe" Shell Script (Overriding Global Browser Functions)**
    *   **Attempt:** A complex `wasm-inline.sh` script Base64-encoded the `.wasm` binary, prepended it to the JS glue, and *also* prepended a self-executing function that overrode global `window.fetch`, `WebAssembly.instantiate`, and `WebAssembly.instantiateStreaming`. This was intended to force the use of the inlined buffer and perform a JS-side API check (`get_memory_api_access_status`).
    *   **Outcome:**
        *   Successfully bypassed the network fetch for the `.wasm` binary.
        *   A JS-side check (`get_memory_api_access_status`) reported `instance.exports.memory` as accessible immediately after the overridden `instantiate` call.
        *   However, subsequent calls from `wasm.ts` to Rust functions (like `get_memory_usage`) *still* indicated that `wasm_bindgen::memory()` was non-functional from Rust's perspective.
    *   **Conclusion:** While forcing buffer use worked at a low JS level, the global overrides likely interfered with `wasm-bindgen`'s complete internal state finalization needed for Rust-side shims. This approach was also deemed too fragile.

*   **3.3. Wails `AssetServer.Middleware` to Set `Content-Type`**
    *   **Proposed Idea:** Implement a Go middleware in Wails to explicitly set `Content-Type: application/wasm` for `.wasm` paths before Wails serves the (proxied) content to the webview.
    *   **Status:** Untested for this specific WASM API access issue.
    *   **Assessment (Likely Ineffective for Full API Functionality):** While this Wails-idiomatic approach *would* correctly set the `Content-Type` header on the Go `http.ResponseWriter` within Wails's internal pipeline, it is **unlikely to resolve the WASM API inaccessibility problem.** The core issue appears to be how WebKitGTK (when handling `wails://` scheme responses) processes or recognizes these headers for `application/wasm`, or how Wails's CGO bridge translates these Go-set headers to the native webview response. If the WebKit engine or the Wails-to-WebKit interface is the point where `Content-Type` recognition fails for custom schemes for WASM instantiation purposes, merely setting the header on the Go side via middleware will not suffice to enable full API access (like `wasm_bindgen::memory()`). The "No response headers" observation in WebKit devtools for `wails://` WASM requests supports this concern. This approach is therefore not currently pursued as a primary solution for the API access problem.

*   **3.4. Current Recommended Strategy: Clean, Targeted WASM Inlining with Surgical JS Glue Modification**
    *   **Build Process (`scripts/build-wasm.sh`):**
        1.  Execute `wasm-pack build --target web --out-dir temp_pkg --out-name log_engine --release` (to get original `log_engine.js` and `log_engine_bg.wasm`).
        2.  Utilize a **dedicated Node.js script** (e.g., `scripts/create-inlined-glue.js`) to:
            *   Read `temp_pkg/log_engine_bg.wasm` and Base64 encode it.
            *   Read the *entire original content* of `temp_pkg/log_engine.js`.
            *   Construct a *new, self-contained JS file* (target: `frontend/src/wasm-generated/pkg/log_engine.js`). This file will contain:
                *   The `const WASM_BINARY_BASE64 = "...";` and `const WASM_BINARY_BUFFER = base64ToArrayBuffer(...);` definitions prepended to the original JS glue content.
                *   A **surgically modified version of the original `async function __wbg_init(module_or_path)`** (which is the default export of `log_engine.js`). This modification must ensure that `module_or_path` is unconditionally set to `WASM_BINARY_BUFFER` *before* `wasm-bindgen`'s internal helper `__wbg_get_imports()` is called and `await __wbg_load(...)` is executed. The original `__wbg_load` and `__wbg_finalize_init` functions must then be allowed to execute with this buffer, so `wasm-bindgen` can complete its internal setup correctly.
        3.  Place `build-info.json` (containing `wasmSizeBytes` of the *original* `.wasm` file) in `frontend/public/wasm/`.
    *   **Frontend Code (`lib/wasm.ts`):**
        1.  Statically import the modified, inlined `log_engine.js`:
            `import initInlinedWasm, * as wasmGeneratedExports from '../wasm-generated/pkg/log_engine.js';`
        2.  Call `await initInlinedWasm();` to initialize.
        3.  Assign to `wasmModule` using `wasmGeneratedExports`.
        4.  Perform the API access check using `wasmModule.get_memory_usage()` (which calls the Rust function) and verify `has_browser_api_access: true` and `total_bytes > 0`.
    *   **Vite Configuration (`vite.config.ts`):**
        *   Retain `vite-plugin-top-level-await`.
        *   Retain `optimizeDeps.exclude: ['log-engine']` (using the name from `pkg/package.json`).
        *   `vite-plugin-wasm` becomes less critical for loading *this specific* inlined module but is good for general project WASM support.
    *   **Rationale & Current Hurdle:** This approach directly bypasses the `wails://` network fetch for the `.wasm` binary, thus solving the `Content-Type` problem at its source for the dev environment. The remaining critical challenge is perfecting the surgical modification of `__wbg_init` within the Node.js inlining script to ensure that `wasm-bindgen`'s `__wbg_finalize_init` function correctly links its internal `wasm.memory` reference for use by `wasm_bindgen::memory()` in Rust. The current API access failures indicate this final linkage step is not yet correct with the inlining attempts made so far.

**4. Prototyping with `wails://` (Limited Use Case)**

For very quick, initial prototyping where full Rust-side memory API access is *not an immediate requirement*, developers *can* load the *original, unmodified* `wasm-pack` JS glue via `wails://` (e.g., by placing the `pkg/` output in `public/wasm/` and using `getEnvironmentOptimizedPaths` to load `wails://.../log_engine.js`). The WASM code will likely execute. However, be aware that this setup **will lead to non-functional WASM APIs for memory introspection from Rust** due to the `Content-Type` issue when `log_engine.js` fetches its `.wasm` binary via `wails://`. This path should be considered strictly for temporary prototyping and abandoned once API access is needed.

**5. Conclusion and Path Forward**

The interaction between Wails v2's development mode asset proxying (which results in `wails://` origins for `src/` JS) and WebKit's handling of `Content-Type` for custom schemes is the primary blocker for standard WASM loading.

The most promising path to a robust solution that works in `wails dev` and provides full API access is **Clean, Targeted Inlining (3.4)**.
