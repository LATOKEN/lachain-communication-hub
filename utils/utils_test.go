package utils

import (
	"bytes"
	"testing"
	"time"
)

func TestQueue(t *testing.T) {
	queue := NewMessageQueue()
	done := make(chan bool)
	fail := make(chan bool)
	goldenMessage := []byte("fgdghfghfhgghfhj")

	for j := 0; j < 100; j++ {
		go func() {
			for i := 0; i < 100; i++ {
				queue.Enqueue(goldenMessage)
			}
		}()
	}

	go func() {
		for i :=0; i < 10000; i++ {
			val, err := queue.DequeueOrWait()
			if err != nil {
				fail <- true
			}
			if !bytes.Equal(val, goldenMessage) {
				fail <- true
			}
		}
		if queue.GetLen() == 0 {
			done <- true
		} else {
			fail <- true
		}
	} ()

	ticker := time.NewTicker(time.Minute)
	select {
	case <-done:
		ticker.Stop()
	case <-fail:
		ticker.Stop()
		t.Error("Failed to process nessages")
	case <-ticker.C:
		t.Error("Failed to receive message in time")
	}
}

