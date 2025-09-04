/*
 * Simple caching library with expiration capabilities
 *     Copyright (c) 2013-2017, Christian Muehlhaeuser <muesli@gmail.com>
 *
 *   For license see LICENSE.txt
 */

package cache2go

import (
	"container/list"
	"log"
	"sync"
	"time"
)

// LFUNode represents a frequency node in the LFU cache
type LFUNode struct {
	frequency int
	items     *list.List // List of cache items with this frequency
}

// LFUCache implements Least Frequently Used cache algorithm
type LFUCache struct {
	sync.RWMutex

	// The cache's name
	name string
	// Maximum capacity of the cache
	capacity int
	// Current size
	size int

	// Map from key to cache item
	items map[interface{}]*CacheItem
	// Map from key to list element (for O(1) access)
	keyToListElement map[interface{}]*list.Element
	// Map from frequency to LFU node
	frequencies map[int]*LFUNode
	// Minimum frequency in the cache
	minFrequency int

	// The logger used for this cache
	logger *log.Logger

	// Callback method triggered when trying to load a non-existing key
	loadData func(key interface{}, args ...interface{}) *CacheItem
	// Callback method triggered when adding a new item to the cache
	addedItem []func(item *CacheItem)
	// Callback method triggered before deleting an item from the cache
	aboutToDeleteItem []func(item *CacheItem)
}

// NewLFUCache creates a new LFU cache with the specified capacity
func NewLFUCache(name string, capacity int) *LFUCache {
	return &LFUCache{
		name:             name,
		capacity:         capacity,
		size:             0,
		items:            make(map[interface{}]*CacheItem),
		keyToListElement: make(map[interface{}]*list.Element),
		frequencies:      make(map[int]*LFUNode),
		minFrequency:     0,
	}
}

// updateFrequency updates the frequency of an item
func (cache *LFUCache) updateFrequency(key interface{}) {
	item := cache.items[key]
	element := cache.keyToListElement[key]
	oldFreq := int(item.AccessCount())
	newFreq := oldFreq + 1

	// Remove from old frequency list
	if oldNode, exists := cache.frequencies[oldFreq]; exists {
		oldNode.items.Remove(element)
		if oldNode.items.Len() == 0 && oldFreq == cache.minFrequency {
			cache.minFrequency++
		}
	}

	// Add to new frequency list
	if _, exists := cache.frequencies[newFreq]; !exists {
		cache.frequencies[newFreq] = &LFUNode{
			frequency: newFreq,
			items:     list.New(),
		}
	}
	newElement := cache.frequencies[newFreq].items.PushFront(key)
	cache.keyToListElement[key] = newElement
}

// evictLFU removes the least frequently used item
func (cache *LFUCache) evictLFU() {
	if cache.size == 0 {
		return
	}

	// Find the LFU item
	minNode := cache.frequencies[cache.minFrequency]
	if minNode == nil || minNode.items.Len() == 0 {
		return
	}

	// Get the least recently used item among items with minimum frequency
	element := minNode.items.Back()
	key := element.Value
	
	// Remove from frequency list
	minNode.items.Remove(element)
	
	// Get the item before deletion for callbacks
	item := cache.items[key]
	
	// Trigger callbacks before deleting
	if cache.aboutToDeleteItem != nil {
		for _, callback := range cache.aboutToDeleteItem {
			callback(item)
		}
	}

	// Remove from cache
	delete(cache.items, key)
	delete(cache.keyToListElement, key)
	cache.size--

	cache.log("Evicted LFU item with key", key, "frequency", cache.minFrequency)
}

