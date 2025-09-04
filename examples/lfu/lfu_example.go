package main

import (
	"fmt"
	"time"

	"github.com/muesli/cache2go"
)

func main() {
	// Create a new LFU cache with capacity of 3 items
	cache := cache2go.NewLFUCache("myLFUCache", 3)

	// Set up callbacks to monitor cache behavior
	cache.SetAddedItemCallback(func(item *cache2go.CacheItem) {
		fmt.Printf("Added: %v (frequency: %d)\n", item.Key(), item.AccessCount())
	})

	cache.SetAboutToDeleteItemCallback(func(item *cache2go.CacheItem) {
		fmt.Printf("Evicting: %v (frequency: %d)\n", item.Key(), item.AccessCount())
	})

	fmt.Println("=== LFU Cache Example ===")
	fmt.Printf("Cache capacity: %d\n\n", cache.Capacity())

	// Add items to the cache
	fmt.Println("1. Adding items to cache:")
	cache.Add("apple", 0, "A red fruit")
	cache.Add("banana", 0, "A yellow fruit")
	cache.Add("cherry", 0, "A small red fruit")

	fmt.Printf("Cache size: %d/%d\n\n", cache.Count(), cache.Capacity())

	// Access items with different frequencies
	fmt.Println("2. Accessing items to create frequency differences:")
	
	// Access apple 3 times
	fmt.Println("Accessing 'apple' 3 times...")
	for i := 0; i < 3; i++ {
		cache.Value("apple")
	}

	// Access banana 1 time
	fmt.Println("Accessing 'banana' 1 time...")
	cache.Value("banana")

	// Access cherry 2 times
	fmt.Println("Accessing 'cherry' 2 times...")
	for i := 0; i < 2; i++ {
		cache.Value("cherry")
	}

	fmt.Println()

	// Show current frequencies
	fmt.Println("3. Current access frequencies:")
	cache.Foreach(func(key interface{}, item *cache2go.CacheItem) {
		fmt.Printf("  %v: %d accesses\n", key, item.AccessCount())
	})

	fmt.Println()

	// Add a fourth item to trigger LFU eviction
	fmt.Println("4. Adding 'date' (should evict least frequently used item):")
	cache.Add("date", 0, "A sweet fruit")

	fmt.Printf("Cache size after eviction: %d/%d\n\n", cache.Count(), cache.Capacity())

	// Show remaining items
	fmt.Println("5. Remaining items in cache:")
	cache.Foreach(func(key interface{}, item *cache2go.CacheItem) {
		fmt.Printf("  %v: %v (accessed %d times)\n", key, item.Data(), item.AccessCount())
	})

	fmt.Println()

	// Demonstrate most accessed items
	fmt.Println("6. Most accessed items:")
	mostAccessed := cache.MostAccessed(3)
	for i, item := range mostAccessed {
		fmt.Printf("  #%d: %v (accessed %d times)\n", i+1, item.Key(), item.AccessCount())
	}

	fmt.Println()

	// Test data loader
	fmt.Println("7. Testing data loader:")
	cache.SetDataLoader(func(key interface{}, args ...interface{}) *cache2go.CacheItem {
		if key.(string) == "grape" {
			fmt.Printf("Data loader: Loading '%v' from external source\n", key)
			return cache2go.NewCacheItem(key, 0, "A purple fruit (loaded)")
		}
		return nil
	})

	// Try to access a non-existing item that can be loaded
	item, err := cache.Value("grape")
	if err == nil {
		fmt.Printf("Loaded item: %v = %v\n", item.Key(), item.Data())
	}

	fmt.Printf("Final cache size: %d/%d\n", cache.Count(), cache.Capacity())
}