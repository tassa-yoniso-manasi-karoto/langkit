# Adding New Settings to Langkit

This document describes how to properly add a new configuration setting that works across the entire stack: backend config, WebRPC API, and frontend UI.

## Overview

Settings in Langkit flow through multiple layers:

```
┌─────────────────────────────────────────────────────────────────────────┐
│                           Frontend (Svelte)                             │
│  Settings.svelte ←→ stores.ts ←→ api/services/settings.ts              │
└─────────────────────────────────────────────────────────────────────────┘
                                    ↕ WebRPC (HTTP/JSON)
┌─────────────────────────────────────────────────────────────────────────┐
│                           Backend (Go)                                  │
│  api/services/settings.go ←→ config/settings.go ←→ viper (YAML file)  │
└─────────────────────────────────────────────────────────────────────────┘
```

Adding a new setting requires changes in **6 locations** (7 if nested):

1. Backend config struct (`internal/config/settings.go`)
2. RIDL schema (`api/schemas/services/settings.ridl`)
3. Regenerate API types (`api/Makefile`)
4. Backend API service (`internal/api/services/settings.go`)
5. Frontend store types and defaults (`internal/gui/frontend/src/lib/stores.ts`)
6. Frontend UI (`internal/gui/frontend/src/components/Settings.svelte`)

---

## Step 1: Backend Config Struct

**File:** `internal/config/settings.go`

### 1.1 Add the field to the Settings struct

```go
type Settings struct {
    // ... existing fields ...

    // Your new setting - include both json and mapstructure tags
    MyNewSetting string `json:"myNewSetting" mapstructure:"my_new_setting"`
}
```

**Tag conventions:**
- `json:` - camelCase, used for API serialization
- `mapstructure:` - snake_case, used for YAML config file

### 1.2 Add to SaveSettings function

Find the `SaveSettings` function and add a `viper.Set` call:

```go
func SaveSettings(settings Settings) error {
    // ... existing viper.Set calls ...

    viper.Set("my_new_setting", settings.MyNewSetting)

    // ... rest of function ...
}
```

### 1.3 (Optional) Set a default value

If your setting needs a default, add it in `InitConfig` or where defaults are set:

```go
viper.SetDefault("my_new_setting", "default_value")
```

---

## Step 2: RIDL Schema

**File:** `api/schemas/services/settings.ridl`

Add your field to the `Settings` struct:

```ridl
struct Settings
  - apiKeys: APIKeys
  - targetLanguage: string
  # ... existing fields ...
  - myNewSetting: string    # Add your new field
```

**RIDL type mappings:**
| Go Type | RIDL Type |
|---------|-----------|
| `string` | `string` |
| `bool` | `bool` |
| `int` | `int32` |
| `float64` | `float64` |
| Nested struct | Define a new `struct` |

### For nested structures

If your setting is a nested object, define a new struct:

```ridl
struct MyNestedConfig
  - enabled: bool
  - value: string

struct Settings
  # ... existing fields ...
  - myNestedSetting: MyNestedConfig
```

---

## Step 3: Regenerate API Types

**Directory:** `api/`

Run the Makefile to regenerate both Go and TypeScript types:

```bash
cd api
make all
```

This generates:
- `internal/api/generated/api.gen.go` - Go types and server/client code
- `internal/gui/frontend/src/api/generated/api.gen.ts` - TypeScript types

**Always verify** the generated files contain your new field before proceeding.

---

## Step 4: Backend API Service

**File:** `internal/api/services/settings.go`

You must map the field in **both directions**.

### 4.1 LoadSettings (config → generated)

```go
func (s *SettingsService) LoadSettings(ctx context.Context) (*generated.Settings, error) {
    settings, err := config.LoadSettings()
    if err != nil {
        return nil, err
    }

    genSettings := &generated.Settings{
        // ... existing mappings ...
        MyNewSetting: settings.MyNewSetting,
    }

    return genSettings, nil
}
```

### 4.2 SaveSettings (generated → config)

```go
func (s *SettingsService) SaveSettings(ctx context.Context, genSettings *generated.Settings) error {
    settings := config.Settings{
        // ... existing mappings ...
        MyNewSetting: genSettings.MyNewSetting,
    }

    // ... rest of function ...
}
```

### For nested structures

```go
// LoadSettings
genSettings := &generated.Settings{
    MyNestedSetting: &generated.MyNestedConfig{
        Enabled: settings.MyNestedSetting.Enabled,
        Value:   settings.MyNestedSetting.Value,
    },
}

// SaveSettings - check for nil to avoid panics
if genSettings.MyNestedSetting != nil {
    settings.MyNestedSetting.Enabled = genSettings.MyNestedSetting.Enabled
    settings.MyNestedSetting.Value = genSettings.MyNestedSetting.Value
}
```

---

## Step 5: Frontend Store

**File:** `internal/gui/frontend/src/lib/stores.ts`

### 5.1 Add to Settings type

