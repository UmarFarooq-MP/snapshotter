package main

import (
	"testing"
)

func TestReaderEpochFlow(t *testing.T) {
	var r Reader
	globalEpoch.Store(10)

	r.EnterRead()
	if r.epoch.Load() != 10 {
		t.Errorf("expected epoch=10, got %d", r.epoch.Load())
	}
	r.ExitRead()
	if r.epoch.Load() != 0 {
		t.Errorf("expected epoch=0 after ExitRead, got %d", r.epoch.Load())
	}
}
