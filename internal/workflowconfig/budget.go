package workflowconfig

import "time"

// BudgetCheckpointInterval is the fixed cadence for persisting active stage time.
const BudgetCheckpointInterval time.Duration = 5 * time.Second

// TerminationGracePeriod is the fixed wait for scoped work to stop after expiry.
const TerminationGracePeriod time.Duration = 15 * time.Second
