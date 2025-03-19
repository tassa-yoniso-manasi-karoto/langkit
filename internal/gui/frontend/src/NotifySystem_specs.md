# Langkit Notification System Architecture

This document provides a comprehensive guide to the notification and display system in Langkit. The system consists of several interconnected components that handle error reporting, log management, progress tracking, and user notifications.

## 1. System Components Overview

### 1.1 Core Components

1. **Log Store (`logStore`):**
   - Central repository for all application logs
   - Categorizes logs by level (DEBUG, INFO, WARN, ERROR)
   - Tracks special log behaviors (abort_task, abort_all, user_cancel, probe)
   - Manages log history with configurable maximum entries

2. **Error Store (`errorStore`):**
   - Manages application errors and warnings
   - Supports different error severities (critical, warning, info)
   - Provides auto-dismissal based on severity
   - Allows action handlers for error resolution

3. **Progress Bars Store (`progressBarsStore`):**
   - Tracks multiple concurrent process progress states
   - Supports error states for individual progress bars
   - Handles process cancellation states

4. **Settings Store (`settings`):**
   - Stores user preferences and application settings
   - Contains internal counters like `appStartCount`
   - Tracks user interaction history (`hasSeenLogViewerTooltip`)

### 1.2 UI Components

1. **LogViewer:**
   - Displays filtered log entries with color-coded styling
   - Provides level filtering (DEBUG, INFO, WARN, ERROR)
   - Contains auto-scroll functionality
   - Uses behavior-specific styling for error logs

2. **LogViewerNotification:**
   - Appears above the LogViewer toggle button
   - Shows different messages for processing vs. errors
   - Displays detailed error counts by type
   - Only shows for new users (first 5 app starts) for processing notifications

3. **Progress Manager:**
   - Visualizes ongoing processes with progress bars
   - Shows error states visually
   - Supports cancellation interactions

4. **Process Error Tooltip:**
   - Displays actionable error messages
   - Provides context-specific actions

## 2. Data Flow and Interactions

### 2.1 Log Event Flow

```
Backend → EventsOn("log") → logStore.addLog() → UI Components
```

1. Backend sends log events
2. App.svelte captures via EventsOn("log")
3. logStore processes and categorizes logs
4. UI components reactively update:
   - LogViewer shows entries
   - LogViewerNotification detects specific error types
   - Log button shows error indicator when needed

### 2.2 Progress Tracking Flow

```
Backend → EventsOn("progress") → progressBarsStore → UI Components
```

1. Backend sends progress events with numeric values and IDs
2. App.svelte captures via EventsOn("progress")
3. progressBarsStore updates current progress state
4. UI components reactively update:
   - Progress Manager shows visual progress
   - Error indicators show on failure

### 2.3 Error Handling Flow

```
Error Detection → errorStore.addError() → UI Components
```

1. Errors detected from:
   - API responses
   - Log messages with ERROR level
   - Progress failures
2. errorStore categorizes and stores errors
3. UI components reactively update:
   - Process Error Tooltip shows error messages
   - LogViewerNotification shows error counts

## 3. Color System and Theme Variables

### 3.1 HSL Variables for Consistent Theming

The system uses HSL color variables defined in multiple layers:

1. **Tailwind Config (Base Definitions):**
   ```javascript
   // Base colors
   const primaryHue = 261;
   const primarySaturation = '90%';
   const primaryLightness = '70%';
   
   // Error state colors
   const errorTaskHue = 50;  // Yellow for task errors
   const errorAllHue = 0;    // Red for critical errors
   const userCancelHue = 220; // Blue-gray for cancellations
   ```

2. **CSS Variables (App-wide Access):**
   ```css
   :root {
     /* Core color variables */
     --primary-hue: 261;
     --primary-saturation: 90%;
     --primary-lightness: 70%;
     
     /* Error state colors */
     --error-task-hue: 50;
     --error-all-hue: 0;
     --user-cancel-hue: 220;
   }
   ```

3. **Component Usage:**
   ```css
   /* Example - log behavior styling */
   .log-behavior-abort-task {
     background: linear-gradient(
       to right,
       hsla(var(--error-task-hue), var(--error-task-saturation), var(--error-task-lightness), 0.08) 0%,
       rgba(0, 0, 0, 0) 70%
     );
     border-left: 2px solid hsla(var(--error-task-hue), var(--error-task-saturation), var(--error-task-lightness), 0.6);
   }
   ```

### 3.2 Behavior Color Mapping

| Behavior    | Color         | Variable Base        | Usage                            |
|-------------|---------------|----------------------|----------------------------------|
| abort_task  | Yellow        | --error-task-*       | Task-specific failures           |
| abort_all   | Red           | --error-all-*        | Critical/global failures         |
| user_cancel | Gray-blue     | --user-cancel-*      | User-initiated cancellations     |
| probe       | Yellow        | --error-task-*       | Non-critical warnings            |

