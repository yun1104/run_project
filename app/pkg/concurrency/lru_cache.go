package concurrency

import (
	"container/list"
	"sync"
)

type LRUCache struct {
	capacity int
	cache    map[string]*list.Element
	lruList  *list.List
	mu       sync.RWMutex
}

type entry struct {
	key   string
	value interface{}
}

func NewLRUCache(capacity int) *LRUCache {
	return &LRUCache{
		capacity: capacity,
		cache:    make(map[string]*list.Element),
		lruList:  list.New(),
	}
}

func (c *LRUCache) Get(key string) (interface{}, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	if elem, ok := c.cache[key]; ok {
		c.lruList.MoveToFront(elem)
		return elem.Value.(*entry).value, true
	}
	return nil, false
}

func (c *LRUCache) Put(key string, value interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	if elem, ok := c.cache[key]; ok {
		c.lruList.MoveToFront(elem)
		elem.Value.(*entry).value = value
		return
	}
	
	elem := c.lruList.PushFront(&entry{key, value})
	c.cache[key] = elem
	
	if c.lruList.Len() > c.capacity {
		oldest := c.lruList.Back()
		if oldest != nil {
			c.lruList.Remove(oldest)
			delete(c.cache, oldest.Value.(*entry).key)
		}
	}
}

func (c *LRUCache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.lruList.Len()
}