// Add adds a key/value pair to the LFU cache
func (cache *LFUCache) Add(key interface{}, lifeSpan time.Duration, data interface{}) *CacheItem {
	cache.Lock()
	defer cache.Unlock()

	// Check if item already exists
	if existingItem, exists := cache.items[key]; exists {
		// Update existing item
		existingItem.Lock()
		existingItem.data = data
		existingItem.lifeSpan = lifeSpan
		existingItem.accessedOn = time.Now()
		existingItem.accessCount++
		existingItem.Unlock()
		
		cache.updateFrequency(key)
		return existingItem
	}

	// Evict if at capacity
	if cache.size >= cache.capacity {
		cache.evictLFU()
	}

	// Create new item
	item := NewCacheItem(key, lifeSpan, data)
	cache.items[key] = item
	cache.size++

	// Add to frequency 1 list
	if _, exists := cache.frequencies[1]; !exists {
		cache.frequencies[1] = &LFUNode{
			frequency: 1,
			items:     list.New(),
		}
	}
	element := cache.frequencies[1].items.PushFront(key)
	cache.keyToListElement[key] = element
	cache.minFrequency = 1

	cache.log("Adding item with key", key, "to LFU cache", cache.name)

	// Trigger callbacks
	if cache.addedItem != nil {
		for _, callback := range cache.addedItem {
			callback(item)
		}
	}

	return item
}

// Value returns an item from the LFU cache and updates its frequency
func (cache *LFUCache) Value(key interface{}, args ...interface{}) (*CacheItem, error) {
	cache.Lock()
	defer cache.Unlock()

	if item, exists := cache.items[key]; exists {
		// Update access info
		item.KeepAlive()
		cache.updateFrequency(key)
		return item, nil
	}

	// Try data loader if available
	if cache.loadData != nil {
		cache.Unlock()
		item := cache.loadData(key, args...)
		cache.Lock()
		if item != nil {
			// Add the loaded item to cache
			if cache.size >= cache.capacity {
				cache.evictLFU()
			}
			cache.items[key] = item
			cache.size++

			// Add to frequency 1 list
			if _, exists := cache.frequencies[1]; !exists {
				cache.frequencies[1] = &LFUNode{
					frequency: 1,
					items:     list.New(),
				}
			}
			element := cache.frequencies[1].items.PushFront(key)
			cache.keyToListElement[key] = element
			cache.minFrequency = 1

			return item, nil
		}
		return nil, ErrKeyNotFoundOrLoadable
	}

	return nil, ErrKeyNotFound
}

// Delete removes an item from the LFU cache
func (cache *LFUCache) Delete(key interface{}) (*CacheItem, error) {
	cache.Lock()
	defer cache.Unlock()

	item, exists := cache.items[key]
	if !exists {
		return nil, ErrKeyNotFound
	}

	// Remove from frequency list
	element := cache.keyToListElement[key]
	freq := int(item.AccessCount())
	if node, exists := cache.frequencies[freq]; exists {
		node.items.Remove(element)
		if node.items.Len() == 0 && freq == cache.minFrequency {
			cache.minFrequency++
		}
	}

	// Trigger callbacks
	if cache.aboutToDeleteItem != nil {
		for _, callback := range cache.aboutToDeleteItem {
			callback(item)
		}
	}

	// Remove from cache
	delete(cache.items, key)
	delete(cache.keyToListElement, key)
	cache.size--

	cache.log("Deleted item with key", key, "from LFU cache", cache.name)
	return item, nil
}

// Exists checks if an item exists in the LFU cache without updating frequency
func (cache *LFUCache) Exists(key interface{}) bool {
	cache.RLock()
	defer cache.RUnlock()
	_, exists := cache.items[key]
	return exists
}

// Count returns the number of items in the LFU cache
func (cache *LFUCache) Count() int {
	cache.RLock()
	defer cache.RUnlock()
	return cache.size
}

// Capacity returns the maximum capacity of the LFU cache
func (cache *LFUCache) Capacity() int {
	return cache.capacity
}

