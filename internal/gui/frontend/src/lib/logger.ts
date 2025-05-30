/* IMPORTANT: make sure to specify a component whenever you use the logger
component inform from which part of the frontend was a given log emitted from
*/
export enum Lvl {
    TRACE = -1,
    DEBUG = 0,
    INFO = 1,
    WARN = 2,
    ERROR = 3,
    CRITICAL = 4
}

export interface LEntry {
    lvl: Lvl;
    comp: string;
    msg: string;
    ts: number;
    ctx?: Record<string, any>;
    op?: string;
    sid?: string;
    tags?: string[];
    stack?: string;
    _unix_time?: number;  // Unix timestamp in seconds
    time?: string;        // Formatted time string (HH:MM:SS)
}

export interface ThrConf {
    en: boolean;
    int: number;
    maxSimL: number;
    byComp: Record<string, { int: number; maxL: number }>;
    sampInt: number;
}

export interface BatConf {
    en: boolean;
    maxSz: number;
    maxWait: number;
    retries: number;
    retryDelay: number;
}

export interface LogConf {
    minLvl: Lvl;
    bufSz: number;
    thrConf: ThrConf;
    batConf: BatConf;
    conOut: boolean;
    capStack: boolean;
    autoErr: boolean;
    devMode: boolean;
    hiVolCats: Set<string>;
    sampRate: number;
    critPats: RegExp[];
    opTimeout: number;
}

class CircBuf<T> {
    private buf: Array<T | null>;
    private head = 0;
    private tail = 0;
    private cnt = 0;

    constructor(private cap: number) {
        this.buf = new Array(cap).fill(null);
    }

    add(item: T): void {
        this.buf[this.tail] = item;
        this.tail = (this.tail + 1) % this.cap;
        if (this.cnt === this.cap) {
            this.head = (this.head + 1) % this.cap;
        } else {
            this.cnt++;
        }
    }

    getAll(): T[] {
        const res: T[] = [];
        if (this.cnt === 0) return res;
        let curr = this.head;
        for (let i = 0; i < this.cnt; i++) {
            const itm = this.buf[curr];
            if (itm !== null) res.push(itm);
            curr = (curr + 1) % this.cap;
        }
        return res;
    }

    clear(): void {
        this.buf.fill(null);
        this.head = 0;
        this.tail = 0;
        this.cnt = 0;
    }

    get size(): number {
        return this.cnt;
    }
}

export class Logger {
    private _buf: CircBuf<LEntry>;
    private _thrMap: Map<string, { count: number; lastTime: number; samples: string[] }> = new Map();
    private _gCtx: Record<string, any> = {};
    private _opCtxs: Map<string, { context: Record<string, any>; startTime: number; timeoutId?: number }> = new Map();
    private _actOp?: string;
    private _sid: string;
    private _batchMode = false;
    private _batLogs: LEntry[] = [];
    private _batTimer?: number;
    private _timers: Map<string, number> = new Map();
    private _retryQ: Array<{ entry: LEntry; retries: number }> = [];
    private _isProcRetryQ = false;
    private _evtListeners: Array<() => void> = [];

    private _cfg: LogConf = {
        minLvl: Lvl.TRACE,
        bufSz: 500,
        thrConf: {
            en: true,
            int: 60000,
            maxSimL: 5,
            byComp: {
                ui: { int: 30000, maxL: 10 },
                api: { int: 10000, maxL: 3 },
                media: { int: 60000, maxL: 3 }
            },
            sampInt: 10
        },
        batConf: {
            en: true,
            maxSz: 20,
            maxWait: 2000,
            retries: 3,
            retryDelay: 1000
        },
        conOut: true,
        capStack: true,
        autoErr: true,
        devMode: false,
        hiVolCats: new Set(['ui', 'api', 'media', 'network', 'performance']),
        sampRate: 0.01,
        critPats: [/error/i, /fail/i, /exception/i, /crash/i],
        opTimeout: 5 * 60 * 1000
    };

