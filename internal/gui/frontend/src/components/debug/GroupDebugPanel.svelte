<!-- src/components/debug/GroupDebugPanel.svelte -->
<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { groupInspector } from '../../lib/groupInspector';
  import { validateGroupSystem, type ValidationError } from '../../lib/groupValidation'; // Import ValidationError type
  import { featureGroupStore } from '../../lib/featureGroupStore';
  
  // Only show in development mode
  export let show: boolean = import.meta.env.DEV;
  
  // State
  let systemState: any = null;
  let validationErrors: ValidationError[] = []; // Use imported type
  let activeTab: 'groups' | 'features' | 'options' | 'validation' = 'groups';
  let refreshInterval: number | null = null;
  let autoRefresh: boolean = false;
  
  // Methods
  function refresh() {
    systemState = groupInspector.getGroupSystemState();
    validationErrors = validateGroupSystem(true); // Pass silent=true
  }
  
  function toggleAutoRefresh() {
    autoRefresh = !autoRefresh;
    
    if (autoRefresh) {
      refreshInterval = window.setInterval(refresh, 1000);
    } else if (refreshInterval !== null) {
      clearInterval(refreshInterval);
      refreshInterval = null;
    }
  }
  
  function forceRefresh() {
    refresh();
  }
  
  // Setup
  onMount(() => {
    refresh();
  });
  
  onDestroy(() => {
    if (refreshInterval !== null) {
      clearInterval(refreshInterval);
    }
  });
</script>

