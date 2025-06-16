package main

import (
	"fmt"
	"io/ioutil"
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

	// 根据配置选择消息生成方式
	switch config.Message.Type {
	case "file":
		// 从文件读取消息内容
		if config.Message.FilePath == "" {
			return nil, fmt.Errorf("文件路径未配置")
		}
		content, err := ioutil.ReadFile(config.Message.FilePath)
		if err != nil {
			return nil, fmt.Errorf("读取消息文件失败: %v", err)
		}
		return content, nil

	case "random", "": // 默认使用随机生成
		// 确保长度配置有效
		if config.Message.Length <= 0 {
			config.Message.Length = 1024 // 默认长度
		}
		// 生成固定长度的随机字符串
		rand.Seed(time.Now().UnixNano())
		letters := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
		b := make([]byte, config.Message.Length)
		for i := range b {
			b[i] = letters[rand.Intn(len(letters))]
		}
		return b, nil

	case "fixed":
		// 使用固定字符串
		if config.Message.Context == "" {
			return nil, fmt.Errorf("固定消息内容未配置")
		}
		return []byte(config.Message.Context), nil

	default:
		return nil, fmt.Errorf("不支持的消息类型: %s", config.Message.Type)
	}
}

// 生成自定义模式的消息
func generatePatternMessage(config *Config) ([]byte, error) {

	// random模式
	return generateTestMessage(config)
}