## 4. LogViewer Component Details

### 4.1 Core Functionality

- **Log Filtering:** Filters logs by level (DEBUG, INFO, WARN, ERROR)
- **Auto-scroll:** Tracks user vs. programmatic scrolling
- **Virtual Display:** Only shows limited logs for performance
- **Behavior Styling:** Special styling for different log behaviors

### 4.2 Styling System

```css
/* Examples of the styling system */

/* Log levels with enhanced visual treatment */
.log-level-debug {
    text-shadow: 0 0 6px hsla(var(--primary-hue), var(--primary-saturation), var(--primary-lightness), 0.4);
    font-weight: 500;
}

.log-level-error {
    text-shadow: 0 0 8px rgba(239, 68, 68, 0.5);
    font-weight: 700;
    letter-spacing: 0.5px;
}

/* Behavior-specific styling */
.log-behavior-abort-task {
    background: linear-gradient(
        to right,
        hsla(var(--error-task-hue), var(--error-task-saturation), var(--error-task-lightness), 0.08) 0%,
        rgba(0, 0, 0, 0) 70%
    );
    border-left: 2px solid hsla(var(--error-task-hue), var(--error-task-saturation), var(--error-task-lightness), 0.6);
}
```

### 4.3 Key Design Considerations

1. **Border Glow:** LogViewer has a special glow effect on bottom and right edges to simulate light source reflection
2. **Glassmorphism:** Uses semi-transparent background with blur effects
3. **Custom Scrollbar:** Styled to match theme with primary color

## 5. LogViewerNotification Component Details

### 5.1 Display Logic

1. **Processing Mode:**
   - Shows for first 5 application starts only (`appStartCount <= 5`)
   - Only appears if user hasn't seen it before (`!hasSeenLogViewerTooltip`)
   - Auto-hides after user interaction

2. **Error Mode:**
   - Shows for all users regardless of experience level
   - Displays different messages based on error types:
     - Task failures (abort_task)
     - Critical failures (abort_all)
     - General errors (ERROR level logs)
   - Stays visible until user interaction

### 5.2 Error Detection Logic

```javascript
// Only consider logs that are ERROR level with behavior
abortTaskLogs = logs.filter(log => 
    log.behavior === 'abort_task' && 
    log.level.toUpperCase() === 'ERROR'
);

abortAllLogs = logs.filter(log => 
    log.behavior === 'abort_all' && 
    log.level.toUpperCase() === 'ERROR'
);

// ERROR logs without specific behaviors
errorLevelLogs = logs.filter(log => 
    log.level.toUpperCase() === 'ERROR' && 
    (!log.behavior || log.behavior !== 'user_cancel') &&
    (!log.message || !log.message.toLowerCase().includes('cancel'))
);
```

### 5.3 Positioning System

- Positioned relative to LogViewer toggle button
- Uses Portal for absolute positioning outside component hierarchy
- Repositions on window resize

## 6. Critical Implementation Details and Caveats

### 6.1 Log Behavior Handling

- **User Cancellations:** Logs with `user_cancel` behavior should NOT be treated as errors
- **Abort Task vs. Abort All:** 
  - `abort_task` affects specific tasks
  - `abort_all` is a more severe, global error

### 6.2 LogViewer Automatic Behavior

- LogViewer is NOT automatically shown during processing (this is intentional)
- Instead, notifications guide users to open it manually
- This prevents disrupting the user's workflow

### 6.3 Mobile Considerations

- All components should preserve responsive design
- Glass effects may need fallbacks on older browsers

### 6.4 Performance Optimizations

- LogViewer uses virtual rendering for large log sets
- Auto-scroll handles performance by limiting visible logs
- Notifications use transitions and minimal DOM updates

### 6.5 Known Edge Cases

1. **Rapid Log Generation:**
   - High-frequency logging can impact UI performance
   - LogViewer implements log limiting (MAX_VISIBLE_LOGS)

2. **Multiple Error States:**
   - System prioritizes abort_all > abort_task > general errors
   - Notifications show combined counts when multiple types exist

## 7. Canonical Implementation Patterns

### 7.1 Adding New Log Behaviors

1. Define HSL variables in tailwind.config.js:
   ```javascript
   const newBehaviorHue = 280; // Example value
   const newBehaviorSaturation = '70%';
   const newBehaviorLightness = '60%';
   ```

2. Add to CSS variables in app.css:
   ```css
   :root {
     --new-behavior-hue: 280;
     --new-behavior-saturation: 70%;
     --new-behavior-lightness: 60%;
   }
   ```

