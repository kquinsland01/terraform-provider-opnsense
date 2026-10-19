// Copyright (c) Matthew Mellor
// SPDX-License-Identifier: MPL-2.0

package unbound

import "sync"

// unboundCreateMu keeps owner discovery, mutation, and ambiguous-outcome
// recovery in one provider-process critical section. The generic client mutex
// only covers the HTTP mutation itself, which is insufficient for ownership
// decisions made by the surrounding preflight and recovery searches.
var unboundCreateMu sync.Mutex

type unboundCreateLocker interface {
	Lock()
	Unlock()
}

func ownerCreateLocker(override unboundCreateLocker) unboundCreateLocker {
	if override != nil {
		return override
	}
	return &unboundCreateMu
}
