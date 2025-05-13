package eta_test

import (
	"testing"
	"time"

	"github.com/tassa-yoniso-manasi-karoto/langkit/pkg/eta"
)

func TestETASimpleCalculatorResumption(t *testing.T) {
	// Create a calculator with 10 total tasks
	calc := eta.NewSimpleETACalculator(10)
	
	// Simulate resumption with 4 tasks already done
	calc.TaskCompleted(4)
	
	// Get the initial ETA
	result := calc.CalculateETAWithConfidence()
	
	// Expect no ETA yet (since we've done no tasks in this session)
	if result.Estimate >= 0 {
		t.Errorf("Expected no ETA result before any tasks completed in this session, got %v", result.Estimate)
	}
	
	// Verify algorithm type is set correctly
	if result.Algorithm != eta.AlgorithmSimple {
		t.Errorf("Expected AlgorithmSimple, got %v", result.Algorithm)
	}
	
	// Verify initial progress through completion percentage
	expected := 4.0 / 10.0
	if result.PercentDone != expected {
		t.Errorf("Expected PercentDone of %.2f, got %.2f", expected, result.PercentDone)
	}
	
	// Complete a task in this session
	calc.TaskCompleted(5)
	
	// Let some artificial time pass - need to wait at least 2 seconds per simpleETAMinimumElapsed
	time.Sleep(3 * time.Second)
	
	// Now we should get an ETA
	result = calc.CalculateETAWithConfidence()
	
	// Expect ETA to be calculated now (but only considering the 1 task done this session)
	if result.Estimate < 0 {
		t.Errorf("Expected valid ETA after a task completed in this session and time passed, got %v", result.Estimate)
	}
	
	// Verify algorithm type is still correct
	if result.Algorithm != eta.AlgorithmSimple {
		t.Errorf("Expected AlgorithmSimple, got %v", result.Algorithm)
	}
	
	// Verify cross-multiplication weight for simple algorithm
	if result.CrossMultWeight != 1.0 {
		t.Errorf("Expected CrossMultWeight of 1.0 for SimpleETACalculator, got %f", result.CrossMultWeight)
	}
}

func TestETAAdvancedCalculatorResumption(t *testing.T) {
	// Create a calculator with 10 total tasks
	calc := eta.NewETACalculator(10)
	
	// Simulate resumption with 4 tasks already done
	calc.TaskCompleted(4)
	
	// Get the initial ETA
	result := calc.CalculateETAWithConfidence()
	
	// Verify algorithm type is set correctly
	if result.Algorithm != eta.AlgorithmAdvanced {
		t.Errorf("Expected AlgorithmAdvanced, got %v", result.Algorithm)
	}
	
	// Verify that with just one sample, we don't get an ETA yet
	if result.Estimate >= 0 {
		t.Errorf("Expected no ETA with only one sample point, got %v", result.Estimate)
	}
	
	// Complete another task
	calc.TaskCompleted(5)
	
	// Let some artificial time pass
	time.Sleep(10 * time.Millisecond)
	
	// Now we should have two samples
	result = calc.CalculateETAWithConfidence()
	
	// Expect ETA to still be invalid until we get more samples or time
	if result.Estimate >= 0 && len(calc.CalculateETAWithConfidence().RatesPerSec) < 4 {
		t.Errorf("Expected no ETA with only %d sample points, got %v", 
			len(calc.CalculateETAWithConfidence().RatesPerSec), result.Estimate)
	}
	
	// Complete more tasks to get enough samples
	for i := 6; i <= 8; i++ {
		calc.TaskCompleted(int64(i))
		time.Sleep(10 * time.Millisecond)
	}
	
	// Now we should get an ETA
	result = calc.CalculateETAWithConfidence()
	
	// Verify algorithm type is still correct
	if result.Algorithm != eta.AlgorithmAdvanced {
		t.Errorf("Expected AlgorithmAdvanced, got %v", result.Algorithm)
	}
}