    constructor(customConfig?: Partial<LogConf>) {
        if (customConfig) {
            this._cfg = this._mergeCfg(this._cfg, customConfig);
        }
        if (customConfig?.devMode === undefined) {
            this._cfg.devMode = this._isDevMode();
        }
        this._buf = new CircBuf<LEntry>(this._cfg.bufSz);
        this._sid = this._genSid();
        this._gCtx = {
            userAgent: navigator.userAgent,
            viewport: `${window.innerWidth}x${window.innerHeight}`,
            timestamp: Date.now(),
            sessionId: this._sid
        };
        try {
            const version = this._getAppVer();
            if (version) {
                this._gCtx.appVersion = version;
            }
        } catch (e) { /* Silently ignore */ }
        if (this._cfg.autoErr) {
            this._setupErrLsnr();
        }
        this._startRetryProc();
        const unloadHandler = this._onBeforeUnload.bind(this);
        window.addEventListener('beforeunload', unloadHandler);
        this._evtListeners.push(() => {
            window.removeEventListener('beforeunload', unloadHandler);
        });
        // Log initialization with global context
        console.info('%c[logger]', 'color: #4caf50; font-weight: normal;', 'Logger initialized with global context:', {
            developerMode: this._cfg.devMode,
            minLevel: 'INFO',
            userAgent: this._gCtx.userAgent,
            viewport: this._gCtx.viewport,
            sessionId: this._gCtx.sessionId,
            appVersion: this._gCtx.appVersion || 'dev'
        });
    }

    log(lvl: Lvl, comp: string, msg: string, ctx?: Record<string, any>, op?: string): void {
        if (lvl < this._cfg.minLvl) return;
        if (lvl <= Lvl.DEBUG && this._cfg.hiVolCats.has(comp) && !this._cfg.devMode) {
            if (Math.random() > this._cfg.sampRate) return;
        }
        const thrKey = this._genThrKey(lvl, comp, msg);
        if (this._shouldThr(lvl, comp, thrKey, msg)) {
            return;
        }

        const timestamp = Date.now();

        // Create formatted time string for display (HH:MM:SS)
        const now = new Date();
        const hours = now.getHours().toString().padStart(2, '0');
        const minutes = now.getMinutes().toString().padStart(2, '0');
        const seconds = now.getSeconds().toString().padStart(2, '0');
        const timeString = hours + ':' + minutes + ':' + seconds;

        const e: LEntry = {
            lvl,
            comp,
            msg,
            ts: timestamp,
            ctx: this._buildCtx(ctx),
            op: op || this._actOp,
            sid: this._sid,
            tags: this._deriveTags(lvl, comp, msg),
            // Add fields to match expected log structure
            _unix_time: Math.floor(timestamp / 1000),  // Unix timestamp in seconds
            time: timeString  // Formatted time string for display
        };
        if (this._cfg.capStack && lvl >= Lvl.ERROR) {
            e.stack = this._capStack();
        }
        if (this._batchMode && this._cfg.batConf.en) {
            this._batLogs.push(e);
            if (!this._batTimer && this._cfg.batConf.maxWait > 0) {
                this._batTimer = window.setTimeout(() => {
                    this.flushBatch();
                    this._batTimer = undefined;
                }, this._cfg.batConf.maxWait);
            }
            if (this._batLogs.length >= this._cfg.batConf.maxSz) {
                this.flushBatch();
            }
        } else {
            this._procLEntry(e);
        }
    }

    trace(comp: string, msg: string, ctx?: Record<string, any>, op?: string): void {
        this.log(Lvl.TRACE, comp, msg, ctx, op);
    }

    debug(comp: string, msg: string, ctx?: Record<string, any>, op?: string): void {
        this.log(Lvl.DEBUG, comp, msg, ctx, op);
    }

    info(comp: string, msg: string, ctx?: Record<string, any>, op?: string): void {
        this.log(Lvl.INFO, comp, msg, ctx, op);
    }

    warn(comp: string, msg: string, ctx?: Record<string, any>, op?: string): void {
        this.log(Lvl.WARN, comp, msg, ctx, op);
    }

