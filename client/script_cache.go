package client

import (
	"errors"
	"github.com/nervosnetwork/ckb-sdk-go/v2/types"
	"perun.network/go-perun/channel"
	"polycry.pt/poly-go/sync"
)

// StableScriptCache is a concurrently safe cache for scripts.
// It is stable in the sense that it does not allow overwriting of scripts.
type StableScriptCache interface {
	// Set stores the script for the given channel ID.
	// It errors if the cache already holds a script for the given ID that is different with respect to
	// types.Script.Equals
	Set(id channel.ID, script *types.Script) error

	// Get returns the script for the given channel ID and true, iff there is a cache entry for that channel id.
	// If there is no cache entry, the second return value is false.
	Get(id channel.ID) (*types.Script, bool)
}

type scriptCache struct {
	cacheLock sync.Mutex
	cache     map[channel.ID]*types.Script
}

func NewStableScriptCache() StableScriptCache {
	return &scriptCache{
		cache: make(map[channel.ID]*types.Script),
	}
}

func (c *scriptCache) Get(id channel.ID) (*types.Script, bool) {
	c.cacheLock.Lock()
	defer c.cacheLock.Unlock()
	script, cached := c.cache[id]
	return script, cached
}

func (c *scriptCache) Set(id channel.ID, script *types.Script) error {
	c.cacheLock.Lock()
	defer c.cacheLock.Unlock()
	old, cached := c.cache[id]
	if cached {
		if old.Equals(script) {
			return nil
		}
		return errors.New("rewrite on constant cache")
	}
	c.cache[id] = script
	return nil
}
