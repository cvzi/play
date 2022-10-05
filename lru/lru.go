// Package lru is a "least recently used" cache.
package lru

import (
	"container/list"
	"fmt"
	"sync"
)

// cache represents a LRU consisting of a map as an index and a list to hold data and indicate the last recently used queue.
type cache struct {
	max   int
	index map[interface{}]*list.Element
	*list.List
	sync.RWMutex
}

// listData is the list payload
type listData struct {
	key   interface{}
	value interface{}
}

// New returns an empty cache with capacity max
// it represents a LRU consisting of a map as an index and a list to hold data and indicate the last recently used queue.
func New(max int) *cache {
	return &cache{
		max:   max,
		index: make(map[interface{}]*list.Element, max+1),
		List:  list.New(),
	}
}

// Get returns element or nil, ok is true if the key x is present in the cache and
// sets the element as the last recently used.
func (c *cache) Get(key interface{}) (value interface{}, ok bool) {
	c.RLock()
	defer c.RUnlock()
	listElement, ok := c.index[key]
	if ok {
		c.MoveToFront(listElement)
		return listElement.Value.(*listData).value, true
	}
	return nil, false
}

// Set inserts or updates the value of key and
// sets the element as the last recently used.
func (c *cache) Set(key interface{}, value interface{}) {
	c.Lock()
	defer c.Unlock()

	listElement, ok := c.index[key]
	if ok {
		c.MoveToFront(listElement)
		listElement.Value.(*listData).value = value
		return
	}
	listElement = c.PushFront(&listData{key: key, value: value})
	c.index[key] = listElement

	if c.max != 0 && c.Len() > c.max {
		lastElement := c.Back()
		lastKey := lastElement.Value.(*listData).key
		c.Remove(lastElement)
		delete(c.index, lastKey)
	}
}

func main() {
	fmt.Println("LRU:")

	x := New(3)

	x.Set("test1", 0)
	x.Set("test1", 1)
	x.Set("test2", 2)
	x.Set("test3", 3)
	x.Set("test4", 4)
	x.Set("test5", 5)

	fmt.Println(x.Get("test1"))
	fmt.Println(x.Get("test2"))
	fmt.Println(x.Get("test3"))
	fmt.Println(x.Get("test3"))
	x.Set("test6", 6)

	fmt.Printf("List:")
	for temp := x.List.Front(); temp != nil; temp = temp.Next() {
		fmt.Println(temp.Value)
	}
}
