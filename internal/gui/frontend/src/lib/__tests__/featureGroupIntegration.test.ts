// src/lib/__tests__/featureGroupIntegration.test.ts
import { describe, it, expect, beforeEach, vi } from 'vitest';
import { enhanceFeaturesWithGroupOptions } from '../featureEnhancer';
import { groupOptionDefinitions, convertToFeatureOption } from '../groupOptions';
import { get } from 'svelte/store';
import { features as actualFeatures, type FeatureDefinition } from '../featureModel'; // Import FeatureDefinition

// Mock the featureGroupStore for testing purposes
vi.mock('../featureGroupStore', () => {
  const mockStore = {
    getGroupOption: (groupId: string, optionId: string) => {
      // Provide mock values or logic as needed for tests
      if (groupId === 'merge' && optionId === 'mergeOutputFiles') return false;
      return undefined;
    },
    isFeatureEnabled: () => false, // Mock implementation
    isTopmostForOption: () => false, // Mock implementation
    // Add other methods if needed by tests
  };
  return { featureGroupStore: mockStore };
});


describe('Feature Group System Integration', () => {
  // Mock feature definitions for testing
  const testFeatures: FeatureDefinition[] = [ // Add type annotation
    {
      id: 'feature1',
      label: 'Feature 1',
      options: {
        featureSpecificOption: {
          type: 'boolean',
          label: 'Feature-specific Option',
          default: false
        }
      },
      featureGroups: ['subtitle', 'merge']
    },
    {
      id: 'feature2',
      label: 'Feature 2',
      options: {},
      featureGroups: ['merge']
    }
  ];
  
  describe('Feature Enhancement', () => {
    it('should properly enhance features with group options', () => {
      const enhanced = enhanceFeaturesWithGroupOptions(testFeatures);
      
      // Check feature1 (belongs to subtitle and merge)
      const feature1 = enhanced.find(f => f.id === 'feature1');
      expect(feature1).toBeDefined();
      
      // Should have options from both groups
      expect(feature1?.options).toHaveProperty('style'); // from subtitle
      expect(feature1?.options).toHaveProperty('mergeOutputFiles'); // from merge
      
      // Should have initialized groupSharedOptions
      expect(feature1?.groupSharedOptions).toHaveProperty('subtitle');
      expect(feature1?.groupSharedOptions).toHaveProperty('merge');
      
      // Should preserve feature-specific options
      expect(feature1?.options).toHaveProperty('featureSpecificOption');
    });
    
    it('should set proper showCondition for group options', () => {
      const enhanced = enhanceFeaturesWithGroupOptions(testFeatures);
      const feature1 = enhanced.find(f => f.id === 'feature1');
      
      // Check that style option has proper showCondition
      expect(feature1?.options.style.showCondition).toBe('context.isTopmostForOption');
      
      // Check that mergingFormat has conditional display converted to showCondition
      expect(feature1?.options.mergingFormat.showCondition).toContain('context.isTopmostForOption');
      expect(feature1?.options.mergingFormat.showCondition).toContain("featureGroupStore.getGroupOption('merge', 'mergeOutputFiles') === true"); // Check specific condition
    });
    
    it('should ensure all options are listed in groupSharedOptions', () => {
      const enhanced = enhanceFeaturesWithGroupOptions(testFeatures);
      const feature1 = enhanced.find(f => f.id === 'feature1');
      
      // Get all subtitle options
      const subtitleOptions = Object.keys(groupOptionDefinitions.subtitle);
      
      // Check that all are included in groupSharedOptions
      subtitleOptions.forEach(optionId => {
        expect(feature1?.groupSharedOptions?.subtitle).toContain(optionId);
      });
    });
  });
  
  describe('Option Conversion', () => {
    it('should properly convert conditionalDisplay to showCondition', () => {
      const groupOption = {
        type: 'boolean',
        label: 'Test Option',
        default: false,
        conditionalDisplay: 'context.needsDocker'
      };
      
      const converted = convertToFeatureOption(groupOption as any); // Cast as any for test
      
      expect(converted.showCondition).toBe('context.isTopmostForOption && (context.needsDocker)');
    });
    
    it('should preserve all properties during conversion', () => {
      const groupOption = {
        type: 'number',
        label: 'Test Number',
        default: 10,
        min: 0,
        max: 100,
        step: '1',
        hovertip: 'Test tooltip',
        placeholder: 'Enter a number'
      };
      
      const converted = convertToFeatureOption(groupOption as any); // Cast as any for test
      
      expect(converted.type).toBe('number');
      expect(converted.label).toBe('Test Number');
      expect(converted.default).toBe(10);
      expect(converted.min).toBe(0);
      expect(converted.max).toBe(100);
      expect(converted.step).toBe('1');
      expect(converted.hovertip).toBe('Test tooltip');
      expect(converted.placeholder).toBe('Enter a number');
    });
  });
  
  describe('Real Feature Definitions', () => {
    it('should have properly enhanced all real features', () => {
      // Check a few specific features
      const subtitleRomanization = actualFeatures.find(f => f.id === 'subtitleRomanization');
      const dubtitles = actualFeatures.find(f => f.id === 'dubtitles');
      
      // Subtitle romanization should have subtitle and merge group options
      expect(subtitleRomanization?.options).toHaveProperty('style');
      expect(subtitleRomanization?.options).toHaveProperty('mergeOutputFiles');
      
      // Dubtitles should have merge group options
      expect(dubtitles?.options).toHaveProperty('mergeOutputFiles');
    });
    
    it('should ensure all features have initialized groupSharedOptions', () => {
      actualFeatures.forEach(feature => {
        if (feature.featureGroups && feature.featureGroups.length > 0) {
          expect(feature.groupSharedOptions).toBeDefined();
          
          feature.featureGroups.forEach(groupId => {
            expect(feature.groupSharedOptions?.[groupId]).toBeDefined(); // Use optional chaining
            expect(Array.isArray(feature.groupSharedOptions?.[groupId])).toBe(true); // Use optional chaining
          });
        }
      });
    });
  });
});