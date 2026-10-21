package cli

import (
	"strings"
	"testing"
)

// TestRunDeploy_PlanOutRequiresDryRun checks that --plan-out is rejected
// without --dry-run before any config is read: a saved plan only makes sense
// for a preview, and failing early keeps a mistyped invocation off the apply
// path.
func TestRunDeploy_PlanOutRequiresDryRun(t *testing.T) {
	deployPlanOut = "plan.txt"
	deployDryRun = false
	t.Cleanup(func() { deployPlanOut = "" })

	err := runDeploy(newTestCommand(t), nil)
	if err == nil {
		t.Fatal("runDeploy returned nil, want an error")
	}
	if !strings.Contains(err.Error(), "--plan-out requires --dry-run") {
		t.Errorf("error %q does not mention --plan-out requires --dry-run", err)
	}
}
