// src/lib/groupValidation.ts
import { features } from './featureModel';
import { groupOptionDefinitions } from './groupOptions';
import { errorStore } from './errorStore';
import { logStore } from './logStore';

/**
 * Validation error types for different aspects of the group system
 */
export enum ValidationErrorType {
  MISSING_GROUP = 'missing-group',
  MISSING_OPTION = 'missing-option',
  TYPE_MISMATCH = 'type-mismatch',
  OPTION_REDEFINITION = 'option-redefinition',
  INVALID_REFERENCE = 'invalid-reference',
  ORDER_INCONSISTENCY = 'order-inconsistency'
}

/**
 * Validation error interface for structured error reporting
 */
export interface ValidationError {
  type: ValidationErrorType;
  message: string;
  featureId?: string;
  groupId?: string;
  optionId?: string;
  severity: 'error' | 'warning';
}

/**
 * Feature Group System validator - detects issues in the system configuration
 */
export class GroupSystemValidator {
  private errors: ValidationError[] = [];
  
  /**
   * Run all validation checks on the feature group system
   * @param silent If true, will not log errors to console
   * @returns Array of validation errors
   */
  validateSystem(silent: boolean = false): ValidationError[] {
    this.errors = [];
    
    // Clear previous errors first
    errorStore.clearErrorsOfType('warning');
    
    // Run all validation checks
    this.validateFeatureGroups();
    this.validateGroupOptions();
    this.validateOptionTypes();
    this.validateOptionOrder();
    this.validateRedundantOptions();
    
    // Log errors if not in silent mode
    if (!silent && this.errors.length > 0) {
      console.group('Feature Group System Validation Errors');
      
      console.log(`Found ${this.errors.length} issues:`);
      console.table(this.errors.map(err => ({
        type: err.type,
        feature: err.featureId || '-',
        group: err.groupId || '-',
        option: err.optionId || '-',
        message: err.message,
        severity: err.severity
      })));
      
      console.groupEnd();
      
      // Add serious errors to the error store
      const seriousErrors = this.errors.filter(err => err.severity === 'error');
      if (seriousErrors.length > 0) {
        errorStore.addError({
          id: 'group-system-validation',
          message: `${seriousErrors.length} group system errors detected. Check console for details.`,
          severity: 'warning',
          dismissible: true
        });
        
        // Log to the log store for persistence
        logStore.addLog({
          level: 'WARN',
          message: `Feature Group System validation found ${seriousErrors.length} serious errors`,
          time: new Date().toISOString()
        });
      }
    }
    
    return this.errors;
  }
  
  /**
   * Validate that all feature groups referenced in features exist
   */
  private validateFeatureGroups(): void {
    features.forEach(feature => {
      if (!feature.featureGroups) return;
      
      feature.featureGroups.forEach(groupId => {
        if (!groupOptionDefinitions[groupId]) {
          this.errors.push({
            type: ValidationErrorType.MISSING_GROUP,
            message: `Feature references undefined group: ${groupId}`,
            featureId: feature.id,
            groupId,
            severity: 'error'
          });
        }
      });
    });
  }
  
  /**
   * Validate that all group options referenced in features exist
   */
  private validateGroupOptions(): void {
    features.forEach(feature => {
      if (!feature.featureGroups || !feature.groupSharedOptions) return;
      
      Object.entries(feature.groupSharedOptions).forEach(([groupId, options]) => {
        if (!groupOptionDefinitions[groupId]) return; // Already captured in validateFeatureGroups
        
        options.forEach(optionId => {
          if (!groupOptionDefinitions[groupId][optionId]) {
            this.errors.push({
              type: ValidationErrorType.MISSING_OPTION,
              message: `Feature references undefined option ${optionId} in group ${groupId}`,
              featureId: feature.id,
              groupId,
              optionId,
              severity: 'error'
            });
          }
        });
      });
    });
  }
  
  /**
   * Validate that option types match between group definitions and feature definitions
   */
  private validateOptionTypes(): void {
    features.forEach(feature => {
      if (!feature.options) return;
      
      // Check each option in the feature
      Object.entries(feature.options).forEach(([optionId, optionDef]) => {
        // Check if it's a group option
        const groupId = this.findGroupForOption(feature, optionId);
        if (!groupId) return; // Not a group option
        
        // Get the group definition
        const groupOptionDef = groupOptionDefinitions[groupId][optionId];
        if (!groupOptionDef) return; // Already captured in validateGroupOptions
        
        // Check if types match
        if (optionDef.type !== groupOptionDef.type) {
          this.errors.push({
            type: ValidationErrorType.TYPE_MISMATCH,
            message: `Option type mismatch: feature defines ${optionId} as ${optionDef.type}, but group defines it as ${groupOptionDef.type}`,
            featureId: feature.id,
            groupId,
            optionId,
            severity: 'error'
          });
        }
      });
    });
  }
  
  /**
   * Validate option order consistency
   */
  private validateOptionOrder(): void {
    features.forEach(feature => {
      if (!feature.optionOrder || !feature.groupSharedOptions) return;
      
      // For each group the feature belongs to
      Object.entries(feature.groupSharedOptions).forEach(([groupId, options]) => {
        // Check if all group options are in the option order
        options.forEach(optionId => {
          if (!feature.optionOrder?.includes(optionId)) {
            this.errors.push({
              type: ValidationErrorType.ORDER_INCONSISTENCY,
              message: `Group option ${optionId} is not included in feature's optionOrder array`,
              featureId: feature.id,
              groupId,
              optionId,
              severity: 'warning'
            });
          }
        });
      });
    });
  }
  
  /**
   * Validate that features don't define redundant options that are already defined by groups
   */
  private validateRedundantOptions(): void {
    features.forEach(feature => {
      if (!feature.options || !feature.groupSharedOptions) return;
      
      // Check each option in the feature
      Object.entries(feature.options).forEach(([optionId, _]) => {
        // Check if it's a group option
        const groupId = this.findGroupForOption(feature, optionId);
        if (!groupId) return; // Not a group option
        
        // This is redundant definition, warn about it
        this.errors.push({
          type: ValidationErrorType.OPTION_REDEFINITION,
          message: `Feature defines option ${optionId} redundantly. This option is already defined by group ${groupId}`,
          featureId: feature.id,
          groupId,
          optionId,
          severity: 'warning'
        });
      });
    });
  }
  
  /**
   * Find which group an option belongs to for a feature
   */
  private findGroupForOption(feature: any, optionId: string): string | null {
    if (!feature.groupSharedOptions) return null;
    
    for (const [groupId, options] of Object.entries(feature.groupSharedOptions)) {
      if ((options as string[]).includes(optionId)) {
        return groupId;
      }
    }
    
    return null;
  }
}

/**
 * Global validator instance for easy access
 */
export const groupValidator = new GroupSystemValidator();

/**
 * Run validation and return results
 * @param silent If true, will not log errors to console
 * @returns Validation results
 */
export function validateGroupSystem(silent: boolean = false): ValidationError[] {
  return groupValidator.validateSystem(silent);
}