package concurrency

import "testing"

func TestLRUCachePutGetAndEvict(t *testing.T) {
	cache := NewLRUCache(2)
	cache.Put("a", 1)
	cache.Put("b", 2)

	if got, ok := cache.Get("a"); !ok || got.(int) != 1 {
		t.Fatalf("Get(a) = %v, %v; want 1, true", got, ok)
	}

	cache.Put("c", 3)
	if _, ok := cache.Get("b"); ok {
		t.Fatal("expected b to be evicted as least recently used")
	}
	if got := cache.Size(); got != 2 {
		t.Fatalf("Size = %d, want 2", got)
	}
}

func TestLRUCacheUpdatesExistingValue(t *testing.T) {
	cache := NewLRUCache(2)
	cache.Put("a", 1)
	cache.Put("a", 10)

	got, ok := cache.Get("a")
	if !ok || got.(int) != 10 {
		t.Fatalf("Get(a) = %v, %v; want 10, true", got, ok)
	}
	if got := cache.Size(); got != 1 {
		t.Fatalf("Size = %d, want 1", got)
	}
}
