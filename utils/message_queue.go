package utils

import (
	"container/list"
	"fmt"
	"sync"
)

type MessageEnvelop struct {
	consensus bool
	data 	  []byte
}

type MessageQueue struct {
	queue *list.List
	lock  *sync.Mutex
	cond  *sync.Cond
}

func NewEnvelop(data []byte, consensus bool) MessageEnvelop {
	return MessageEnvelop {
		consensus: consensus,
		data: data,
	}
}

func (envelop *MessageEnvelop) IsConsensus() bool {
	return envelop.consensus
}

func (envelop *MessageEnvelop) Data() []byte {
	return envelop.data
}

func NewMessageQueue() *MessageQueue {
	ret := &MessageQueue{
		queue: list.New(),
		lock:  &sync.Mutex{},
	}
	ret.cond = sync.NewCond(ret.lock)
	return ret
}

func (c *MessageQueue) Enqueue(value MessageEnvelop) {
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

func (c *MessageQueue) Front() (MessageEnvelop, error) {
	c.lock.Lock()
	defer c.lock.Unlock()
	return c.frontUnlocked()
}

func (c *MessageQueue) frontUnlocked() (MessageEnvelop, error) {
	if c.queue.Len() > 0 {
		if val, ok := c.queue.Front().Value.(MessageEnvelop); ok {
			return val, nil
		}
		return NewEnvelop(nil, false), fmt.Errorf("Peek Error: Queue Datatype is incorrect")
	}
	return NewEnvelop(nil, false), fmt.Errorf("Peek Error: Queue is empty")
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

func (c *MessageQueue) DequeueOrWait() (MessageEnvelop, error) {
	c.lock.Lock()
	defer c.lock.Unlock()
	for c.queue.Len() == 0 {
		c.cond.Wait()
	}
	val := c.queue.Front()
	c.queue.Remove(val)
	res := val.Value.(MessageEnvelop)
	if res.Data() == nil {
		return res, fmt.Errorf("Peek Error: Queue Datatype is incorrect")
	}
	return res,  nil
}
