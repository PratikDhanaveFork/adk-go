// Copyright 2026 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package triggers

import "testing"

// TestNewPubSubControllerWithConfig_SemaphoreHonorsMaxConcurrentRuns is a
// white-box regression test: the PubSub controller must size its concurrency
// semaphore from TriggerConfig.MaxConcurrentRuns, exactly as the Eventarc
// controller does. Before the fix the field was left nil, so the
// `if c.semaphore != nil` guard in PubSubTriggerHandler never fired and
// MaxConcurrentRuns was silently ignored (unbounded concurrent agent runs).
func TestNewPubSubControllerWithConfig_SemaphoreHonorsMaxConcurrentRuns(t *testing.T) {
	const want = 4
	cfg := ControllerConfig{TriggerConfig: TriggerConfig{MaxConcurrentRuns: want}}

	ps, err := NewPubSubControllerWithConfig(cfg)
	if err != nil {
		t.Fatalf("NewPubSubControllerWithConfig: %v", err)
	}
	if got := cap(ps.semaphore); got != want {
		t.Errorf("pubsub semaphore cap = %d, want %d (MaxConcurrentRuns ignored)", got, want)
	}

	// Parity with the sibling controller, which already initializes correctly.
	ev, err := NewEventarcControllerWithConfig(cfg)
	if err != nil {
		t.Fatalf("NewEventarcControllerWithConfig: %v", err)
	}
	if got := cap(ev.semaphore); got != want {
		t.Errorf("eventarc semaphore cap = %d, want %d", got, want)
	}
}
