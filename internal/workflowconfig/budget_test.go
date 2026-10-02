package workflowconfig_test

import (
	"testing"
	"time"

	"github.com/ricrsantos/ai_workflow_hero/internal/workflowconfig"
)

func TestBudgetConstantsMatchExecutionContract(t *testing.T) {
	if got, want := workflowconfig.BudgetCheckpointInterval, 5*time.Second; got != want {
		t.Fatalf("BudgetCheckpointInterval = %s, want %s", got, want)
	}
	if got, want := workflowconfig.TerminationGracePeriod, 15*time.Second; got != want {
		t.Fatalf("TerminationGracePeriod = %s, want %s", got, want)
	}
}
