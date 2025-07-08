<script lang="ts">
    // Props
    export let currentStatistics: any;
    export let currentSettings: any;
    export let currentUserActivityState: string;
    export let isForced: boolean;
</script>

<h4>Application State</h4>
    
<div class="state-section">
    <h5 class="text-xs font-semibold mb-2 opacity-80">Counter Values</h5>
    <table class="state-table">
        <tbody>
            <tr>
                <td class="state-key">countAppStart</td>
                <td class="state-value">{currentStatistics?.countAppStart || 0}</td>
                <td class="state-description">App launch count</td>
            </tr>
            <tr>
                <td class="state-key">countProcessStart</td>
                <td class="state-value">{currentStatistics?.countProcessStart || 0}</td>
                <td class="state-description">Processing run count</td>
            </tr>
        </tbody>
    </table>
</div>

<div class="state-section">
    <h5 class="text-xs font-semibold mb-2 opacity-80">File Settings</h5>
    <table class="state-table">
        <tbody>
            <tr>
                <td class="state-key">intermediaryFileMode</td>
                <td class="state-value">{currentSettings?.intermediaryFileMode || 'keep'}</td>
                <td class="state-description">Intermediary file handling</td>
            </tr>
        </tbody>
    </table>
</div>

<div class="state-section">
    <h5 class="text-xs font-semibold mb-2 opacity-80">User Activity</h5>
    <table class="state-table">
        <tbody>
            <tr>
                <td class="state-key">userActivityState</td>
                <td class="state-value">
                    <span class:text-green-400={currentUserActivityState === 'active'}
                          class:text-yellow-400={currentUserActivityState === 'idle'}
                          class:text-red-400={currentUserActivityState === 'afk'}>
                        {currentUserActivityState}
                        {#if isForced}
                            <span class="text-purple-400 text-xs">(forced)</span>
                        {/if}
                    </span>
                </td>
                <td class="state-description">
                    {#if currentUserActivityState === 'active'}
                        User is actively interacting
                    {:else if currentUserActivityState === 'idle'}
                        No activity for 5s-5min
                    {:else}
                        Away from keyboard >5min
                    {/if}
                </td>
            </tr>
        </tbody>
    </table>
</div>

<style>
    h4 {
        margin: 0 0 12px 0;
        font-size: 13px;
        opacity: 0.9;
    }
    
    /* State tab styles */
    .state-section {
        margin-bottom: 16px;
        padding-bottom: 12px;
        border-bottom: 1px solid rgba(255, 255, 255, 0.1);
    }
    
    .state-section:last-child {
        border-bottom: none;
        margin-bottom: 0;
    }
    
    .state-table {
        width: 100%;
        border-collapse: collapse;
        font-size: 12px;
    }
    
    .state-table tr {
        border-bottom: 1px solid rgba(255, 255, 255, 0.05);
    }
    
    .state-table tr:last-child {
        border-bottom: none;
    }
    
    .state-key {
        width: 40%;
        padding: 6px 4px;
        color: var(--primary-color, #9f6ef7);
        font-family: monospace;
    }
    
    .state-value {
        width: 20%;
        padding: 6px 4px;
        color: rgba(255, 255, 255, 0.9);
        font-weight: 600;
    }
    
    .state-description {
        width: 40%;
        padding: 6px 4px;
        color: rgba(255, 255, 255, 0.6);
        font-style: italic;
    }
    
    /* Activity state colors */
    .text-green-400 {
        color: #68e796;
    }
    
    .text-yellow-400 {
        color: #fbbf24;
    }
    
    .text-red-400 {
        color: #f87171;
    }
    
    .text-purple-400 {
        color: #a78bfa;
    }
    
    .text-xs {
        font-size: 0.75rem;
    }
    
    .text-gray-500 {
        color: rgba(255, 255, 255, 0.5);
    }
    
    .opacity-80 {
        opacity: 0.8;
    }
    
    .font-semibold {
        font-weight: 600;
    }
    
    .mb-2 {
        margin-bottom: 0.5rem;
    }
</style>