    error(comp: string, msg: string, ctx?: Record<string, any>, op?: string): void {
        this.log(Lvl.ERROR, comp, msg, ctx, op);
    }

    critical(comp: string, msg: string, ctx?: Record<string, any>, op?: string): void {
        this.log(Lvl.CRITICAL, comp, msg, ctx, op);
    }

    logErr(err: Error, comp: string, msg?: string, ctx?: Record<string, any>): void {
        const errorMsg = msg || `Error: ${err.message}`;
        const errorCtx = {
            ...ctx,
            errorType: err.name,
            errorMessage: err.message,
            stack: err.stack
        };
        this.log(Lvl.ERROR, comp, errorMsg, errorCtx);
    }

    setGCtx(context: Record<string, any>): void {
        this._gCtx = { ...this._gCtx, ...context };
    }

    startOp(name: string, context?: Record<string, any>, timeout?: number): void {
        if (this._actOp) {
            this.endOp({ status: 'interrupted', reason: 'New operation started' });
        }
        this._actOp = name;
        const existing = this._opCtxs.get(name);
        if (existing?.timeoutId) {
            window.clearTimeout(existing.timeoutId);
        }
        const actualTimeout = timeout ?? this._cfg.opTimeout;
        let timeoutId: number | undefined;
        if (actualTimeout > 0) {
            timeoutId = window.setTimeout(() => {
                if (this._actOp === name) {
                    this.warn('operations', `Operation timed out: ${name}`, {
                        timeoutMs: actualTimeout
                    });
                    this.endOp({ status: 'timeout', timeoutMs: actualTimeout });
                } else {
                    this._opCtxs.delete(name);
                }
            }, actualTimeout);
        }
        this._opCtxs.set(name, {
            context: context || {},
            startTime: Date.now(),
            timeoutId
        });
        this.info('operations', `Operation started: ${name}`, context);
    }

    endOp(result?: string | Record<string, any>): void {
        if (!this._actOp) return;
        const name = this._actOp;
        const opData = this._opCtxs.get(name);
        if (opData?.timeoutId) {
            window.clearTimeout(opData.timeoutId);
        }
        const duration = opData ? Date.now() - opData.startTime : undefined;
        const context = typeof result === 'string'
            ? { result, durationMs: duration }
            : { ...(result || {}), durationMs: duration };
        this.info('operations', `Operation completed: ${name}`, context);
        this._opCtxs.delete(name);
        this._actOp = undefined;
    }

    startTimer(name: string, component?: string): void {
        const start = performance.now();
        this._timers.set(name, start);
        if (component) {
            this.trace(component, `Timer started: ${name}`);
        }
    }

    endTimer(name: string, component?: string, logLevel: Lvl = Lvl.DEBUG): number {
        const start = this._timers.get(name);
        if (start === undefined) {
            this.warn('performance', `Timer "${name}" was never started`);
            return 0;
        }
        const end = performance.now();
        const duration = end - start;
        this._timers.delete(name);
        if (component) {
            this.log(logLevel, component, `Timer ${name}: ${duration.toFixed(2)}ms`, {
                duration,
                timerName: name
            });
        }
        return duration;
    }

    trackAction(action: string, details?: Record<string, any>): void {
        this.info('user', `User action: ${action}`, details);
    }

    private _filterGlobalCtx(ctx?: Record<string, any>): Record<string, any> | undefined {
        if (!ctx) return undefined;
        
        // Filter out global context fields
        const globalKeys = new Set(['userAgent', 'viewport', 'timestamp', 'sessionId', 'appVersion']);
        const filtered: Record<string, any> = {};
        
        for (const [key, value] of Object.entries(ctx)) {
            if (!globalKeys.has(key)) {
                filtered[key] = value;
            }
        }
        
        return Object.keys(filtered).length > 0 ? filtered : undefined;
    }

    beginBatch(): void {
        if (this._batLogs.length > 0) {
            this.flushBatch();
        }
        this._batchMode = true;
        this._batLogs = [];
    }

