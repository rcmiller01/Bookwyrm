package launcher

import (
	"testing"
	"time"
)

func TestRestartPolicy_AllowsWithinLimitThenBlocks(t *testing.T) {
	policy := RestartPolicy{
		Limit:     3,
		Window:    1 * time.Minute,
		BaseDelay: 1 * time.Second,
		MaxDelay:  8 * time.Second,
	}
	now := time.Now().UTC()
	for i := 0; i < 3; i++ {
		ok, delay := policy.AllowRestart(now.Add(time.Duration(i) * time.Second))
		if !ok {
			t.Fatalf("expected restart %d to be allowed", i+1)
		}
		if delay <= 0 {
			t.Fatalf("expected positive delay")
		}
	}
	ok, _ := policy.AllowRestart(now.Add(4 * time.Second))
	if ok {
		t.Fatalf("expected restart to be blocked after limit")
	}
}
