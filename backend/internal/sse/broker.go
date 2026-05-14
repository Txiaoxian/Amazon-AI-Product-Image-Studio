package sse

import (
	"context"
	"sync"

	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/database"
)

const defaultBrokerBuffer = 64

type Broker struct {
	buffer int

	mu          sync.RWMutex
	nextID      uint64
	subscribers map[uint64]chan database.TaskEvent
}

func NewBroker(buffer int) *Broker {
	if buffer <= 0 {
		buffer = defaultBrokerBuffer
	}
	return &Broker{
		buffer:      buffer,
		subscribers: map[uint64]chan database.TaskEvent{},
	}
}

func (b *Broker) PublishTaskEvent(_ context.Context, event database.TaskEvent) {
	if b == nil {
		return
	}
	b.mu.RLock()
	defer b.mu.RUnlock()
	for _, subscriber := range b.subscribers {
		select {
		case subscriber <- event:
		default:
		}
	}
}

func (b *Broker) Subscribe(ctx context.Context) (<-chan database.TaskEvent, func()) {
	if b == nil {
		b = NewBroker(defaultBrokerBuffer)
	}
	if ctx == nil {
		ctx = context.Background()
	}

	ch := make(chan database.TaskEvent, b.buffer)
	done := make(chan struct{})
	b.mu.Lock()
	b.nextID++
	id := b.nextID
	b.subscribers[id] = ch
	b.mu.Unlock()

	var once sync.Once
	unsubscribe := func() {
		once.Do(func() {
			b.mu.Lock()
			delete(b.subscribers, id)
			close(ch)
			b.mu.Unlock()
			close(done)
		})
	}

	go func() {
		select {
		case <-ctx.Done():
		case <-done:
			return
		}
		unsubscribe()
	}()

	return ch, unsubscribe
}

func (b *Broker) SubscriberCount() int {
	if b == nil {
		return 0
	}
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.subscribers)
}