    endBatch(flush = true): void {
        this._batchMode = false;
        if (flush && this._batLogs.length > 0) {
            this.flushBatch();
        }
        if (this._batTimer) {
            window.clearTimeout(this._batTimer);
            this._batTimer = undefined;
        }
    }

    flushBatch(): void {
        if (this._batLogs.length === 0) return;
        const batch = [...this._batLogs];
        this._batLogs = [];
        if (this._batTimer) {
            window.clearTimeout(this._batTimer);
            this._batTimer = undefined;
        }
        for (const e of batch) {
            // ONLY store non-TRACE logs to prevent TRACE logs from burdening the UI/log store
            if (e.lvl > Lvl.TRACE) {
                 this._buf.add(e);
            }
            if (this._cfg.conOut) {
                this._conOut(e);
            }
        }
        this._relayBatchBE(batch);
    }

    clearLogs(): void {
        this._buf.clear();
        this._thrMap.clear();
        this.info('logger', 'Logs cleared');
    }

    getAllLogs(): LEntry[] {
        return this._buf.getAll();
    }

    setMinLvl(level: Lvl): void {
        this._cfg.minLvl = level;
        this.info('logger', `Log level set to: ${this.getLvlName(level)}`);
    }

    getLvlName(level: Lvl): string {
        switch (level) {
            case Lvl.TRACE: return 'TRACE';
            case Lvl.DEBUG: return 'DEBUG';
            case Lvl.INFO: return 'INFO';
            case Lvl.WARN: return 'WARN';
            case Lvl.ERROR: return 'ERROR';
            case Lvl.CRITICAL: return 'CRITICAL';
            default: return 'UNKNOWN';
        }
    }

    destroy(): void {
        for (const [, data] of this._opCtxs.entries()) {
            if (data.timeoutId) {
                window.clearTimeout(data.timeoutId);
            }
        }
        if (this._batTimer) {
            window.clearTimeout(this._batTimer);
        }
        if (this._batLogs.length > 0) {
            this.flushBatch();
        }
        for (const cleanup of this._evtListeners) {
            cleanup();
        }
        this.info('logger', 'Logger destroyed');
    }

    private _procLEntry(e: LEntry): void {
        // ONLY store non-TRACE logs to prevent TRACE logs from burdening the UI/log store
        if (e.lvl > Lvl.TRACE) {
            this._buf.add(e);
        }
        if (this._cfg.conOut) {
            this._conOut(e);
        }
        this._relayBE(e);
    }

    private _buildCtx(localContext?: Record<string, any>): Record<string, any> {
        const context: Record<string, any> = {};
        
        // Only add operation context if present
        if (this._actOp) {
            const opData = this._opCtxs.get(this._actOp);
            if (opData?.context) {
                Object.assign(context, opData.context);
                context.operationElapsedMs = Date.now() - opData.startTime;
            }
        }
        
        // Add local context if provided
        if (localContext) {
            Object.assign(context, localContext);
        }
        
        // Return undefined if no context to avoid sending empty objects
        return Object.keys(context).length > 0 ? context : {};
    }

    private _conOut(e: LEntry): void {
        if (!this._cfg.devMode && e.lvl <= Lvl.DEBUG) {
            return;
        }
        const pfx = `[${e.comp}]`;
        // Filter out global context fields
        const ctx = this._filterGlobalCtx(e.ctx);
        let mth = 'log';
        let stl = '';
        switch (e.lvl) {
            case Lvl.TRACE: mth = 'debug'; stl = 'color: #8c84e8; font-weight: normal;'; break;
            case Lvl.DEBUG: mth = 'debug'; stl = 'color: #84a9e8; font-weight: normal;'; break;
            case Lvl.INFO: mth = 'info'; stl = 'color: #4caf50; font-weight: normal;'; break;
            case Lvl.WARN: mth = 'warn'; stl = 'color: #ff9800; font-weight: bold;'; break;
            case Lvl.ERROR: mth = 'error'; stl = 'color: #f44336; font-weight: bold;'; break;
            case Lvl.CRITICAL: mth = 'error'; stl = 'color: #b71c1c; font-weight: bold; font-size: 1.1em;'; break;
        }
        if (ctx && Object.keys(ctx).length > 0) {
            console[mth](`%c${pfx}`, stl, e.msg, ctx);
        } else {
            console[mth](`%c${pfx}`, stl, e.msg);
        }
        if (e.stack && e.lvl >= Lvl.ERROR) {
            console.groupCollapsed('Stack trace');
            console.error(e.stack);
            console.groupEnd();
        }
    }

