/*
 * Simple caching library with expiration capabilities
 *     Copyright (c) 2013-2017, Christian Muehlhaeuser <muesli@gmail.com>
 *
 *   For license see LICENSE.txt
 */

package cache2go

import (
	"testing"
	"time"
)

func TestLFUBasicOperations(t *testing.T) {
	cache := NewLFUCache("testLFU", 3)

	// Test Add
	item1 := cache.Add("key1", 0, "value1")
	if item1 == nil {
		t.Error("Failed to add item to LFU cache")
	}

	// Test Exists
	if !cache.Exists("key1") {
		t.Error("Item should exist in cache")
	}

	// Test Value
	retrieved, err := cache.Value("key1")
	if err != nil || retrieved.Data().(string) != "value1" {
		t.Error("Failed to retrieve item from LFU cache")
	}

	// Test Count
	if cache.Count() != 1 {
		t.Error("Cache count should be 1")
	}

	// Test Delete
	deleted, err := cache.Delete("key1")
	if err != nil || deleted == nil {
		t.Error("Failed to delete item from LFU cache")
	}

	if cache.Exists("key1") {
		t.Error("Item should not exist after deletion")
	}
}

func TestLFUEviction(t *testing.T) {
	cache := NewLFUCache("testLFUEviction", 2)

	// Add items to fill capacity
	cache.Add("key1", 0, "value1")
	cache.Add("key2", 0, "value2")

	// Access key1 multiple times to increase its frequency
	cache.Value("key1")
	cache.Value("key1")
	cache.Value("key1")

	// Access key2 once
	cache.Value("key2")

	// Add a third item, should evict key2 (lower frequency)
	cache.Add("key3", 0, "value3")

	// key1 should still exist (higher frequency)
	if !cache.Exists("key1") {
		t.Error("key1 should still exist (higher frequency)")
	}

	// key2 should be evicted (lower frequency)
	if cache.Exists("key2") {
		t.Error("key2 should be evicted (lower frequency)")
	}

	// key3 should exist (newly added)
	if !cache.Exists("key3") {
		t.Error("key3 should exist (newly added)")
	}

	if cache.Count() != 2 {
		t.Error("Cache should contain exactly 2 items after eviction")
	}
}

func TestLFUEvictionSameFrequency(t *testing.T) {
	cache := NewLFUCache("testLFUEvictionSame", 2)

	// Add two items
	cache.Add("key1", 0, "value1")
	cache.Add("key2", 0, "value2")

	// Access both once (same frequency)
	cache.Value("key1")
	cache.Value("key2")

	// Add a third item, should evict the least recently used among same frequency
	cache.Add("key3", 0, "value3")

	// Should have exactly 2 items
	if cache.Count() != 2 {
		t.Error("Cache should contain exactly 2 items")
	}

	// key3 should definitely exist
	if !cache.Exists("key3") {
		t.Error("key3 should exist (newly added)")
	}
}

func TestLFUFlush(t *testing.T) {
	cache := NewLFUCache("testLFUFlush", 5)

	// Add multiple items
	for i := 0; i < 5; i++ {
		cache.Add(i, 0, i*10)
	}

	if cache.Count() != 5 {
		t.Error("Cache should contain 5 items before flush")
	}

	// Flush the cache
	cache.Flush()

	if cache.Count() != 0 {
		t.Error("Cache should be empty after flush")
	}

	// Verify no items exist
	for i := 0; i < 5; i++ {
		if cache.Exists(i) {
			t.Error("No items should exist after flush")
		}
	}
}

func TestLFUMostAccessed(t *testing.T) {
	cache := NewLFUCache("testLFUMostAccessed", 5)

	// Add items with different access patterns
	cache.Add("key1", 0, "value1")
	cache.Add("key2", 0, "value2")
	cache.Add("key3", 0, "value3")

	// Create different access frequencies
	// key1: 3 accesses, key2: 1 access, key3: 2 accesses
	cache.Value("key1")
	cache.Value("key1")
	cache.Value("key1")

	cache.Value("key2")

	cache.Value("key3")
	cache.Value("key3")

	// Get most accessed items
	mostAccessed := cache.MostAccessed(3)

	if len(mostAccessed) != 3 {
		t.Error("Should return 3 most accessed items")
	}

	// Should be ordered by frequency: key1 (3), key3 (2), key2 (1)
	if mostAccessed[0].Key() != "key1" {
		t.Error("Most accessed item should be key1")
	}
}

func TestLFUCallbacks(t *testing.T) {
	cache := NewLFUCache("testLFUCallbacks", 2)

	addedKey := ""
	deletedKey := ""

	// Set up callbacks
	cache.SetAddedItemCallback(func(item *CacheItem) {
		addedKey = item.Key().(string)
	})

	cache.SetAboutToDeleteItemCallback(func(item *CacheItem) {
		deletedKey = item.Key().(string)
	})

	// Add an item
	cache.Add("testKey", 0, "testValue")

	if addedKey != "testKey" {
		t.Error("AddedItem callback not triggered correctly")
	}

	// Fill cache to trigger eviction
	cache.Add("key1", 0, "value1")
	cache.Add("key2", 0, "value2") // This should trigger eviction

	if deletedKey != "testKey" {
		t.Error("AboutToDeleteItem callback not triggered correctly during eviction")
	}
}

func TestLFUDataLoader(t *testing.T) {
	cache := NewLFUCache("testLFUDataLoader", 3)

	// Set up data loader
	cache.SetDataLoader(func(key interface{}, args ...interface{}) *CacheItem {
		if key.(string) == "loadable" {
			return NewCacheItem(key, 0, "loaded_value")
		}
		return nil
	})

	// Test loading existing key
	item, err := cache.Value("loadable")
	if err != nil || item.Data().(string) != "loaded_value" {
		t.Error("Data loader should load the item")
	}

	// Test loading non-existing key
	_, err = cache.Value("non_loadable")
	if err != ErrKeyNotFoundOrLoadable {
		t.Error("Should return ErrKeyNotFoundOrLoadable for non-loadable keys")
	}
}