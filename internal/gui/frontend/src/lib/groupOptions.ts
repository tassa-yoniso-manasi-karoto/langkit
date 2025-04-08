// src/lib/groupOptions.ts
import type { FeatureOption } from './featureModel';

/**
 * Interface for group option definitions with an additional conditionalDisplay field
 * This extends FeatureOption to maintain compatibility while adding group-specific properties
 */
export interface GroupOptionDefinition extends Omit<FeatureOption, 'showCondition'> {
  conditionalDisplay?: string; // Optional condition beyond isTopmostForOption
}

/**
 * Centralized definitions for all group-shared options.
 * This is the single source of truth for group option properties.
 */
export const groupOptionDefinitions: Record<string, Record<string, GroupOptionDefinition>> = {
  // Subtitle Group Options
  subtitle: {
    style: {
      type: 'romanizationDropdown',
      label: 'Romanization Style',
      default: '',
    },
    provider: {
      type: 'provider',
      label: 'Provider',
      default: '',
      conditionalDisplay: "context.romanizationSchemes.length > 0"
    },
    dockerRecreate: {
      type: 'boolean',
      label: 'Recreate Docker containers',
      default: false,
      hovertip: "Use this if the previous run failed or if you're experiencing issues.",
      conditionalDisplay: "context.needsDocker"
    },
    browserAccessURL: {
      type: 'string',
      label: 'Browser access URL',
      default: '',
      hovertip: "Optional URL to a Chromium-based browser's DevTools interface.\nIf not provided or invalid, a browser will be automatically downloaded and managed.\n\nTo get a URL: Run Chrome/Chromium with:\n --remote-debugging-port=9222 flag\nand use the WebSocket URL displayed in the terminal or in:\nchrome://inspect/#devices",
      placeholder: "e.g. ws://127.0.0.1:9222/devtools/browser/xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx",
      conditionalDisplay: "context.needsScraper"
    }
  },
  
  // Merge Group Options
  merge: {
    mergeOutputFiles: {
      type: 'boolean',
      label: 'Merge all processed outputs',
      default: false,
      hovertip: "When enabled, all processed outputs (dubtitles, enhanced audio, romanized subtitles, etc.) will be merged into a single video file."
    },
    mergingFormat: {
      type: 'dropdown',
      label: 'Merging Format',
      default: 'mp4',
      choices: ['mp4', 'mkv'],
      conditionalDisplay: "featureGroupStore.getGroupOption('merge', 'mergeOutputFiles') === true"
    }
  }
};

/**
 * Convert a GroupOptionDefinition to a FeatureOption by adding the proper showCondition
 * @param optionDef The group option definition
 * @returns A FeatureOption compatible with the existing system
 */
export function convertToFeatureOption(optionDef: GroupOptionDefinition): FeatureOption {
  const featureOption: FeatureOption = {
    type: optionDef.type,
    label: optionDef.label,
    default: optionDef.default,
  };
  
  // Copy all optional properties
  if (optionDef.min !== undefined) featureOption.min = optionDef.min;
  if (optionDef.max !== undefined) featureOption.max = optionDef.max;
  if (optionDef.step !== undefined) featureOption.step = optionDef.step;
  if (optionDef.choices !== undefined) featureOption.choices = optionDef.choices;
  if (optionDef.hovertip !== undefined) featureOption.hovertip = optionDef.hovertip;
  if (optionDef.placeholder !== undefined) featureOption.placeholder = optionDef.placeholder;
  
  // Convert conditionalDisplay to showCondition with isTopmostForOption check
  if (optionDef.conditionalDisplay) {
    featureOption.showCondition = `context.isTopmostForOption && (${optionDef.conditionalDisplay})`; // Corrected encoding
  } else {
    featureOption.showCondition = "context.isTopmostForOption";
  }
  
  return featureOption;
}

/**
 * Get all option IDs defined for a specific group
 * @param groupId The group ID
 * @returns Array of option IDs
 */
export function getGroupOptionIds(groupId: string): string[] {
  const groupOptions = groupOptionDefinitions[groupId];
  return groupOptions ? Object.keys(groupOptions) : [];
}

/**
 * Get all group IDs defined in the system
 * @returns Array of group IDs
 */
export function getAllGroupIds(): string[] {
  return Object.keys(groupOptionDefinitions);
}