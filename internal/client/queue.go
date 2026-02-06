package client

import (
	"sync"

	"github.com/chiredeqiudong-dev/go-tunnel-lite/internal/pkg/log"
	"github.com/chiredeqiudong-dev/go-tunnel-lite/internal/pkg/proto"
)

// MessageQueue 消息队列 - 优化版本：slice-based环形缓冲区
type MessageQueue struct {
	messages  []*proto.Message // 消息数组
	head      int              // 队列头部索引
	tail      int              // 队列尾部索引
	size      int              // 当前队列大小
	capacity  int              // 队列容量
	mu        sync.Mutex
	cond      *sync.Cond
	closed    bool
	batchSize int
}

// NewMessageQueue 创建消息队列
func NewMessageQueue(batchSize int) *MessageQueue {
	// 预分配容量，避免频繁扩容
	capacity := batchSize * 4
	if capacity < 16 {
		capacity = 16
	}

	mq := &MessageQueue{
		messages:  make([]*proto.Message, capacity),
		capacity:  capacity,
		batchSize: batchSize,
	}
	mq.cond = sync.NewCond(&mq.mu)
	return mq
}

// Push 推送消息到队列
func (mq *MessageQueue) Push(msg *proto.Message) {
	mq.mu.Lock()
	defer mq.mu.Unlock()

	if mq.closed {
		return
	}

	// 检查是否需要扩容
	if mq.size >= mq.capacity {
		mq.expand()
	}

	// 添加消息到队列尾部
	mq.messages[mq.tail] = msg
	mq.tail = (mq.tail + 1) % mq.capacity
	mq.size++

	mq.cond.Signal()
}

// expand 扩容队列
func (mq *MessageQueue) expand() {
	newCapacity := mq.capacity * 2
	newMessages := make([]*proto.Message, newCapacity)

	// 复制现有元素
	if mq.tail > mq.head {
		// 没有环形 wrap
		copy(newMessages, mq.messages[mq.head:mq.tail])
	} else {
		// 有环形 wrap
		copy(newMessages, mq.messages[mq.head:])
		copy(newMessages[mq.capacity-mq.head:], mq.messages[:mq.tail])
	}

	mq.messages = newMessages
	mq.head = 0
	mq.tail = mq.size
	mq.capacity = newCapacity
}

// PopBatch 批量弹出消息
func (mq *MessageQueue) PopBatch() []*proto.Message {
	mq.mu.Lock()
	defer mq.mu.Unlock()

	// 等待消息到达
	for mq.size == 0 && !mq.closed {
		mq.cond.Wait()
	}

	if mq.closed {
		return nil
	}

	// 批量获取消息
	batchSize := mq.batchSize
	if batchSize > mq.size {
		batchSize = mq.size
	}

	batch := make([]*proto.Message, 0, batchSize)
	for i := 0; i < batchSize; i++ {
		msg := mq.messages[mq.head]
		batch = append(batch, msg)
		mq.messages[mq.head] = nil // 清除引用，帮助GC
		mq.head = (mq.head + 1) % mq.capacity
		mq.size--
	}

	return batch
}

// Close 关闭消息队列
func (mq *MessageQueue) Close() {
	mq.mu.Lock()
	defer mq.mu.Unlock()

	mq.closed = true
	mq.cond.Broadcast()
}

// Size 获取队列大小
func (mq *MessageQueue) Size() int {
	mq.mu.Lock()
	defer mq.mu.Unlock()
	return mq.size
}

// BatchProcessor 批量处理器
type BatchProcessor struct {
	queue   *MessageQueue
	workers int
	stopCh  chan struct{}
	wg      sync.WaitGroup
	handler func([]*proto.Message)
}

// NewBatchProcessor 创建批量处理器
func NewBatchProcessor(workers int, batchSize int, handler func([]*proto.Message)) *BatchProcessor {
	return &BatchProcessor{
		queue:   NewMessageQueue(batchSize),
		workers: workers,
		stopCh:  make(chan struct{}),
		handler: handler,
	}
}

// Start 启动批量处理器
func (bp *BatchProcessor) Start() {
	for i := 0; i < bp.workers; i++ {
		bp.wg.Add(1)
		go bp.worker(i)
	}
}

// Stop 停止批量处理器
func (bp *BatchProcessor) Stop() {
	close(bp.stopCh)
	bp.queue.Close()
	bp.wg.Wait()
}

// Push 推送消息到处理器
func (bp *BatchProcessor) Push(msg *proto.Message) {
	bp.queue.Push(msg)
}

// worker 工作协程
func (bp *BatchProcessor) worker(id int) {
	defer bp.wg.Done()

	log.Debug("批量处理器工作协程启动", "worker", id)

	for {
		select {
		case <-bp.stopCh:
			log.Debug("批量处理器工作协程停止", "worker", id)
			return
		default:
			batch := bp.queue.PopBatch()
			if batch == nil {
				return
			}

			if len(batch) > 0 {
				bp.handler(batch)
			}
		}
	}
}

// Stats 获取统计信息
func (bp *BatchProcessor) Stats() (int, int) {
	return bp.queue.Size(), bp.workers
}
