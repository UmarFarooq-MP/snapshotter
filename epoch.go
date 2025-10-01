package main

import "sync/atomic"

/************** Epoch bookkeeping (RCU-style) **************/

var globalEpoch atomic.Uint64 // advanced by matcher only

type Reader struct{ epoch atomic.Uint64 } // 0 => not reading

func (r *Reader) EnterRead() { r.epoch.Store(globalEpoch.Load()) }
func (r *Reader) ExitRead()  { r.epoch.Store(0) }

func minReaderEpoch(rs ...*Reader) uint64 {
	min := ^uint64(0)
	for _, r := range rs {
		e := r.epoch.Load()
		if e != 0 && e < min {
			min = e
		}
	}
	return min
}
