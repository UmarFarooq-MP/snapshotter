package main

import "testing"

func TestRBTreeInsertFindDelete(t *testing.T) {
	tree := NewRBTree()
	pl1 := tree.UpsertLevel(100)
	if pl1 == nil {
		t.Fatal("UpsertLevel failed")
	}
	pl2 := tree.FindLevel(100)
	if pl2 != pl1 {
		t.Error("FindLevel did not return same PriceLevel")
	}

	tree.UpsertLevel(200)
	if tree.MinLevel().Price != 100 {
		t.Error("expected min=100")
	}
	if tree.MaxLevel().Price != 200 {
		t.Error("expected max=200")
	}

	if !tree.DeleteLevel(100) {
		t.Error("DeleteLevel failed")
	}
	if tree.FindLevel(100) != nil {
		t.Error("expected level 100 to be gone")
	}
}
