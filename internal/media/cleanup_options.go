package media

import (
	"context"
)

// RegisteredIDsLister returns Hero session IDs registered in the durable store.
type RegisteredIDsLister func(context.Context) ([]string, error)

// CleanupOptionsFromRegistered builds cleanup options that retain directories
// for registered Hero session IDs.
func CleanupOptionsFromRegistered(dataHome string, list RegisteredIDsLister) CleanupOptions {
	opts := CleanupOptions{DataHome: dataHome}
	if list == nil {
		return opts
	}
	opts.ListRegistered = func() (RegisteredSessionIDs, error) {
		ids, err := list(context.Background())
		if err != nil {
			return nil, err
		}
		return RegisteredSessionIDSet(ids), nil
	}
	return opts
}