    private _relayBE(e: LEntry): void {
        try {
            const eCopy = { ...e };
            if (eCopy.ctx) {
                eCopy.ctx = this._sanitizeCtx(eCopy.ctx);
            }
            // Send individual log to BackendLogger
            (window as any).go.gui.App.BackendLogger(e.comp, JSON.stringify(eCopy));
        } catch (err) {
            console.error("Failed to relay log to backend:", err);
            if (e.lvl >= Lvl.ERROR) {
                this._retryQ.push({ entry: e, retries: 0 });
            } else if (navigator.sendBeacon) {
                try {
                    const beacon = new Blob([JSON.stringify({ component: e.comp, log: e })], { type: 'application/json' });
                    navigator.sendBeacon('/api/logs', beacon);
                } catch (beaconErr) {
                    console.error("Failed to send log via beacon:", beaconErr);
                }
            }
        }
    }

    private _relayBatchBE(entries: LEntry[]): void {
        if (entries.length === 0) return;
        try {
            const component = entries[0].comp;
            const sanEntries = entries.map(e => {
                const copy = { ...e };
                if (copy.ctx) {
                    copy.ctx = this._sanitizeCtx(copy.ctx);
                }
                return copy;
            });
            // This function exists in app.go (line 153)
            (window as any).go.gui.App.BackendLoggerBatch(component, JSON.stringify(sanEntries));
        } catch (err) {
            console.error("Failed to relay batch to backend:", err);
            // Try to send logs individually
            try {
                for (const e of sanEntries) {
                    (window as any).go.gui.App.BackendLogger(e.comp || component, JSON.stringify(e));
                }
            } catch (individualErr) {
                console.error("Failed individual log fallback:", individualErr);
                // Queue error logs for retry
                for (const e of entries) {
                    if (e.lvl >= Lvl.ERROR) {
                        this._retryQ.push({ entry: e, retries: 0 });
                    }
                }
            }

            if (navigator.sendBeacon) {
                try {
                    const beacon = new Blob([JSON.stringify({ batch: entries })], { type: 'application/json' });
                    navigator.sendBeacon('/api/logs/batch', beacon);
                } catch (beaconErr) { /* Already logged */ }
            }
        }
    }

    private _startRetryProc(): void {
        const procQ = async () => {
            if (this._isProcRetryQ || this._retryQ.length === 0) return;
            this._isProcRetryQ = true;
            try {
                const item = this._retryQ.shift();
                if (!item) {
                    this._isProcRetryQ = false;
                    return;
                }
                const { entry: e, retries: r } = item;
                if (r < this._cfg.batConf.retries) {
                    try {
                        // Use BackendLogger for individual log
                        (window as any).go.gui.App.BackendLogger(e.comp, JSON.stringify(e));
                    } catch (err) {
                        this._retryQ.push({ entry: e, retries: r + 1 });
                        await new Promise(resolve => setTimeout(resolve, this._cfg.batConf.retryDelay));
                    }
                } else {
                    if (navigator.sendBeacon) {
                        try {
                            const beacon = new Blob([JSON.stringify({ component: e.comp, log: e })], { type: 'application/json' });
                            navigator.sendBeacon('/api/logs', beacon);
                        } catch (beaconErr) { /* Give up */ }
                    }
                }
            } finally {
                this._isProcRetryQ = false;
                if (this._retryQ.length > 0) {
                    setTimeout(procQ, 10);
                }
            }
        };
        setInterval(procQ, 5000);
    }

    private _isDevMode(): boolean {
        return (
            (window as any).__LANGKIT_VERSION === 'dev' ||
            window.location.hostname === 'localhost' ||
            window.location.hostname === '127.0.0.1' ||
            ['3000', '8080', '5173'].includes(window.location.port) ||
            window.location.href.includes('wails.localhost') ||
            new URLSearchParams(window.location.search).has('dev')
        );
    }

