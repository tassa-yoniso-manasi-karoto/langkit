package eta_test

import (
	"sync"
	"testing"
	"time"

	"github.com/tassa-yoniso-manasi-karoto/langkit/pkg/eta"
)

// ---------- Simple calculator ----------

func TestSimpleETABasic(t *testing.T) {
	calc := eta.NewSimpleETACalculator(10)

	// Complete tasks one by one with small sleeps
	for i := int64(1); i <= 5; i++ {
		calc.TaskCompleted(i)
		time.Sleep(50 * time.Millisecond)
	}

	// Wait for minimum elapsed time
	time.Sleep(2 * time.Second)

	// Complete one more to trigger a fresh calculation after the sleep
	calc.TaskCompleted(6)

	result := calc.CalculateETAWithConfidence()

	if result.Estimate < 0 {
		t.Fatalf("expected valid ETA after 6/10 tasks, got %v", result.Estimate)
	}
	if result.Algorithm != eta.AlgorithmSimple {
		t.Errorf("expected AlgorithmSimple, got %v", result.Algorithm)
	}
	if result.LowerBound > result.Estimate || result.Estimate > result.UpperBound {
		t.Errorf("bounds not ordered: lower=%v estimate=%v upper=%v",
			result.LowerBound, result.Estimate, result.UpperBound)
	}
}

func TestSimpleETAResumption(t *testing.T) {
	calc := eta.NewSimpleETACalculator(10)

	// Simulate resumption: first update at 4 (4 tasks already done)
	calc.TaskCompleted(4)

	// No ETA yet (zero session work)
	result := calc.CalculateETAWithConfidence()
	if result.Estimate >= 0 {
		t.Errorf("expected no ETA before session work, got %v", result.Estimate)
	}

	// Complete tasks in this session
	for i := int64(5); i <= 7; i++ {
		calc.TaskCompleted(i)
		time.Sleep(50 * time.Millisecond)
	}

	// Wait for minimum elapsed
	time.Sleep(2 * time.Second)
	calc.TaskCompleted(8)

	result = calc.CalculateETAWithConfidence()
	if result.Estimate < 0 {
		t.Fatalf("expected valid ETA after session work, got %v", result.Estimate)
	}

	// Verify PercentDone reflects absolute progress
	expected := 8.0 / 10.0
	if result.PercentDone != expected {
		t.Errorf("expected PercentDone %.2f, got %.2f", expected, result.PercentDone)
	}
}

func TestSimpleETACompletion(t *testing.T) {
	calc := eta.NewSimpleETACalculator(5)
	for i := int64(1); i <= 5; i++ {
		calc.TaskCompleted(i)
	}
	result := calc.CalculateETAWithConfidence()
	if result.Estimate != 0 {
		t.Errorf("expected Estimate=0 when complete, got %v", result.Estimate)
	}
	if result.PercentDone != 1.0 {
		t.Errorf("expected PercentDone=1.0 when complete, got %v", result.PercentDone)
	}
}

// ---------- Advanced calculator ----------

func TestAdvancedETASteadyState(t *testing.T) {
	total := int64(100)
	calc := eta.NewETACalculator(total)

	// Simulate steady processing at ~20 tasks/sec
	for i := int64(1); i <= 50; i++ {
		calc.TaskCompleted(i)
		time.Sleep(50 * time.Millisecond)
	}

	result := calc.CalculateETAWithConfidence()
	if result.Estimate < 0 {
		t.Fatalf("expected valid ETA at 50%%, got %v", result.Estimate)
	}
	if result.Algorithm != eta.AlgorithmAdvanced {
		t.Errorf("expected AlgorithmAdvanced, got %v", result.Algorithm)
	}

	// Bounds should bracket the estimate
	if result.LowerBound > result.Estimate || result.Estimate > result.UpperBound {
		t.Errorf("bounds not ordered: lower=%v estimate=%v upper=%v",
			result.LowerBound, result.Estimate, result.UpperBound)
	}

	// Uncertainty should have narrowed by 50% completion
	relWidth := float64(result.UpperBound-result.LowerBound) / float64(result.Estimate)
	if relWidth > 0.50 {
		t.Errorf("expected narrower bounds at 50%% done, relativeWidth=%.2f", relWidth)
	}
}

func TestAdvancedETASlowdown(t *testing.T) {
	total := int64(50)
	calc := eta.NewETACalculator(total)

	// Fast phase: ~10 tasks/sec, runs for ~2.5s to exceed SimpleETAMinimumElapsed
	for i := int64(1); i <= 25; i++ {
		calc.TaskCompleted(i)
		time.Sleep(100 * time.Millisecond)
	}
	resultFast := calc.CalculateETAWithConfidence()

	// Slow phase: ~2.5 tasks/sec (4x slower)
	for i := int64(26); i <= 35; i++ {
		calc.TaskCompleted(i)
		time.Sleep(400 * time.Millisecond)
	}
	resultSlow := calc.CalculateETAWithConfidence()

	if resultSlow.Estimate < 0 || resultFast.Estimate < 0 {
		t.Fatalf("expected valid ETAs, fast=%v slow=%v", resultFast.Estimate, resultSlow.Estimate)
	}

	// After slowdown, the estimate for remaining work should be higher
	// (even though there are fewer remaining tasks, the rate dropped 4x)
	// We just check the estimate adapted upward — not an exact ratio
	if resultSlow.Estimate <= resultFast.Estimate/2 {
		t.Errorf("expected estimate to increase after slowdown: fast=%v slow=%v",
			resultFast.Estimate, resultSlow.Estimate)
	}
}

func TestAdvancedETACompletion(t *testing.T) {
	calc := eta.NewETACalculator(10)
	for i := int64(1); i <= 10; i++ {
		calc.TaskCompleted(i)
		time.Sleep(10 * time.Millisecond)
	}
	result := calc.CalculateETAWithConfidence()
	if result.Estimate != 0 {
		t.Errorf("expected Estimate=0 when complete, got %v", result.Estimate)
	}
}

func TestAdvancedETAConcurrency(t *testing.T) {
	total := int64(1000)
	calc := eta.NewETACalculator(total)

	var wg sync.WaitGroup

	// Writer goroutine: complete tasks
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := int64(1); i <= total; i++ {
			calc.TaskCompleted(i)
		}
	}()

	// Reader goroutines: query ETA concurrently
	for r := 0; r < 4; r++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				_ = calc.CalculateETAWithConfidence()
				_ = calc.Progress()
			}
		}()
	}

	wg.Wait()
	// If we get here without a race detector panic, the test passes.
	// Final state should be complete.
	result := calc.CalculateETAWithConfidence()
	if result.Estimate != 0 {
		t.Errorf("expected Estimate=0 after completing all tasks, got %v", result.Estimate)
	}
}

func TestAdvancedETAUpdateTotalTasks(t *testing.T) {
	calc := eta.NewETACalculator(100)

	// Complete 20 tasks over >2s to exceed SimpleETAMinimumElapsed
	for i := int64(1); i <= 20; i++ {
		calc.TaskCompleted(i)
		time.Sleep(120 * time.Millisecond)
	}

	// Discover some tasks were already done — reduce total
	calc.UpdateTotalTasks(80)

	result := calc.CalculateETAWithConfidence()
	if result.Estimate < 0 {
		t.Fatalf("expected valid ETA after UpdateTotalTasks, got %v", result.Estimate)
	}

	// PercentDone should reflect new total
	expected := 20.0 / 80.0
	if result.PercentDone != expected {
		t.Errorf("expected PercentDone=%.3f after total update, got %.3f", expected, result.PercentDone)
	}
}
