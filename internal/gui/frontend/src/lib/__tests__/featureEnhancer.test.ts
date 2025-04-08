// src/lib/__tests__/featureEnhancer.test.ts
import { describe, it, expect } from 'vitest';
import { enhanceFeaturesWithGroupOptions } from '../featureEnhancer';
import { groupOptionDefinitions } from '../groupOptions';
import type { FeatureDefinition } from '../featureModel'; // Import FeatureDefinition type

describe('Feature Enhancer', () => {
  it('should add group options to features', () => {
    // Create a simple test feature
    const testFeature: FeatureDefinition = { // Add type annotation
      id: 'testFeature',
      label: 'Test Feature',
      options: {},
      featureGroups: ['subtitle']
    };
    
    // Enhance the feature
    const [enhanced] = enhanceFeaturesWithGroupOptions([testFeature]);
    
    // Verify that group options were added
    expect(enhanced.options).toHaveProperty('style');
    expect(enhanced.options).toHaveProperty('provider');
    expect(enhanced.options).toHaveProperty('dockerRecreate');
    expect(enhanced.options).toHaveProperty('browserAccessURL');
  });
  
  it('should preserve feature-specific options', () => {
    // Create a test feature with its own options
    const testFeature: FeatureDefinition = { // Add type annotation
      id: 'testFeature',
      label: 'Test Feature',
      options: {
        customOption: {
          type: 'boolean',
          label: 'Custom Option',
          default: true
        }
      },
      featureGroups: ['subtitle']
    };
    
    // Enhance the feature
    const [enhanced] = enhanceFeaturesWithGroupOptions([testFeature]);
    
    // Verify that both custom and group options exist
    expect(enhanced.options).toHaveProperty('customOption');
    expect(enhanced.options).toHaveProperty('style');
  });
  
  it('should handle features with multiple groups', () => {
    // Create a test feature with multiple group memberships
    const testFeature: FeatureDefinition = { // Add type annotation
      id: 'testFeature',
      label: 'Test Feature',
      options: {},
      featureGroups: ['subtitle', 'merge']
    };
    
    // Enhance the feature
    const [enhanced] = enhanceFeaturesWithGroupOptions([testFeature]);
    
    // Verify that options from both groups were added
    expect(enhanced.options).toHaveProperty('style'); // From subtitle group
    expect(enhanced.options).toHaveProperty('mergeOutputFiles'); // From merge group
  });
  
  it('should handle features with no group membership', () => {
    // Create a test feature with no groups
    const testFeature: FeatureDefinition = { // Add type annotation
      id: 'testFeature',
      label: 'Test Feature',
      options: {
        customOption: {
          type: 'boolean',
          label: 'Custom Option',
          default: true
        }
      }
      // No featureGroups property
    };
    
    // Enhance the feature
    const [enhanced] = enhanceFeaturesWithGroupOptions([testFeature]);
    
    // Verify that the feature is unchanged
    expect(enhanced).toEqual(testFeature);
  });
});