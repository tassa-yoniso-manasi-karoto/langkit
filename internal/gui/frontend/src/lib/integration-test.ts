// Integration testing utilities for WebSocket + Summary system
import { llmStateStore } from './stores';
import { LLMWebSocket } from './llm-websocket';
import { testSummaryGeneration } from './websocket-debug';
import { logger } from './logger';

export interface TestResult {
    success: boolean;
    duration: number;
    error?: string;
    details?: any;
}

export class IntegrationTester {
    private websocket: LLMWebSocket | null = null;
    private testResults: Map<string, TestResult> = new Map();

    // Test WebSocket connection establishment
    async testWebSocketConnection(): Promise<TestResult> {
        const startTime = Date.now();
        
        try {
            this.websocket = new LLMWebSocket();
            await this.websocket.connect();
            
            // Wait a bit to see if connection establishes
            await new Promise(resolve => setTimeout(resolve, 2000));
            
            const isConnected = this.websocket.isConnected();
            const stats = this.websocket.getConnectionStats();
            
            if (!isConnected) {
                throw new Error('WebSocket failed to connect within timeout');
            }
            
            const result: TestResult = {
                success: true,
                duration: Date.now() - startTime,
                details: { 
                    connectionStats: stats,
                    isConnected
                }
            };
            
            this.testResults.set('websocket_connection', result);
            logger.trace('integration-test', 'WebSocket connection test passed', result);
            
            return result;
        } catch (error) {
            const result: TestResult = {
                success: false,
                duration: Date.now() - startTime,
                error: error instanceof Error ? error.message : String(error)
            };
            
            this.testResults.set('websocket_connection', result);
            logger.error('integration-test', 'WebSocket connection test failed', result);
            
            return result;
        }
    }

    // Test LLM state reception
    async testLLMStateReception(): Promise<TestResult> {
        const startTime = Date.now();
        
        try {
            if (!this.websocket || !this.websocket.isConnected()) {
                throw new Error('WebSocket not connected');
            }
            
            // Subscribe to state store and wait for state updates
            let receivedState = false;
            let stateData: any = null;
            
            const unsubscribe = llmStateStore.subscribe(state => {
                if (state) {
                    receivedState = true;
                    stateData = state;
                    logger.trace('integration-test', 'Received LLM state:', state);
                }
            });
            
            // Wait up to 10 seconds for state update
            const timeout = 10000;
            const startWait = Date.now();
            
            while (!receivedState && (Date.now() - startWait) < timeout) {
                await new Promise(resolve => setTimeout(resolve, 100));
            }
            
            unsubscribe();
            
            if (!receivedState) {
                throw new Error('No LLM state received within timeout');
            }
            
            const result: TestResult = {
                success: true,
                duration: Date.now() - startTime,
                details: {
                    stateReceived: receivedState,
                    globalState: stateData?.globalState,
                    providerCount: Object.keys(stateData?.providerStatesSnapshot || {}).length
                }
            };
            
            this.testResults.set('llm_state_reception', result);
            logger.trace('integration-test', 'LLM state reception test passed', result);
            
            return result;
        } catch (error) {
            const result: TestResult = {
                success: false,
                duration: Date.now() - startTime,
                error: error instanceof Error ? error.message : String(error)
            };
            
            this.testResults.set('llm_state_reception', result);
            logger.error('integration-test', 'LLM state reception test failed', result);
            
            return result;
        }
    }

    // Test provider availability
    async testProviderAvailability(): Promise<TestResult> {
        const startTime = Date.now();
        
        try {
            const providers = await window.go.gui.App.GetAvailableSummaryProviders();
            
            const result: TestResult = {
                success: true,
                duration: Date.now() - startTime,
                details: {
                    available: providers.available,
                    providerCount: providers.names?.length || 0,
                    status: providers.status,
                    providers: providers.names
                }
            };
            
            this.testResults.set('provider_availability', result);
            logger.trace('integration-test', 'Provider availability test passed', result);
            
            return result;
        } catch (error) {
            const result: TestResult = {
                success: false,
                duration: Date.now() - startTime,
                error: error instanceof Error ? error.message : String(error)
            };
            
            this.testResults.set('provider_availability', result);
            logger.error('integration-test', 'Provider availability test failed', result);
            
            return result;
        }
    }

