package main

import (
	"fmt"
	"log"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/time/rate"
)

// LoadGenerator 负载生成器，用于控制消息发送速率和模式
type LoadGenerator struct {
	config        *Config
	clientFactory *MQTTClientFactory
	metrics       *MetricsCollector
	clients       []*MQTTClient
	message       []byte
	running       int32
	stopChan      chan struct{}
	wg            sync.WaitGroup
}

// NewLoadGenerator 创建新的负载生成器
func NewLoadGenerator(config *Config, clientFactory *MQTTClientFactory, metrics *MetricsCollector) *LoadGenerator {
	// 生成测试消息内容
	message, err := generateTestMessage(config)
	if err != nil {
		return nil
	}
	if err != nil {
		log.Fatalf("生成测试消息失败: %v", err)
	}

	return &LoadGenerator{
		config:        config,
		clientFactory: clientFactory,
		metrics:       metrics,
		message:       message,
		stopChan:      make(chan struct{}),
	}
}

// Start 启动负载测试
func (l *LoadGenerator) Start() error {
	// 创建指定数量的客户端
	clients, err := l.clientFactory.CreateClients(l.config.ConcurrentClients)
	if err != nil {
		return fmt.Errorf("创建客户端失败: %v", err)
	}
	l.clients = clients

	// 标记测试为运行中
	atomic.StoreInt32(&l.running, 1)

	// 计算每个客户端的消息速率
	messagesPerClient := l.config.MessageRate / len(clients)
	extraMessages := l.config.MessageRate % len(clients)

	// 为每个客户端启动消息发送协程
	for i, client := range clients {
		messageRate := messagesPerClient
		if i < extraMessages {
			messageRate++ // 分配剩余的消息
		}

		if messageRate > 0 {
			l.wg.Add(1)
			go l.startClientLoad(client, messageRate)
		}
	}

	log.Printf("负载测试已启动，并发客户端: %d, 总消息速率: %d条/秒", len(clients), l.config.MessageRate)
	return nil
}

// Stop 停止负载测试
func (l *LoadGenerator) Stop() {
	if !atomic.CompareAndSwapInt32(&l.running, 1, 0) {
		return // 已经停止
	}

	close(l.stopChan)
	l.wg.Wait()
	l.clientFactory.CloseAll()

	log.Println("负载测试已停止")
}

// 为单个客户端启动负载发送
func (l *LoadGenerator) startClientLoad(client *MQTTClient, messageRate int) {
	defer l.wg.Done()

	if messageRate <= 0 {
		return
	}

	// 创建速率限制器
	burstSize := int(messageRate)
	limiter := rate.NewLimiter(rate.Limit(messageRate), burstSize)
	ticker := time.NewTicker(time.Second / time.Duration(messageRate))
	defer ticker.Stop()

	for {
		select {
		case <-l.stopChan:
			return
		case <-ticker.C:
			// 检查速率限制
			if !limiter.Allow() {
				continue
			}

			// 检查客户端连接状态
			if !client.IsConnected() {
				l.metrics.RecordClientDisconnected()
				log.Printf("客户端 %s 已断开连接，无法发送消息", client.clientID)
				continue
			}

			// 发送消息
			go func() {
				if err := client.Publish(l.message); err != nil {
					log.Printf("客户端 %s 发送消息失败: %v", client.clientID, err)
				}
			}()
		}
	}
}

// 生成测试消息
func generateTestMessage(config *Config) ([]byte, error) {
	size := config.MessageSize
	if size <= 0 {
		return []byte{}, nil
	}

	// 创建随机字节数组
	b := make([]byte, size)
	_, err := rand.Read(b)
	if err != nil {
		// 如果随机生成失败，使用固定模式填充
		for i := 0; i < size; i++ {
			b[i] = byte(i % 256)
		}
	}

	return []byte(b), nil
}

// 生成自定义模式的消息
func generatePatternMessage(config *Config) ([]byte, error) {

	// random模式
	return generateTestMessage(config)
}