    private _getAppVer(): string | null {
        return (window as any).__LANGKIT_VERSION ||
            (window as any).appVersion ||
            document.querySelector('meta[name="app-version"]')?.getAttribute('content') ||
            null;
    }

    private _genSid(): string {
        return Date.now().toString(36) + Math.random().toString(36).substring(2, 9);
    }

    private _genThrKey(lvl: Lvl, comp: string, msg: string): string {
        const normalized = msg
            .replace(/[0-9a-f]{8}(-[0-9a-f]{4}){3}-[0-9a-f]{12}/gi, '[UUID]')
            .replace(/\b\d{3,}\b/g, '[NUM]')
            .replace(/\d{1,2}:\d{2}(:\d{2})?(\.\d+)?/g, '[TIME]')
            .replace(/\d{4}-\d{2}-\d{2}/g, '[DATE]')
            .replace(/(https?:\/\/[^\s]+)/g, '[URL]')
            .replace(/([\\\/][\w\-. ]+)+/g, '[PATH]')
            .trim();
        const signature = normalized.length > 40 ? normalized.substring(0, 40) : normalized;
        return `${lvl}:${comp}:${signature}`;
    }

    private _shouldThr(lvl: Lvl, comp: string, thrKey: string, msg: string): boolean {
        if (lvl >= Lvl.WARN) return false;
        if (this._cfg.critPats.some(pattern => pattern.test(msg))) return false;
        if (!this._cfg.thrConf.en) return false;

        const now = Date.now();
        const thrInfo = this._thrMap.get(thrKey);
        const compConf = this._cfg.thrConf.byComp[comp];
        const thrInt = compConf?.int || this._cfg.thrConf.int;
        const maxL = compConf?.maxL || this._cfg.thrConf.maxSimL;
        const effMaxL = lvl === Lvl.TRACE ? Math.max(1, Math.floor(maxL / 3)) : maxL;

        if (thrInfo) {
            if (now - thrInfo.lastTime < thrInt) {
                if (thrInfo.count % this._cfg.thrConf.sampInt === 0 && thrInfo.samples.length < 3) {
                    thrInfo.samples.push(msg);
                }
                thrInfo.count++;
                this._thrMap.set(thrKey, thrInfo);
                return thrInfo.count > effMaxL;
            } else {
                if (thrInfo.count > effMaxL) {
                    const samplesText = thrInfo.samples.length > 0 ? ` Examples: ${thrInfo.samples.join(" | ")}` : '';
                    this._procLEntry({
                        lvl,
                        comp,
                        msg: `${msg} (${thrInfo.count} similar messages throttled in last ${Math.round(thrInt / 1000)}s)${samplesText}`,
                        ts: now,
                        ctx: { throttled: true, count: thrInfo.count },
                        sid: this._sid,
                        tags: ['throttled']
                    });
                }
                this._thrMap.set(thrKey, { count: 1, lastTime: now, samples: [] });
                return false;
            }
        } else {
            this._thrMap.set(thrKey, { count: 1, lastTime: now, samples: [] });
            return false;
        }
    }

    private _capStack(): string {
        return new Error().stack || '';
    }

    private _deriveTags(lvl: Lvl, comp: string, msg: string): string[] {
        const tags: string[] = [comp];
        switch (lvl) {
            case Lvl.TRACE: tags.push('trace'); break;
            case Lvl.DEBUG: tags.push('debug'); break;
            case Lvl.INFO: tags.push('info'); break;
            case Lvl.WARN: tags.push('warning'); break;
            case Lvl.ERROR: tags.push('error'); break;
            case Lvl.CRITICAL: tags.push('critical'); break;
        }
        if (this._actOp) {
            tags.push(`op:${this._actOp}`);
        }
        if (lvl >= Lvl.ERROR) {
            tags.push('error');
        }
        if (msg.toLowerCase().includes('performance') || msg.toLowerCase().includes('timer') || comp === 'performance') {
            tags.push('performance');
        }
        return tags;
    }