3. Add styling in LogViewer.svelte:
   ```css
   .log-behavior-new-behavior {
     background: linear-gradient(
       to right,
       hsla(var(--new-behavior-hue), var(--new-behavior-saturation), var(--new-behavior-lightness), 0.08) 0%,
       rgba(0, 0, 0, 0) 70%
     );
     border-left: 2px solid hsla(var(--new-behavior-hue), var(--new-behavior-saturation), var(--new-behavior-lightness), 0.6);
   }
   ```

4. Update behaviorColors map:
   ```javascript
   const behaviorColors: Record<string, string> = {
     // Existing behaviors...
     'new_behavior': 'text-new-behavior log-behavior-new-behavior'
   };
   ```

5. Update LogViewerNotification detection logic:
   ```javascript
   newBehaviorLogs = logs.filter(log => 
     log.behavior === 'new_behavior' && 
     log.level.toUpperCase() === 'ERROR'
   );
   ```

### 7.2 Modifying Notification Messages

The notification messages use conditional rendering with specific wording based on error types:

```svelte
{#if abortTaskLogs.length > 0 && abortAllLogs.length > 0}
  {abortTaskLogs.length} task{abortTaskLogs.length !== 1 ? 's' : ''} and {abortAllLogs.length} major process{abortAllLogs.length !== 1 ? 'es' : ''} stopped with errors
{:else if abortTaskLogs.length > 0}
  {abortTaskLogs.length} task{abortTaskLogs.length !== 1 ? 's' : ''} stopped with errors
{:else if abortAllLogs.length > 0}
  {abortAllLogs.length} major process{abortAllLogs.length !== 1 ? 'es' : ''} stopped with errors
{:else}
  {errorLevelLogs.length} error{errorLevelLogs.length !== 1 ? 's' : ''} detected during processing
{/if}
```

Update the text directly in the LogViewerNotification component for message changes.

### 7.3 Adjusting Notification Display Conditions

The key conditions that control when notifications appear:

```javascript
// Processing notification
$: shouldShowProcessingTooltip = mode === 'processing' && 
   processingActive && 
   appStartCount <= 5 && 
   !hasSeenTooltip;

// Error notification
$: shouldShowErrorTooltip = mode === 'error' && 
   (abortTaskLogs.length > 0 || abortAllLogs.length > 0 || errorLevelLogs.length > 0);
```

Modify these conditions to change when notifications appear.

## 8. Future Considerations and Enhancements

### 8.1 Potential Improvements

1. **Notification Grouping:**
   - Group similar errors to reduce notification noise
   - Show summary counts with expandable details

2. **Log Search Functionality:**
   - Add search within LogViewer
   - Filter by custom text patterns

3. **Log Export:**
   - Add ability to export filtered logs
   - Support multiple formats (JSON, TXT)

4. **Interactive Log Details:**
   - Expand log entries to show more context
   - Link to relevant documentation

### 8.2 Accessibility Considerations

- Ensure color contrast meets WCAG standards
- Add keyboard navigation for log entries
- Provide screen reader support for notifications

## 9. Integration with Backend

### 9.1 Log Event Structure

```typescript
interface LogMessage {
  level: string;         // DEBUG, INFO, WARN, ERROR
  message: string;       // The log message
  time: string;          // Timestamp
  behavior?: string;     // Optional behavior flag
  [key: string]: any;    // Additional structured fields
}
```

### 9.2 Progress Event Structure

```typescript
interface ProgressEvent {
  id: string;            // Unique identifier
  name: string;          // Display name
  progress: number;      // 0-100 completion percentage
  status: string;        // Current status message
  errorState?: string;   // Optional error state (abort_task, abort_all, user_cancel)
}
```

## 10. Summary of Critical Rules

1. **User Cancellations:**
   - NEVER treat user cancellations as errors
   - Logs with `user_cancel` behavior should be visually distinct

2. **Color System:**
   - ALWAYS use HSL variables for consistency
   - Add new colors through the tailwind.config.js → app.css → component pipeline

3. **Error Detection:**
   - CHECK both log level AND behavior for proper error handling
   - Filter out cancellation-related messages in error detection

4. **LogViewer Visibility:**
   - DO NOT auto-show LogViewer during processing
   - Use notifications to guide user to open it manually

5. **Component Styling:**
   - MAINTAIN glassmorphic styling across components
   - Preserve the special glow effect on LogViewer's right and bottom edge

6. **Performance:**
   - LIMIT visible logs for better performance
   - Handle high-frequency log updates efficiently

This document serves as the definitive reference for the notification system architecture. All modifications should adhere to the patterns and principles outlined here to ensure consistency and reliability.