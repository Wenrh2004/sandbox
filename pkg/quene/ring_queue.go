package quene

import (
	"errors"
	"sync"
)

var (
	// ErrQueueFull 队列已满错误
	ErrQueueFull = errors.New("queue is full")
	// ErrQueueEmpty 队列为空错误
	ErrQueueEmpty = errors.New("queue is empty")
)

// RingQueue 是一个泛型环形队列
type RingQueue[T any] struct {
	items    []T
	head     int
	tail     int
	size     int
	capacity int
	mu       sync.RWMutex
}

// NewRingQueue 创建一个新的环形队列
func NewRingQueue[T any](capacity int) *RingQueue[T] {
	return &RingQueue[T]{
		items:    make([]T, capacity),
		head:     0,
		tail:     0,
		size:     0,
		capacity: capacity,
	}
}

// Size 返回队列中的元素数量
func (q *RingQueue[T]) Size() int {
	q.mu.RLock()
	defer q.mu.RUnlock()
	return q.size
}

// IsEmpty 判断队列是否为空
func (q *RingQueue[T]) IsEmpty() bool {
	q.mu.RLock()
	defer q.mu.RUnlock()
	return q.size == 0
}

// IsFull 判断队列是否已满
func (q *RingQueue[T]) IsFull() bool {
	q.mu.RLock()
	defer q.mu.RUnlock()
	return q.size == q.capacity
}

// Enqueue 将元素添加到队列尾部
func (q *RingQueue[T]) Enqueue(item T) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	
	if q.size == q.capacity {
		return ErrQueueFull
	}
	
	q.items[q.tail] = item
	q.tail = (q.tail + 1) % q.capacity
	q.size++
	
	return nil
}

// Dequeue 从队列头部取出元素
func (q *RingQueue[T]) Dequeue() (T, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	
	var zero T
	
	if q.size == 0 {
		return zero, ErrQueueEmpty
	}
	
	item := q.items[q.head]
	q.head = (q.head + 1) % q.capacity
	q.size--
	
	return item, nil
}

// Peek 查看队列头部元素但不移除
func (q *RingQueue[T]) Peek() (T, error) {
	q.mu.RLock()
	defer q.mu.RUnlock()
	
	var zero T
	
	if q.size == 0 {
		return zero, ErrQueueEmpty
	}
	
	return q.items[q.head], nil
}

// Clear 清空队列
func (q *RingQueue[T]) Clear() {
	q.mu.Lock()
	defer q.mu.Unlock()
	
	q.head = 0
	q.tail = 0
	q.size = 0
}

// EnqueueBatch 批量入队元素，如果空间不足则返回错误
func (q *RingQueue[T]) EnqueueBatch(items []T) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	
	if q.size+len(items) > q.capacity {
		return ErrQueueFull
	}
	
	for _, item := range items {
		q.items[q.tail] = item
		q.tail = (q.tail + 1) % q.capacity
		q.size++
	}
	
	return nil
}

// ForEach 遍历队列中的每个元素
func (q *RingQueue[T]) ForEach(fn func(T)) {
	q.mu.RLock()
	defer q.mu.RUnlock()
	
	if q.size == 0 {
		return
	}
	
	idx := q.head
	for i := 0; i < q.size; i++ {
		fn(q.items[idx])
		idx = (idx + 1) % q.capacity
	}
}