{#if show}
  <div class="group-debug-panel">
    <div class="panel-header">
      <h2>Feature Group System Debug</h2>
      <div class="controls">
        <button on:click={forceRefresh} class="refresh-btn">
          🔄 Refresh
        </button>
        <label>
          <input type="checkbox" bind:checked={autoRefresh} on:change={toggleAutoRefresh} />
          Auto-refresh
        </label>
      </div>
    </div>
    
    <div class="tabs">
      <button 
        class:active={activeTab === 'groups'} 
        on:click={() => activeTab = 'groups'}
      >
        Groups ({systemState?.groups.length ?? 0})
      </button>
      <button 
        class:active={activeTab === 'features'} 
        on:click={() => activeTab = 'features'}
      >
        Features ({systemState?.features.length ?? 0})
      </button>
      <button 
        class:active={activeTab === 'options'} 
        on:click={() => activeTab = 'options'}
      >
        Options ({systemState?.options.length ?? 0})
      </button>
      <button 
        class:active={activeTab === 'validation'} 
        on:click={() => activeTab = 'validation'}
        class:has-errors={validationErrors.length > 0}
      >
        Validation ({validationErrors.length})
      </button>
    </div>
    
    <div class="panel-content">
      {#if activeTab === 'groups' && systemState}
        <table>
          <thead>
            <tr>
              <th>Group ID</th>
              <th>Options</th>
              <th>Features</th>
              <th>Enabled</th>
              <th>Active Feature</th>
            </tr>
          </thead>
          <tbody>
            {#each systemState.groups as group}
              <tr>
                <td>{group.id}</td>
                <td>{group.optionCount}</td>
                <td>{group.featureCount}</td>
                <td>{group.enabledCount}</td>
                <td>{group.activeFeature || '-'}</td>
              </tr>
            {/each}
          </tbody>
        </table>
      {:else if activeTab === 'features' && systemState}
        <table>
          <thead>
            <tr>
              <th>Feature ID</th>
              <th>Groups</th>
              <th>Enabled</th>
              <th>Is Topmost</th>
            </tr>
          </thead>
          <tbody>
            {#each systemState.features as feature}
              <tr>
                <td>{feature.id}</td>
                <td>{feature.groups.join(', ') || '-'}</td>
                <td>{feature.enabled ? '✓' : '-'}</td>
                <td>{feature.isTopmost ? '✓' : '-'}</td>
              </tr>
            {/each}
          </tbody>
        </table>
      {:else if activeTab === 'options' && systemState}
        <table>
          <thead>
            <tr>
              <th>Group</th>
              <th>Option</th>
              <th>Type</th>
              <th>Value</th>
              <th>Visible In</th>
            </tr>
          </thead>
          <tbody>
            {#each systemState.options as option}
              <tr>
                <td>{option.groupId}</td>
                <td>{option.optionId}</td>
                <td>{option.type}</td>
                <td class="value-cell">
                  {JSON.stringify(option.currentValue)}
                </td>
                <td>{option.visibleInFeature || '-'}</td>
              </tr>
            {/each}
          </tbody>
        </table>
      {:else if activeTab === 'validation'}
        {#if validationErrors.length === 0}
          <div class="validation-success">
            ✓ No validation errors detected
          </div>
        {:else}
          <table>
            <thead>
              <tr>
                <th>Type</th>
                <th>Feature</th>
                <th>Group</th>
                <th>Option</th>
                <th>Severity</th>
                <th>Message</th>
              </tr>
            </thead>
            <tbody>
              {#each validationErrors as error}
                <tr class="error-row" class:error-critical={error.severity === 'error'}>
                  <td>{error.type}</td>
                  <td>{error.featureId || '-'}</td>
                  <td>{error.groupId || '-'}</td>
                  <td>{error.optionId || '-'}</td>
                  <td>{error.severity}</td>
                  <td>{error.message}</td>
                </tr>
              {/each}
            </tbody>
          </table>
        {/if}
      {/if}
    </div>
  </div>
{/if}

<style>
  .group-debug-panel {
    position: fixed;
    bottom: 0;
    right: 0;
    width: 80%;
    max-width: 1200px;
    height: 400px;
    background: rgba(30, 30, 40, 0.95);
    border: 1px solid #444;
    border-top-left-radius: 8px;
    z-index: 9999;
    color: #eee;
    font-family: monospace;
    display: flex;
    flex-direction: column;
    box-shadow: 0 0 20px rgba(0, 0, 0, 0.5);
  }
  
  .panel-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 8px 16px;
    background: rgba(50, 50, 60, 0.8);
    border-bottom: 1px solid #555;
  }
  
  .panel-header h2 {
    margin: 0;
    font-size: 16px;
  }
  
  .controls {
    display: flex;
    gap: 12px;
    align-items: center;
  }
  
  .tabs {
    display: flex;
    border-bottom: 1px solid #444;
  }
  
  .tabs button {
    background: transparent;
    border: none;
    padding: 8px 16px;
    color: #ccc;
    font-family: monospace;
    cursor: pointer;
  }
  
  .tabs button.active {
    background: rgba(70, 70, 90, 0.6);
    color: #fff;
    border-bottom: 2px solid var(--primary-color);
  }
  
  .tabs button.has-errors {
    color: var(--error-task-color);
  }
  
  .panel-content {
    flex: 1;
    overflow: auto;
    padding: 8px;
  }
  
  table {
    width: 100%;
    border-collapse: collapse;
    font-size: 12px;
  }
  
  th, td {
    padding: 4px 8px;
    text-align: left;
    border-bottom: 1px solid #333;
  }
  
  th {
    background: rgba(60, 60, 80, 0.6);
    position: sticky;
    top: 0;
  }
  
  .error-row.error-critical {
    background: rgba(255, 50, 50, 0.2);
  }
  
  .value-cell {
    max-width: 200px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  
  .validation-success {
    padding: 20px;
    text-align: center;
    color: #8f8;
  }
  
  .refresh-btn {
    background: rgba(100, 100, 120, 0.4);
    border: 1px solid #555;
    color: #eee;
    padding: 4px 8px;
    border-radius: 4px;
    cursor: pointer;
  }
  
  .refresh-btn:hover {
    background: rgba(100, 100, 120, 0.6);
  }
</style>