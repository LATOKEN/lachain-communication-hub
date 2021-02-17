package utils

import (
	"container/list"
	"fmt"
	"sync"
)

type MessageQueue struct {
	queue *list.List
	lock  *sync.Mutex
	cond  *sync.Cond
}

func NewMessageQueue() *MessageQueue {
	ret := &MessageQueue{
		queue: list.New(),
		lock:  &sync.Mutex{},
	}
	ret.cond = sync.NewCond(ret.lock)
	return ret
}

func (c *MessageQueue) Enqueue(value []byte) {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.queue.PushBack(value)
	c.cond.Broadcast()
}

func (c *MessageQueue) Dequeue() error {
	c.lock.Lock()
	defer c.lock.Unlock()
	if c.queue.Len() > 0 {
		ele := c.queue.Front()
		c.queue.Remove(ele)
	}
	return fmt.Errorf("Pop Error: Queue is empty")
}

func (c *MessageQueue) Front() ([]byte, error) {
	c.lock.Lock()
	defer c.lock.Unlock()
	return c.frontUnlocked()
}

func (c *MessageQueue) frontUnlocked() ([]byte, error) {
	if c.queue.Len() > 0 {
		if val, ok := c.queue.Front().Value.([]byte); ok {
			return val, nil
		}
		return nil, fmt.Errorf("Peek Error: Queue Datatype is incorrect")
	}
	return nil, fmt.Errorf("Peek Error: Queue is empty")
}

func (c *MessageQueue) GetLen() int {
	c.lock.Lock()
	defer c.lock.Unlock()
	return c.queue.Len()
}

func (c *MessageQueue) Empty() bool {
	c.lock.Lock()
	defer c.lock.Unlock()
	return c.queue.Len() == 0
}

func (c *MessageQueue) Clear() {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.queue.Init()
}

func (c *MessageQueue) DequeueOrWait() ([]byte, error) {
	c.lock.Lock()
	defer c.lock.Unlock()
	for c.queue.Len() == 0 {
		c.cond.Wait()
	}
	val := c.queue.Front()
	c.queue.Remove(val)
	res := val.Value.([]byte)
	if res == nil {
		return nil,fmt.Errorf("Peek Error: Queue Datatype is incorrect")
	}
	return res,  nil
}
