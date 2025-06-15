package main

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

// MQTTClient 封装MQTT客户端及其相关信息
type MQTTClient struct {
	client    mqtt.Client
	clientID  string
	config    *Config
	metrics   *MetricsCollector
	connected bool
	mutex     sync.RWMutex
}

// MQTTClientFactory 客户端工厂，用于创建和管理多个MQTT客户端
type MQTTClientFactory struct {
	config  *Config
	metrics *MetricsCollector
	clients map[string]*MQTTClient
	mutex   sync.RWMutex
}

// NewMQTTClientFactory 创建新的客户端工厂
func NewMQTTClientFactory(config *Config, metrics *MetricsCollector) *MQTTClientFactory {
	return &MQTTClientFactory{
		config:  config,
		metrics: metrics,
		clients: make(map[string]*MQTTClient),
	}
}

// CreateClient 创建新的MQTT客户端
func (f *MQTTClientFactory) CreateClient() (*MQTTClient, error) {
	clientID := f.generateClientID()

	// 创建MQTT客户端选项
	opts := mqtt.NewClientOptions()
	opts.AddBroker(f.config.BrokerURL)
	opts.SetClientID(clientID)
	opts.SetCleanSession(f.config.CleanSession)
	opts.SetUsername(f.config.Username)
	opts.SetPassword(f.config.Password)
	opts.SetKeepAlive(time.Duration(f.config.KeepAlive) * time.Second)
	opts.SetMaxReconnectInterval(time.Duration(f.config.MaxReconnectInterval) * time.Second)

	// 设置MQTT版本
	if f.config.MQTTVersion == "5.0" {
		opts.SetProtocolVersion(5)
	} else {
		opts.SetProtocolVersion(4) // MQTT 3.1.1
	}

	// 设置连接回调
	opts.SetOnConnectHandler(func(client mqtt.Client) {
		mqttClient := f.getClientByID(clientID)
		if mqttClient != nil {
			mqttClient.onConnect()
		}
	})

	// 设置连接丢失回调
	opts.SetConnectionLostHandler(func(client mqtt.Client, err error) {
		mqttClient := f.getClientByID(clientID)
		if mqttClient != nil {
			mqttClient.onConnectionLost(err)
		}
	})

	// 创建客户端
	client := mqtt.NewClient(opts)

	mqttClient := &MQTTClient{
		client:   client,
		clientID: clientID,
		config:   f.config,
		metrics:  f.metrics,
	}

	// 将客户端添加到工厂管理
	f.mutex.Lock()
	f.clients[clientID] = mqttClient
	f.mutex.Unlock()

	return mqttClient, nil
}

// Connect 连接到MQTT broker
func (c *MQTTClient) Connect() error {
	startTime := time.Now()

	if token := c.client.Connect(); token.Wait() && token.Error() != nil {
		c.metrics.RecordConnectionFailure()
		return fmt.Errorf("连接失败: %v", token.Error())
	}

	// 记录连接成功和延迟
	connectDuration := time.Since(startTime)
	c.metrics.RecordConnectionSuccess(connectDuration)

	c.mutex.Lock()
	c.connected = true
	c.mutex.Unlock()

	log.Printf("客户端 %s 连接成功", c.clientID)
	return nil
}

// Disconnect 断开与MQTT broker的连接
func (c *MQTTClient) Disconnect() {
	if c.client.IsConnected() {
		c.client.Disconnect(250)
		c.mutex.Lock()
		c.connected = false
		c.mutex.Unlock()
		log.Printf("客户端 %s 已断开连接", c.clientID)
	}
}

// Publish 发布消息
func (c *MQTTClient) Publish(message []byte) error {
	if !c.IsConnected() {
		return errors.New("客户端未连接")
	}

	startTime := time.Now()
	token := c.client.Publish(c.config.Topic, byte(c.config.QoS), false, message)
	token.Wait()

	if token.Error() != nil {
		c.metrics.RecordMessageFailure()
		return token.Error()
	}

	// 记录消息发送成功和延迟
	publishDuration := time.Since(startTime)
	c.metrics.RecordMessageSuccess(publishDuration)

	return nil
}

// IsConnected 检查客户端是否已连接
func (c *MQTTClient) IsConnected() bool {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	return c.connected && c.client.IsConnected()
}

// 生成唯一的客户端ID
func (f *MQTTClientFactory) generateClientID() string {
	prefix := f.config.ClientIDPrefix
	if prefix == "" {
		prefix = "mqtt-bench-"
	}

	// 生成随机ID
	b := make([]byte, 8)
	_, err := rand.Read(b)
	if err != nil {
		panic(err)
	}

	return fmt.Sprintf("%s%s", prefix, hex.EncodeToString(b))
}

// 根据客户端ID获取客户端
func (f *MQTTClientFactory) getClientByID(clientID string) *MQTTClient {
	f.mutex.RLock()
	defer f.mutex.RUnlock()
	return f.clients[clientID]
}

// 连接成功回调
func (c *MQTTClient) onConnect() {
	c.mutex.Lock()
	c.connected = true
	c.mutex.Unlock()

	c.metrics.RecordReconnection()
	log.Printf("客户端 %s 重新连接成功", c.clientID)
}

// 连接丢失回调
func (c *MQTTClient) onConnectionLost(err error) {
	c.mutex.Lock()
	c.connected = false
	c.mutex.Unlock()

	c.metrics.RecordConnectionLost()
	log.Printf("客户端 %s 连接丢失: %v", c.clientID, err)
}

// CreateClients 创建多个MQTT客户端
func (f *MQTTClientFactory) CreateClients(count int) ([]*MQTTClient, error) {
	var clients []*MQTTClient
	var wg sync.WaitGroup
	errChan := make(chan error, count)

	// 限制并发创建客户端的数量
	concurrency := 10
	if count < concurrency {
		concurrency = count
	}
	limiter := make(chan struct{}, concurrency)

	for i := 0; i < count; i++ {
		wg.Add(1)
		limiter <- struct{}{}

		go func() {
			defer wg.Done()
			defer func() { <-limiter }()

			client, err := f.CreateClient()
			if err != nil {
				errChan <- fmt.Errorf("创建客户端失败: %v", err)
				return
			}

			if err := client.Connect(); err != nil {
				errChan <- fmt.Errorf("客户端连接失败: %v", err)
				return
			}

			clients = append(clients, client)
		}()
	}

	wg.Wait()
	close(errChan)

	// 检查是否有错误
	if len(errChan) > 0 {
		var errMsg string
		for err := range errChan {
			errMsg += err.Error() + "; "
		}
		return nil, fmt.Errorf("创建客户端时发生错误: %s", errMsg)
	}

	return clients, nil
}

// CloseAll 关闭所有客户端连接
func (f *MQTTClientFactory) CloseAll() {
	f.mutex.RLock()
	defer f.mutex.RUnlock()

	var wg sync.WaitGroup
	for _, client := range f.clients {
		wg.Add(1)
		go func(c *MQTTClient) {
			defer wg.Done()
			c.Disconnect()
		}(client)
	}

	wg.Wait()
}
