/*
 * Simple caching library with expiration capabilities
 *     Copyright (c) 2012, Radu Ioan Fericean
 *                   2013-2017, Christian Muehlhaeuser <muesli@gmail.com>
 *
 *   For license see LICENSE.txt
 */

package cache2go

import (
	"sync"
)

var (
	cache = make(map[string]*CacheTable)
	lfuCaches = make(map[string]*LFUCache)
	mutex sync.RWMutex
)

// Cache returns the existing cache table with given name or creates a new one
// if the table does not exist yet.
func Cache(table string) *CacheTable {
	mutex.RLock()
	t, ok := cache[table]
	mutex.RUnlock()

	if !ok {
		mutex.Lock()
		t, ok = cache[table]
		// Double check whether the table exists or not.
		if !ok {
			t = &CacheTable{
				name:  table,
				items: make(map[interface{}]*CacheItem),
			}
			cache[table] = t
		}
		mutex.Unlock()
	}

	return t
}

// LFUCache returns the existing LFU cache with given name or creates a new one
// if the cache does not exist yet.
func LFUCache(name string, capacity int) *LFUCache {
	mutex.RLock()
	c, ok := lfuCaches[name]
	mutex.RUnlock()

	if !ok {
		mutex.Lock()
		c, ok = lfuCaches[name]
		// Double check whether the cache exists or not.
		if !ok {
			c = NewLFUCache(name, capacity)
			lfuCaches[name] = c
		}
		mutex.Unlock()
	}

	return c
}