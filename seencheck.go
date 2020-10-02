package main

import (
	"sync"

	"github.com/paulbellamy/ratecounter"
	"github.com/philippgille/gokv/leveldb"
)

type Seencheck struct {
	Mutex     *sync.Mutex
	SeenRate  *ratecounter.RateCounter
	SeenCount *ratecounter.Counter
	SeenDB    leveldb.Store
	WriteChan chan *Item
}

func (seencheck *Seencheck) IsSeen(ID string) bool {
	var retrievedValue = new(bool)

	found, err := seencheck.SeenDB.Get(ID, &retrievedValue)
	if err != nil {
		panic(err)
	}

	if !found {
		return false
	}

	return true
}

func (seencheck *Seencheck) Seen(item *Item) {
	seencheck.SeenDB.Set(item.ID, true)
	seencheck.WriteChan <- item
	seencheck.SeenCount.Incr(1)
	seencheck.SeenRate.Incr(1)
}
