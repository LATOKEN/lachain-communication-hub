package utils

import (
	"container/list"
	"fmt"
	"lachain-communication-hub/communication"
	"sync"
)

type Envelop = communication.MessageEnvelop
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

func (c *MessageQueue) Enqueue(value Envelop) {
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

func (c *MessageQueue) Front() (Envelop, error) {
	c.lock.Lock()
	defer c.lock.Unlock()
	return c.frontUnlocked()
}

func (c *MessageQueue) frontUnlocked() (Envelop, error) {
	if c.queue.Len() > 0 {
		if val, ok := c.queue.Front().Value.(Envelop); ok {
			return val, nil
		}
		return communication.NewEnvelop(nil, false), fmt.Errorf("Peek Error: Queue Datatype is incorrect")
	}
	return communication.NewEnvelop(nil, false), fmt.Errorf("Peek Error: Queue is empty")
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

func (c *MessageQueue) DequeueOrWait() (Envelop, error) {
	c.lock.Lock()
	defer c.lock.Unlock()
	for c.queue.Len() == 0 {
		c.cond.Wait()
	}
	val := c.queue.Front()
	c.queue.Remove(val)
	res := val.Value.(Envelop)
	if res.Data() == nil {
		return res, fmt.Errorf("Peek Error: Queue Datatype is incorrect")
	}
	return res,  nil
}
