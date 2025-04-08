// src/lib/metrics.ts
import { writable } from 'svelte/store';

// Create metrics store
export const metricsStore = writable({
    // Store metrics
    storeUpdates: 0,
    updateRate: 0,
    activeSubscriptions: 0,
    totalSubscriptions: 0,
    lastUpdatedOption: '',
    lastUpdateTime: Date.now(),
    
    // Component metrics
    activeComponents: 0,
    componentsCreated: 0,
    componentsDestroyed: 0,
    
    // Debug history
    recentChanges: [] as string[]
});

// Add a change to history
export function trackChange(message: string) {
    const timestamp = new Date().toISOString().substr(11, 12);
    
    metricsStore.update(m => {
        m.recentChanges = [`${timestamp} ${message}`, ...m.recentChanges.slice(0, 19)];
        return m;
    });
}

// Track store updates
export function trackStoreUpdate(groupId: string, optionId: string, value: any) {
    metricsStore.update(m => {
        m.storeUpdates++;
        m.lastUpdatedOption = `${groupId}.${optionId}=${value}`;
        
        // Calculate update rate
        const now = Date.now();
        const elapsed = (now - m.lastUpdateTime) / 1000;
        m.updateRate = elapsed > 0 ? 1 / elapsed : m.updateRate;
        m.lastUpdateTime = now;
        
        return m;
    });
    
    trackChange(`Update: ${groupId}.${optionId}=${value}`);
}

// Track subscriptions
export function trackSubscription(isAdd: boolean) {
    metricsStore.update(m => {
        if (isAdd) {
            m.activeSubscriptions++;
            m.totalSubscriptions++;
            trackChange(`New subscription (${m.activeSubscriptions} active)`);
        } else {
            m.activeSubscriptions--;
            trackChange(`Subscription removed (${m.activeSubscriptions} active)`);
        }
        return m;
    });
}

// Track component lifecycle
export function trackComponentMount(id: string) {
    metricsStore.update(m => {
        m.activeComponents++;
        m.componentsCreated++;
        trackChange(`Component mounted: ${id} (${m.activeComponents} active)`);
        return m;
    });
}

export function trackComponentDestroy(id: string) {
    metricsStore.update(m => {
        m.activeComponents--;
        m.componentsDestroyed++;
        trackChange(`Component destroyed: ${id} (${m.activeComponents} active)`);
        return m;
    });
}

// Reset metrics
export function resetMetrics() {
    metricsStore.set({
        storeUpdates: 0,
        updateRate: 0,
        activeSubscriptions: 0,
        totalSubscriptions: 0,
        lastUpdatedOption: '',
        lastUpdateTime: Date.now(),
        
        activeComponents: 0,
        componentsCreated: 0,
        componentsDestroyed: 0,
        
        recentChanges: []
    });
    
    trackChange("Metrics reset");
}