// Flush removes all items from the LFU cache
func (cache *LFUCache) Flush() {
	cache.Lock()
	defer cache.Unlock()

	cache.log("Flushing LFU cache", cache.name)

	// Trigger callbacks for all items
	if cache.aboutToDeleteItem != nil {
		for _, item := range cache.items {
			for _, callback := range cache.aboutToDeleteItem {
				callback(item)
			}
		}
	}

	cache.items = make(map[interface{}]*CacheItem)
	cache.keyToListElement = make(map[interface{}]*list.Element)
	cache.frequencies = make(map[int]*LFUNode)
	cache.size = 0
	cache.minFrequency = 0
}

// SetDataLoader configures a data-loader callback
func (cache *LFUCache) SetDataLoader(f func(interface{}, ...interface{}) *CacheItem) {
	cache.Lock()
	defer cache.Unlock()
	cache.loadData = f
}

// SetAddedItemCallback configures a callback for when items are added
func (cache *LFUCache) SetAddedItemCallback(f func(*CacheItem)) {
	if len(cache.addedItem) > 0 {
		cache.RemoveAddedItemCallbacks()
	}
	cache.Lock()
	defer cache.Unlock()
	cache.addedItem = append(cache.addedItem, f)
}

// AddAddedItemCallback appends a new callback to the addedItem queue
func (cache *LFUCache) AddAddedItemCallback(f func(*CacheItem)) {
	cache.Lock()
	defer cache.Unlock()
	cache.addedItem = append(cache.addedItem, f)
}

// RemoveAddedItemCallbacks empties the added item callback queue
func (cache *LFUCache) RemoveAddedItemCallbacks() {
	cache.Lock()
	defer cache.Unlock()
	cache.addedItem = nil
}

// SetAboutToDeleteItemCallback configures a callback for when items are about to be deleted
func (cache *LFUCache) SetAboutToDeleteItemCallback(f func(*CacheItem)) {
	if len(cache.aboutToDeleteItem) > 0 {
		cache.RemoveAboutToDeleteItemCallback()
	}
	cache.Lock()
	defer cache.Unlock()
	cache.aboutToDeleteItem = append(cache.aboutToDeleteItem, f)
}

// AddAboutToDeleteItemCallback appends a new callback to the AboutToDeleteItem queue
func (cache *LFUCache) AddAboutToDeleteItemCallback(f func(*CacheItem)) {
	cache.Lock()
	defer cache.Unlock()
	cache.aboutToDeleteItem = append(cache.aboutToDeleteItem, f)
}

// RemoveAboutToDeleteItemCallback empties the about to delete item callback queue
func (cache *LFUCache) RemoveAboutToDeleteItemCallback() {
	cache.Lock()
	defer cache.Unlock()
	cache.aboutToDeleteItem = nil
}

// SetLogger sets the logger to be used by this LFU cache
func (cache *LFUCache) SetLogger(logger *log.Logger) {
	cache.Lock()
	defer cache.Unlock()
	cache.logger = logger
}

// Internal logging method for convenience
func (cache *LFUCache) log(v ...interface{}) {
	if cache.logger == nil {
		return
	}
	cache.logger.Println(v...)
}

// MostAccessed returns the most frequently accessed items
func (cache *LFUCache) MostAccessed(count int64) []*CacheItem {
	cache.RLock()
	defer cache.RUnlock()

	var result []*CacheItem
	collected := int64(0)

	// Iterate from highest frequency to lowest
	for freq := len(cache.frequencies); freq > 0 && collected < count; freq-- {
		if node, exists := cache.frequencies[freq]; exists {
			for element := node.items.Front(); element != nil && collected < count; element = element.Next() {
				key := element.Value
				if item, exists := cache.items[key]; exists {
					result = append(result, item)
					collected++
				}
			}
		}
	}

	return result
}

// Foreach iterates over all items in the LFU cache
func (cache *LFUCache) Foreach(trans func(key interface{}, item *CacheItem)) {
	cache.RLock()
	defer cache.RUnlock()

	for k, v := range cache.items {
		trans(k, v)
	}
}