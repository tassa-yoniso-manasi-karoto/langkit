/**
 * Graphics/WebGL/WebGPU capability detection for debug reports.
 * Works across all WebView engines (WebView2, WebKit, QtWebEngine, browsers).
 */

export interface GraphicsInfo {
    // WebGL
    webglAvailable: boolean;
    webgl2Available: boolean;
    renderer: string;
    vendor: string;
    hardwareAccelerated: boolean;
    softwareRenderer: string | null;
    maxTextureSize: number;

    // WebGPU
    webgpuAvailable: boolean;
    webgpuIsFallback: boolean;
    webgpuAdapter: string | null;
}

// Known software renderer signatures
const SOFTWARE_RENDERERS = [
    { pattern: /swiftshader/i, name: "SwiftShader" },
    { pattern: /llvmpipe/i, name: "llvmpipe (Mesa)" },
    { pattern: /microsoft basic render/i, name: "Microsoft Basic Render Driver" },
    { pattern: /software/i, name: "Software Renderer" },
    { pattern: /lavapipe/i, name: "lavapipe (Mesa Vulkan CPU)" },
];

/**
 * Detects WebGL capabilities (synchronous).
 */
function getWebGLInfo(): Partial<GraphicsInfo> {
    const info: Partial<GraphicsInfo> = {
        webglAvailable: false,
        webgl2Available: false,
        renderer: "unknown",
        vendor: "unknown",
        hardwareAccelerated: false,
        softwareRenderer: null,
        maxTextureSize: 0,
    };

    const canvas = document.createElement("canvas");
    canvas.width = 1;
    canvas.height = 1;

    let gl: WebGLRenderingContext | WebGL2RenderingContext | null = null;

    try {
        gl = canvas.getContext("webgl2") as WebGL2RenderingContext | null;
        if (gl) {
            info.webgl2Available = true;
            info.webglAvailable = true;
        }
    } catch (e) {
        // WebGL2 not available
    }

    if (!gl) {
        try {
            gl = canvas.getContext("webgl") as WebGLRenderingContext | null;
            if (gl) {
                info.webglAvailable = true;
            }
        } catch (e) {
            // WebGL1 not available either
        }
    }

    if (!gl) {
        return info;
    }

    const debugInfo = gl.getExtension("WEBGL_debug_renderer_info");

    if (debugInfo) {
        info.renderer = gl.getParameter(debugInfo.UNMASKED_RENDERER_WEBGL) || "unknown";
        info.vendor = gl.getParameter(debugInfo.UNMASKED_VENDOR_WEBGL) || "unknown";
    } else {
        info.renderer = gl.getParameter(gl.RENDERER) || "unknown";
        info.vendor = gl.getParameter(gl.VENDOR) || "unknown";
    }

    info.maxTextureSize = gl.getParameter(gl.MAX_TEXTURE_SIZE) || 0;

    // Check for software rendering
    const rendererLower = (info.renderer || "").toLowerCase();
    for (const { pattern, name } of SOFTWARE_RENDERERS) {
        if (pattern.test(rendererLower)) {
            info.softwareRenderer = name;
            info.hardwareAccelerated = false;
            return info;
        }
    }

    info.hardwareAccelerated = info.renderer !== "unknown";

    return info;
}

/**
 * Detects WebGPU capabilities (async).
 */
async function getWebGPUInfo(): Promise<Partial<GraphicsInfo>> {
    const info: Partial<GraphicsInfo> = {
        webgpuAvailable: false,
        webgpuIsFallback: false,
        webgpuAdapter: null,
    };

    // Check if WebGPU is available
    if (typeof navigator === "undefined" || !("gpu" in navigator)) {
        return info;
    }

    try {
        const gpu = (navigator as any).gpu;
        if (!gpu) {
            return info;
        }

        const adapter = await gpu.requestAdapter();
        if (!adapter) {
            return info;
        }

        info.webgpuAvailable = true;
        info.webgpuIsFallback = adapter.isFallbackAdapter || false;

        // Get adapter info if available
        if (adapter.info) {
            const adapterInfo = adapter.info;
            const parts: string[] = [];
            if (adapterInfo.vendor) parts.push(adapterInfo.vendor);
            if (adapterInfo.architecture) parts.push(adapterInfo.architecture);
            if (adapterInfo.device) parts.push(adapterInfo.device);
            if (adapterInfo.description) parts.push(adapterInfo.description);
            info.webgpuAdapter = parts.length > 0 ? parts.join(" / ") : null;
        }
    } catch (e) {
        // WebGPU not available or error
    }

    return info;
}

/**
 * Detects WebGL and WebGPU capabilities of the current WebView.
 * This is useful for debug reports to understand if hardware acceleration
 * is actually working, regardless of what the host environment reports.
 */
export async function getGraphicsInfo(): Promise<GraphicsInfo> {
    const webglInfo = getWebGLInfo();
    const webgpuInfo = await getWebGPUInfo();

    return {
        webglAvailable: webglInfo.webglAvailable || false,
        webgl2Available: webglInfo.webgl2Available || false,
        renderer: webglInfo.renderer || "unknown",
        vendor: webglInfo.vendor || "unknown",
        hardwareAccelerated: webglInfo.hardwareAccelerated || false,
        softwareRenderer: webglInfo.softwareRenderer || null,
        maxTextureSize: webglInfo.maxTextureSize || 0,
        webgpuAvailable: webgpuInfo.webgpuAvailable || false,
        webgpuIsFallback: webgpuInfo.webgpuIsFallback || false,
        webgpuAdapter: webgpuInfo.webgpuAdapter || null,
    };
}

/**
 * Returns a formatted string suitable for debug reports.
 */
export async function getGraphicsInfoString(): Promise<string> {
    const info = await getGraphicsInfo();

    const lines: string[] = [];
    lines.push("WebGL: " + (info.webgl2Available ? "2.0" : info.webglAvailable ? "1.0" : "unavailable"));
    lines.push("Renderer: " + info.renderer);
    lines.push("Vendor: " + info.vendor);
    lines.push("Hardware Accelerated: " + (info.hardwareAccelerated ? "yes" : "no"));

    if (info.softwareRenderer) {
        lines.push("Software Renderer: " + info.softwareRenderer);
    }

    lines.push("Max Texture Size: " + info.maxTextureSize);

    // WebGPU section
    if (info.webgpuAvailable) {
        const gpuType = info.webgpuIsFallback ? "software fallback" : "hardware";
        lines.push("WebGPU: available (" + gpuType + ")");
        if (info.webgpuAdapter) {
            lines.push("WebGPU Adapter: " + info.webgpuAdapter);
        }
    } else {
        lines.push("WebGPU: unavailable");
    }

    return lines.join("\n");
}

/**
 * Quick check if hardware acceleration appears to be working.
 * Useful for showing warnings in the UI.
 */
export async function isHardwareAccelerated(): Promise<boolean> {
    const info = await getGraphicsInfo();
    return info.hardwareAccelerated;
}
