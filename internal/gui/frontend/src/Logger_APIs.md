# Logger API Documentation

IMPORTANT: make sure to specify a component whenever you use the logger.
component inform from which part of the frontend was a given log emitted from.

IMPORTANT: for log statement inside stores, always prefix their component name by "store/"

## Core API

```typescript
import logger from './lib/logger';
```

### Log Levels

Note: prefer using method that embed the level already (i.e. log.warn) rather
than using the log method and passing an explicit Lvl.

```typescript
enum Lvl {
  TRACE = -1,
  DEBUG = 0,
  INFO = 1,
  WARN = 2,
  ERROR = 3,
  CRITICAL = 4
}
```

### Basic Logging Methods

| Method | Description | Parameters |
|--------|-------------|------------|
| `log(level, component, message, context?, operation?)` | Main logging method | `LogLevel`, string, string, object?, string? |
| `trace(component, message, context?, operation?)` | TRACE level logging | string, string, object?, string? |
| `debug(component, message, context?, operation?)` | DEBUG level logging | string, string, object?, string? |
| `info(component, message, context?, operation?)` | INFO level logging | string, string, object?, string? |
| `warn(component, message, context?, operation?)` | WARN level logging | string, string, object?, string? |
| `error(component, message, context?, operation?)` | ERROR level logging | string, string, object?, string? |
| `critical(component, message, context?, operation?)` | CRITICAL level logging | string, string, object?, string? |
| `logError(err, component, message?, context?)` | Log Error object with stack | Error, string, string?, object? |

### Context Management

| Method | Description | Parameters |
|--------|-------------|------------|
| `setGlobalContext(context)` | Set global context for all logs | object |
| `startOperation(name, context?, timeout?)` | Start named operation with context | string, object?, number? |
| `endOperation(result?)` | End current operation | string\|object? |

### Performance Tracking

| Method | Description | Parameters | Returns |
|--------|-------------|------------|---------|
| `startTimer(name, component?)` | Start performance timer | string, string? | void |
| `endTimer(name, component?, logLevel?)` | End timer and log duration | string, string?, LogLevel? | number |
| `trackUserAction(action, details?)` | Track user interaction | string, object? | void |

### Log Management

| Method | Description | Parameters | Returns |
|--------|-------------|------------|---------|
| `beginBatch()` | Start batching logs | none | void |
| `endBatch(flush?)` | End batching and flush | boolean? | void |
| `flushBatch()` | Force flush batched logs | none | void |
| `clearLogs()` | Clear log buffer | none | void |
| `getAllLogs()` | Get all buffered logs | none | LogEntry[] |
| `setMinLogLevel(level)` | Set minimum log level | LogLevel | void |
| `destroy()` | Clean up resources | none | void |

## Configuration

```typescript
// Default configuration
const config: LoggerConfig = {
  minLevel: LogLevel.INFO,           // Minimum log level
  bufferSize: 500,                   // Local buffer size
  throttling: {
    enabled: true,                   // Enable log throttling
    interval: 60000,                 // Window in ms
    maxSimilarLogs: 5,               // Max similar logs in window
    byComponent: {                   // Component-specific rules
      ui: { interval: 30000, maxLogs: 10 },
      api: { interval: 10000, maxLogs: 3 }
    },
    sampleInterval: 10               // Sampling for throttle summaries
  },
  batching: {
    enabled: true,                   // Enable batching
    maxSize: 20,                     // Logs per batch
    maxWaitMs: 2000,                 // Max wait time
    retryCount: 3,                   // Retries for failed sends
    retryDelayMs: 1000               // Time between retries
  },
  consoleOutput: true,               // Also log to console
  captureStack: true,                // Capture stack traces for errors
  autoLogErrors: true,               // Auto-capture unhandled errors
  developerMode: false,              // Auto-detected from env
  highVolumeCategories: Set<string>, // Categories with reduced logging
  sampleRate: 0.01,                  // Sample rate for high volume
  criticalPatterns: RegExp[],        // Patterns never throttled
  operationTimeout: 300000           // 5 min operation timeout
}
```