    // Test model fetching
    async testModelFetching(providerName: string): Promise<TestResult> {
        const startTime = Date.now();
        
        try {
            const models = await window.go.gui.App.GetAvailableSummaryModels(providerName);
            
            const result: TestResult = {
                success: true,
                duration: Date.now() - startTime,
                details: {
                    provider: providerName,
                    available: models.available,
                    modelCount: models.names?.length || 0,
                    models: models.names
                }
            };
            
            this.testResults.set(`model_fetching_${providerName}`, result);
            logger.trace('integration-test', `Model fetching test passed for ${providerName}`, result);
            
            return result;
        } catch (error) {
            const result: TestResult = {
                success: false,
                duration: Date.now() - startTime,
                error: error instanceof Error ? error.message : String(error)
            };
            
            this.testResults.set(`model_fetching_${providerName}`, result);
            logger.error('integration-test', `Model fetching test failed for ${providerName}`, result);
            
            return result;
        }
    }

    // Test summary generation
    async testSummaryGeneration(): Promise<TestResult> {
        const startTime = Date.now();
        
        try {
            const summary = await testSummaryGeneration();
            
            const result: TestResult = {
                success: true,
                duration: Date.now() - startTime,
                details: {
                    summaryLength: summary.length,
                    summary: summary.substring(0, 200) + (summary.length > 200 ? '...' : '')
                }
            };
            
            this.testResults.set('summary_generation', result);
            logger.trace('integration-test', 'Summary generation test passed', result);
            
            return result;
        } catch (error) {
            const result: TestResult = {
                success: false,
                duration: Date.now() - startTime,
                error: error instanceof Error ? error.message : String(error)
            };
            
            this.testResults.set('summary_generation', result);
            logger.error('integration-test', 'Summary generation test failed', result);
            
            return result;
        }
    }

    // Run all tests in sequence
    async runFullTestSuite(): Promise<Map<string, TestResult>> {
        logger.trace('integration-test', 'Starting full integration test suite');
        
        // Test 1: WebSocket Connection
        await this.testWebSocketConnection();
        
        // Test 2: LLM State Reception
        await this.testLLMStateReception();
        
        // Test 3: Provider Availability
        await this.testProviderAvailability();
        
        // Test 4: Model Fetching (if providers available)
        const providerResult = this.testResults.get('provider_availability');
        if (providerResult?.success && providerResult.details?.providers?.length > 0) {
            for (const provider of providerResult.details.providers) {
                await this.testModelFetching(provider);
            }
        }
        
        // Test 5: Summary Generation (if everything else works)
        const allPreviousSuccess = Array.from(this.testResults.values()).every(result => result.success);
        if (allPreviousSuccess) {
            await this.testSummaryGeneration();
        }
        
        this.cleanup();
        
        const summary = this.getTestSummary();
        logger.trace('integration-test', 'Integration test suite completed', summary);
        
        return this.testResults;
    }

    // Get test summary
    getTestSummary() {
        const results = Array.from(this.testResults.values());
        const passed = results.filter(r => r.success).length;
        const failed = results.filter(r => !r.success).length;
        const totalDuration = results.reduce((sum, r) => sum + r.duration, 0);
        
        return {
            total: results.length,
            passed,
            failed,
            successRate: results.length > 0 ? (passed / results.length * 100).toFixed(1) + '%' : '0%',
            totalDuration: totalDuration + 'ms',
            results: Object.fromEntries(this.testResults)
        };
    }

    // Cleanup resources
    cleanup() {
        if (this.websocket) {
            this.websocket.disconnect();
            this.websocket = null;
        }
    }
}

// Quick test function for manual testing
export async function quickIntegrationTest() {
    const tester = new IntegrationTester();
    const results = await tester.runFullTestSuite();
    const summary = tester.getTestSummary();
    
    console.log('=== Integration Test Results ===');
    console.log(summary);
    console.log('=== Detailed Results ===');
    console.log(results);
    
    return { results, summary };
}