    private _setupErrLsnr(): void {
        const errHndlr = (event: ErrorEvent) => {
            this.logErr(
                event.error || new Error(event.message),
                'window',
                'Unhandled error',
                { source: event.filename, line: event.lineno, column: event.colno }
            );
        };
        const rejHndlr = (event: PromiseRejectionEvent) => {
            const error = event.reason instanceof Error ? event.reason : new Error(String(event.reason));
            this.logErr(error, 'promise', 'Unhandled promise rejection', { reason: String(event.reason) });
        };
        window.addEventListener('error', errHndlr);
        window.addEventListener('unhandledrejection', rejHndlr);
        this._evtListeners.push(() => {
            window.removeEventListener('error', errHndlr);
            window.removeEventListener('unhandledrejection', rejHndlr);
        });
    }

    private _onBeforeUnload(e: BeforeUnloadEvent): void {
        if (this._batLogs.length > 0) {
            this.flushBatch();
        }
        if (this._retryQ.length > 0 && navigator.sendBeacon) {
            try {
                const criticalRetries = this._retryQ
                    .filter(item => item.entry.lvl >= Lvl.ERROR)
                    .map(item => item.entry);
                if (criticalRetries.length > 0) {
                    const beacon = new Blob([JSON.stringify({ batch: criticalRetries })], { type: 'application/json' });
                    navigator.sendBeacon('/api/logs/batch', beacon);
                }
            } catch (err) { /* Ignore errors during unload */ }
        }
    }

    private _sanitizeCtx(context: Record<string, any>): Record<string, any> {
        const result: Record<string, any> = {};
        const seen = new WeakMap();
        const sVal = (value: any, depth = 0): any => {
            if (depth > 5) return '[MAX_DEPTH]';
            if (value === null || value === undefined) return value;
            if (typeof value !== 'object' && typeof value !== 'function') return value;
            if (typeof value === 'function') return '[FUNCTION]';
            if (value instanceof Object) {
                if (seen.has(value)) return '[CIRCULAR]';
                seen.set(value, true);
            }
            if (Array.isArray(value)) {
                return value.map(item => sVal(item, depth + 1));
            }
            if (value instanceof Node) return value.nodeName || '[DOM_NODE]';
            try {
                const obj: Record<string, any> = {};
                const entries = Object.entries(value).slice(0, 20);
                for (const [key, val] of entries) {
                    if (typeof val === 'function' || typeof key === 'symbol') continue;
                    obj[key] = sVal(val, depth + 1);
                }
                if (Object.keys(value).length > 20) {
                    obj['...'] = `[${Object.keys(value).length - 20} more properties]`;
                }
                return obj;
            } catch (e) {
                return '[UNSERIALIZABLE]';
            }
        };
        for (const [key, value] of Object.entries(context)) {
            try {
                result[key] = sVal(value);
            } catch (e) {
                result[key] = '[ERROR_SERIALIZING]';
            }
        }
        return result;
    }

    private _mergeCfg(defCfg: LogConf, custCfg: Partial<LogConf>): LogConf {
        const result = { ...defCfg };
        for (const key in custCfg) {
            if (key === 'thrConf' && custCfg.thrConf) {
                result.thrConf = {
                    ...result.thrConf,
                    ...custCfg.thrConf,
                    byComp: {
                        ...result.thrConf.byComp,
                        ...(custCfg.thrConf.byComp || {})
                    }
                };
            } else if (key === 'batConf' && custCfg.batConf) {
                result.batConf = { ...result.batConf, ...custCfg.batConf };
            } else if (key === 'hiVolCats' && custCfg.hiVolCats) {
                result.hiVolCats = new Set([...result.hiVolCats, ...custCfg.hiVolCats]);
            } else if (key === 'critPats' && Array.isArray(custCfg.critPats)) {
                result.critPats = [...custCfg.critPats];
            } else {
                (result as any)[key] = (custCfg as any)[key];
            }
        }
        return result;
    }
}

export const logger = new Logger();