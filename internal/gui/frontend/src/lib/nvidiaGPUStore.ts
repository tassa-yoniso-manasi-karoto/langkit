import { writable, derived, get } from 'svelte/store';
import { logger } from './logger';
import { GetNvidiaGPUInfo } from '../api/services/system';
import { sepLibVRAMRequirements } from './featureModel';

// NVIDIA GPU status interface
export interface NvidiaGPUStatus {
    available: boolean;  // Whether an NVIDIA GPU with drivers is detected
    name: string;        // GPU name (e.g., "NVIDIA GeForce RTX 3080")
    vramMiB: number;     // Total VRAM in MiB
    checked: boolean;    // Whether the check has been performed
}

// Create the base store
const { subscribe, set, update } = writable<NvidiaGPUStatus>({
    available: false,
    name: '',
    vramMiB: 0,
    checked: false
});

// Export the store with custom methods
export const nvidiaGPUStore = {
    subscribe,
    set: (value: NvidiaGPUStatus) => {
        logger.trace('store/nvidiaGPU', 'NVIDIA GPU status updated', value);
        set(value);
    },
    update,

    /**
     * Fetch NVIDIA GPU info from the backend and update the store
     */
    async refresh(): Promise<NvidiaGPUStatus> {
        try {
            logger.debug('store/nvidiaGPU', 'Fetching NVIDIA GPU info...');
            const info = await GetNvidiaGPUInfo();
            const status: NvidiaGPUStatus = {
                available: info.available,
                name: info.name,
                vramMiB: info.vramMiB,
                checked: true
            };
            set(status);
            logger.info('store/nvidiaGPU', 'NVIDIA GPU info retrieved', status);
            return status;
        } catch (error) {
            logger.error('store/nvidiaGPU', 'Failed to fetch NVIDIA GPU info', { error });
            const status: NvidiaGPUStatus = {
                available: false,
                name: '',
                vramMiB: 0,
                checked: true
            };
            set(status);
            return status;
        }
    },

    /**
     * Check if the available VRAM is sufficient for a given model
     * @param modelName The base model name (e.g., 'mel-roformer-kim', 'demucs')
     * @returns true if VRAM is sufficient, false otherwise
     */
    hasEnoughVRAM(modelName: string): boolean {
        const status = get({ subscribe });
        if (!status.available || status.vramMiB === 0) {
            return false;
        }
        const requiredVRAM = sepLibVRAMRequirements[modelName] || 0;
        return status.vramMiB >= requiredVRAM;
    },

    /**
     * Get the VRAM requirement for a given model in MiB
     * @param modelName The base model name (e.g., 'mel-roformer-kim', 'demucs')
     * @returns Required VRAM in MiB, or 0 if unknown
     */
    getRequiredVRAM(modelName: string): number {
        return sepLibVRAMRequirements[modelName] || 0;
    }
};

// Derived store for whether GPU acceleration should be auto-enabled
// Returns the model name that can be accelerated, or null if none
export const canAutoEnableGPU = derived(
    nvidiaGPUStore,
    ($gpu) => {
        if (!$gpu.checked || !$gpu.available) {
            return false;
        }
        // Check if we have enough VRAM for at least one model
        for (const [model, requiredVRAM] of Object.entries(sepLibVRAMRequirements)) {
            if ($gpu.vramMiB >= requiredVRAM) {
                return true;
            }
        }
        return false;
    }
);
