package utils

import (
	"container/list"
	"fmt"
)

type MessageQueue struct {
	queue *list.List
}

func NewMessageQueue() *MessageQueue {
	ret := &MessageQueue{
		queue: list.New(),
	}
	return ret
}


func (c *MessageQueue) Enqueue(value []byte) {
	c.queue.PushBack(value)
}

func (c *MessageQueue) Dequeue() error {
	if c.queue.Len() > 0 {
		ele := c.queue.Front()
		c.queue.Remove(ele)
	}
	return fmt.Errorf("Pop Error: Queue is empty")
}

func (c *MessageQueue) Front() ([]byte, error) {
	if c.queue.Len() > 0 {
		if val, ok := c.queue.Front().Value.([]byte); ok {
			return val, nil
		}
		return nil, fmt.Errorf("Peep Error: Queue Datatype is incorrect")
	}
	return nil, fmt.Errorf("Peep Error: Queue is empty")
}

func (c *MessageQueue) GetLen() int {
	return c.queue.Len()
}

func (c *MessageQueue) Empty() bool {
	return c.queue.Len() == 0
}

func (c* MessageQueue) Clear() {
	c.queue.Init()
}
