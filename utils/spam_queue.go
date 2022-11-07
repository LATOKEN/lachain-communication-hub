package utils

import (
	"container/list"
	"fmt"
	"sync"
)

type SpamQueue struct {
	queue *list.List
	lock  *sync.Mutex
}


func NewSpamQueue() *SpamQueue {
	ret := &SpamQueue{
		queue: list.New(),
		lock:  &sync.Mutex{},
	}
	return ret
}

func (c *SpamQueue) Enqueue(value uint64) {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.queue.PushBack(value)
}

func (c *SpamQueue) Dequeue() (uint64, error) {
	c.lock.Lock()
	defer c.lock.Unlock()
	if c.queue.Len() > 0 {
		val := c.queue.Front()
		c.queue.Remove(val)
		res := val.Value.(uint64)
		return res, nil
	}
	return 0, fmt.Errorf("Pop Error: Queue is empty")
}

func (c *SpamQueue) Front() (uint64, error) {
	c.lock.Lock()
	defer c.lock.Unlock()
	return c.frontUnlocked()
}

func (c *SpamQueue) frontUnlocked() (uint64, error) {
	if c.queue.Len() > 0 {
		if val, ok := c.queue.Front().Value.(uint64); ok {
			return val, nil
		}
		return 0, fmt.Errorf("Peek Error: Queue Datatype is incorrect")
	}
	return 0, fmt.Errorf("Peek Error: Queue is empty")
}

func (c *SpamQueue) GetLen() int {
	c.lock.Lock()
	defer c.lock.Unlock()
	return c.queue.Len()
}

func (c *SpamQueue) Empty() bool {
	c.lock.Lock()
	defer c.lock.Unlock()
	return c.queue.Len() == 0
}

func (c *SpamQueue) Clear() {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.queue.Init()
}