```typescript
type Settings = {
    // ... existing fields ...
    myNewSetting?: string;  // Use ? for optional fields
};
```

### 5.2 Add default value in initSettings

```typescript
const initSettings: Settings = {
    // ... existing defaults ...
    myNewSetting: 'default_value',
};
```

### 5.3 For nested structures: Update mergeSettingsWithDefaults

If your setting is a nested object, you **must** add merge logic to prevent null errors:

```typescript
export function mergeSettingsWithDefaults(loaded: Partial<Settings>): Settings {
    return {
        ...initSettings,
        ...loaded,
        // ... existing nested merges ...

        // Add your nested setting merge
        myNestedSetting: {
            enabled: loaded.myNestedSetting?.enabled ?? initSettings.myNestedSetting!.enabled,
            value: loaded.myNestedSetting?.value || initSettings.myNestedSetting!.value,
        },
    };
}
```

**Why this matters:** The spread operator `...loaded` will overwrite defaults with `null` if the backend returns `null` for nested objects. The explicit merge ensures the structure is always complete.

---

## Step 6: Frontend UI

**File:** `internal/gui/frontend/src/components/Settings.svelte`

### 6.1 Add to currentSettings initialization

```typescript
let currentSettings = {
    // ... existing fields ...
    myNewSetting: 'default_value',
};
```

### 6.2 Add the UI control

Add your setting in the appropriate section of the template:

```svelte
<div class="setting-row">
    <div class="setting-label">
        <span>My New Setting</span>
        <span class="setting-description">Description of what this does</span>
    </div>
    <div class="setting-control">
        <!-- For text input -->
        <TextInput
            bind:value={currentSettings.myNewSetting}
            placeholder="Enter value"
            className="w-full"
        />

        <!-- OR for boolean toggle -->
        <label class="toggle-switch">
            <input
                type="checkbox"
                bind:checked={currentSettings.myNewSetting}
                on:change={updateSettings}
            />
            <span class="slider round"></span>
        </label>

        <!-- OR for dropdown -->
        <SelectInput
            bind:value={currentSettings.myNewSetting}
            on:change={updateSettings}
        >
            <option value="option1">Option 1</option>
            <option value="option2">Option 2</option>
        </SelectInput>
    </div>
</div>
```

### 6.3 For settings that need immediate save

If the setting should save immediately when changed (like toggles), add `on:change={updateSettings}`.

For settings that save with the "Save" button, just use `bind:value`.

---

## Complete Example: Adding a "Theme" Setting

Here's a complete walkthrough for adding a `theme` setting with values "light", "dark", or "auto".

### 1. config/settings.go

```go
type Settings struct {
    // ... existing ...
    Theme string `json:"theme" mapstructure:"theme"`
}

// In SaveSettings:
viper.Set("theme", settings.Theme)
```

### 2. settings.ridl

```ridl
struct Settings
  # ... existing ...
  - theme: string
```

### 3. Regenerate

```bash
cd api && make all
```

### 4. services/settings.go

```go
// LoadSettings
genSettings := &generated.Settings{
    // ...
    Theme: settings.Theme,
}

// SaveSettings
settings := config.Settings{
    // ...
    Theme: genSettings.Theme,
}
```

### 5. stores.ts

```typescript
type Settings = {
    // ...
    theme?: string;
};

const initSettings: Settings = {
    // ...
    theme: 'auto',
};
```

### 6. Settings.svelte

```typescript
let currentSettings = {
    // ...
    theme: 'auto',
};
```

```svelte
<div class="setting-row">
    <div class="setting-label">
        <span>Theme</span>
        <span class="setting-description">Application color theme</span>
    </div>
    <div class="setting-control">
        <SelectInput
            bind:value={currentSettings.theme}
            on:change={updateSettings}
        >
            <option value="auto">Auto (System)</option>
            <option value="light">Light</option>
            <option value="dark">Dark</option>
        </SelectInput>
    </div>
</div>
```

---

## Troubleshooting

### Setting doesn't persist after restart

1. Check that the field is in the RIDL schema and you ran `make all`
2. Verify the field is mapped in both `LoadSettings` and `SaveSettings` in `services/settings.go`
3. Verify `viper.Set()` is called in `config/settings.go`

### "null is not an object" error for nested settings

1. Add the nested structure to `mergeSettingsWithDefaults()` in `stores.ts`
2. Use optional chaining (`?.`) when accessing nested properties
3. Ensure `initSettings` has the complete nested structure

### Setting shows default value instead of saved value

1. Check the JSON field name matches between Go struct tag and TypeScript
2. Verify the RIDL field name matches the JSON name (camelCase)
3. Check that `LoadSettings` in the API service maps the field

### Type mismatch errors after regeneration

1. Clean and regenerate: `cd api && make clean && make all`
2. Restart the TypeScript language server in your IDE
3. Rebuild the frontend: `cd internal/gui/frontend && npm run build`
