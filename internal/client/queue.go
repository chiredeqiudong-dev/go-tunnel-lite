package client

import (
	"container/list"
	"sync"

	"github.com/chiredeqiudong-dev/go-tunnel-lite/internal/pkg/log"
	"github.com/chiredeqiudong-dev/go-tunnel-lite/internal/pkg/proto"
)

// MessageQueue 消息队列
type MessageQueue struct {
	queue     *list.List
	mu        sync.Mutex
	cond      *sync.Cond
	closed    bool
	batchSize int
}

// NewMessageQueue 创建消息队列
func NewMessageQueue(batchSize int) *MessageQueue {
	mq := &MessageQueue{
		queue:     list.New(),
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

	mq.queue.PushBack(msg)
	mq.cond.Signal()
}

// PopBatch 批量弹出消息
func (mq *MessageQueue) PopBatch() []*proto.Message {
	mq.mu.Lock()
	defer mq.mu.Unlock()

	// 等待消息到达
	for mq.queue.Len() == 0 && !mq.closed {
		mq.cond.Wait()
	}

	if mq.closed {
		return nil
	}

	// 批量获取消息
	batch := make([]*proto.Message, 0, mq.batchSize)
	for i := 0; i < mq.batchSize && mq.queue.Len() > 0; i++ {
		elem := mq.queue.Front()
		if elem == nil {
			break
		}
		msg := elem.Value.(*proto.Message)
		batch = append(batch, msg)
		mq.queue.Remove(elem)
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
	return mq.queue.Len()
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