## Initialization

```typescript
// Auto-initialized singleton:
import { logger } from './lib/logger';

// Custom configuration:
import { Logger, LogLevel } from './lib/logger';
const customLogger = new Logger({
  minLevel: LogLevel.DEBUG,
  consoleOutput: true,
  bufferSize: 1000
});
```

## Usage in Svelte Components

### Basic Component Logging

```svelte
<script lang="ts">
  import { logger } from '../lib/logger';
  import { onMount, onDestroy } from 'svelte';
  
  export let itemId: string;
  
  onMount(() => {
    logger.info('component', 'Component mounted', { itemId });
  });
  
  function handleClick() {
    logger.debug('ui', 'Button clicked', { itemId });
    // Logic...
  }
  
  onDestroy(() => {
    logger.debug('component', 'Component destroyed', { itemId });
  });
</script>

<button on:click={handleClick}>Click me</button>
```

### API Request Tracking

```svelte
<script lang="ts">
  import { logger } from '../lib/logger';
  
  async function fetchData() {
    logger.startOperation('fetch-data', { endpoint: '/api/data' });
    
    try {
      logger.startTimer('api-call');
      const response = await fetch('/api/data');
      const data = await response.json();
      const duration = logger.endTimer('api-call', 'api');
      
      logger.info('api', 'Data fetched successfully', { 
        items: data.length, 
        responseTime: duration 
      });
      
      logger.endOperation({ success: true });
      return data;
    } catch (error) {
      logger.logError(error as Error, 'api', 'Failed to fetch data');
      logger.endOperation({ success: false, errorType: (error as Error).name });
      throw error;
    }
  }
</script>
```

### Form Submission with User Tracking

```svelte
<script lang="ts">
  import { logger } from '../lib/logger';
  
  export let formId: string;
  let formData = { name: '', email: '' };
  
  function handleInput(field: string, value: string) {
    formData[field] = value;
    logger.trace('form', `Field updated: ${field}`);
  }
  
  function handleSubmit() {
    logger.trackUserAction('form-submit', { 
      formId,
      hasName: !!formData.name,
      hasEmail: !!formData.email
    });
    
    logger.info('form', 'Form submitted', { formId, formData });
    // Submit logic...
  }
</script>
```

### Batch Logging for High-Volume Operations

Batching is completely optional.
You only need to use it in specific high-volume logging scenarios where you want to optimize performance.

The logger works in two distinct modes:

1. **Normal Mode (Default)**: Each log is processed and sent individually as it occurs
2. **Batch Mode**: Must be explicitly enabled by calling `beginBatch()`

Logs are only batched when:
- You've explicitly called `beginBatch()`
- AND `config.batching.enabled` is true (which is the default)

When not using batching, each call to `logger.info()`, `logger.debug()`, etc. will immediately:
1. Add the log to the internal buffer 
2. Output to console (if enabled)
3. Send to backend individually

You only need to explicitly use batching for scenarios where you're generating many logs in a short period, like:

```svelte
<script lang="ts">
  import { logger } from '../lib/logger';
  
  export let items: any[];
  
  async function processItems() {
    logger.info('processing', `Starting batch processing of ${items.length} items`);
    
    // Begin batch to avoid log flooding
    logger.beginBatch();
    
    for (const item of items) {
      logger.debug('processing', `Processing item ${item.id}`, item);
      // Processing logic...
    }
    
    // Flush all logs in single batch
    logger.endBatch();
    
    logger.info('processing', `Completed processing ${items.length} items`);
  }
</script>
```

## Best Practices

1. **Use meaningful component names** for better filtering and organization
2. **Include context objects** with relevant data, avoiding sensitive information
3. **Use operations for workflow tracking** across component boundaries
4. **Apply appropriate log levels** - INFO for user-relevant events, DEBUG for development
5. **Enable batching** for high-volume logging scenarios
6. **Include timers** for performance-critical operations
7. **Call destroy()** when applicable (e.g., during application shutdown)
8. **Set minimum log level** based on environment (DEBUG in dev, INFO in prod)
