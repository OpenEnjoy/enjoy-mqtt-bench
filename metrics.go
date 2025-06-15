package main

import (
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"
)

// MetricsData 单个时间窗口的指标数据
type MetricsData struct {
	StartTime time.Time
	EndTime   time.Time

	// 连接指标
	ConnectionsAttempted int64
	ConnectionsSucceeded int64
	ConnectionsFailed    int64
	Reconnections        int64
	ConnectionLost       int64

	// 消息指标
	MessagesSent        int64
	MessagesFailed      int64
	TotalMessageLatency int64

	// 客户端指标
	ClientsDisconnected int64
}

// MetricsCollector 指标收集器，用于收集和统计性能指标
type MetricsCollector struct {
	windowSize     time.Duration
	currentWindow  *MetricsData
	windowChan     chan *MetricsData
	metricsHistory []*MetricsData

	// 全局统计
	TotalConnectionsAttempted int64
	TotalConnectionsSucceeded int64
	TotalConnectionsFailed    int64
	TotalReconnections        int64
	TotalConnectionLost       int64
	TotalMessagesSent         int64
	TotalMessagesFailed       int64
	TotalMessageLatency       int64
	TotalClientsDisconnected  int64

	running  int32
	mutex    sync.RWMutex
	stopChan chan struct{}
}

// NewMetricsCollector 创建新的指标收集器
func NewMetricsCollector(windowSize int) *MetricsCollector {
	return &MetricsCollector{
		windowSize: time.Duration(windowSize) * time.Second,
		windowChan: make(chan *MetricsData, 100),
		currentWindow: &MetricsData{
			StartTime: time.Now(),
		},
		stopChan: make(chan struct{}),
	}
}

// Start 启动指标收集器
func (m *MetricsCollector) Start() {
	atomic.StoreInt32(&m.running, 1)

	// 启动窗口轮换协程
	go func() {
		for {
			select {
			case <-time.After(m.windowSize):
				m.rotateWindow()
			case <-m.stopChan:
				m.rotateWindow()
				return
			}
		}
	}()

	// 启动窗口处理协程
	go m.processWindows()
}

// Stop 停止指标收集器
func (m *MetricsCollector) Stop() {
	if !atomic.CompareAndSwapInt32(&m.running, 1, 0) {
		return
	}

	close(m.stopChan)
}

// 轮换指标窗口
func (m *MetricsCollector) rotateWindow() {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// 设置当前窗口结束时间
	m.currentWindow.EndTime = time.Now()

	// 将当前窗口发送到处理通道
	select {
	case m.windowChan <- m.currentWindow:
	default:
		log.Println("指标窗口缓冲区已满，丢弃一个窗口数据")
	}

	// 创建新的指标窗口
	m.currentWindow = &MetricsData{
		StartTime: time.Now(),
	}
}

// 处理指标窗口数据
func (m *MetricsCollector) processWindows() {
	for window := range m.windowChan {
		m.mutex.Lock()
		m.metricsHistory = append(m.metricsHistory, window)

		// 更新全局统计
		m.TotalConnectionsAttempted += window.ConnectionsAttempted
		m.TotalConnectionsSucceeded += window.ConnectionsSucceeded
		m.TotalConnectionsFailed += window.ConnectionsFailed
		m.TotalReconnections += window.Reconnections
		m.TotalConnectionLost += window.ConnectionLost
		m.TotalMessagesSent += window.MessagesSent
		m.TotalMessagesFailed += window.MessagesFailed
		m.TotalMessageLatency += window.TotalMessageLatency
		m.TotalClientsDisconnected += window.ClientsDisconnected

		m.mutex.Unlock()

		// 计算并记录窗口指标
		m.calculateWindowMetrics(window)
	}
}

// 计算窗口指标
func (m *MetricsCollector) calculateWindowMetrics(window *MetricsData) {
	duration := window.EndTime.Sub(window.StartTime).Seconds()
	if duration <= 0 {
		duration = 1 // 避免除以零
	}

	// 计算吞吐量
	throughput := float64(window.MessagesSent) / duration

	// 计算平均延迟
	avgLatency := time.Duration(0)
	if window.MessagesSent > 0 {
		avgLatency = time.Duration(window.TotalMessageLatency) / time.Duration(window.MessagesSent)
	}

	// 计算连接成功率
	connSuccessRate := 0.0
	if window.ConnectionsAttempted > 0 {
		connSuccessRate = float64(window.ConnectionsSucceeded) / float64(window.ConnectionsAttempted) * 100
	}

	// 计算消息成功率
	msgSuccessRate := 0.0
	if window.MessagesSent+window.MessagesFailed > 0 {
		msgSuccessRate = float64(window.MessagesSent) / float64(window.MessagesSent+window.MessagesFailed) * 100
	}

	log.Printf(
		"[窗口指标] 时间: %s - %s | 持续时间: %.2fs | 吞吐量: %.2f条/秒 | 平均延迟: %v | 连接成功率: %.2f%% | 消息成功率: %.2f%%",
		window.StartTime.Format("15:04:05.000"),
		window.EndTime.Format("15:04:05.000"),
		duration,
		throughput,
		avgLatency,
		connSuccessRate,
		msgSuccessRate,
	)
}

// RecordConnectionAttempted 记录连接尝试
func (m *MetricsCollector) RecordConnectionAttempted() {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	if m.currentWindow != nil {
		atomic.AddInt64(&m.currentWindow.ConnectionsAttempted, 1)
	}
}

// RecordConnectionSuccess 记录连接成功
func (m *MetricsCollector) RecordConnectionSuccess(duration time.Duration) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	if m.currentWindow != nil {
		atomic.AddInt64(&m.currentWindow.ConnectionsSucceeded, 1)
		atomic.AddInt64(&m.currentWindow.ConnectionsAttempted, 1)
	}
}

// RecordConnectionFailure 记录连接失败
func (m *MetricsCollector) RecordConnectionFailure() {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	if m.currentWindow != nil {
		atomic.AddInt64(&m.currentWindow.ConnectionsFailed, 1)
		atomic.AddInt64(&m.currentWindow.ConnectionsAttempted, 1)
	}
}

// RecordReconnection 记录重连成功
func (m *MetricsCollector) RecordReconnection() {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	if m.currentWindow != nil {
		atomic.AddInt64(&m.currentWindow.Reconnections, 1)
	}
}

// RecordConnectionLost 记录连接丢失
func (m *MetricsCollector) RecordConnectionLost() {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	if m.currentWindow != nil {
		atomic.AddInt64(&m.currentWindow.ConnectionLost, 1)
	}
}

// RecordMessageSuccess 记录消息发送成功
func (m *MetricsCollector) RecordMessageSuccess(latency time.Duration) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	if m.currentWindow != nil {
		atomic.AddInt64(&m.currentWindow.MessagesSent, 1)
		atomic.AddInt64(&m.currentWindow.TotalMessageLatency, int64(latency))
	}
}

// RecordMessageFailure 记录消息发送失败
func (m *MetricsCollector) RecordMessageFailure() {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	if m.currentWindow != nil {
		atomic.AddInt64(&m.currentWindow.MessagesFailed, 1)
	}
}

// RecordClientDisconnected 记录客户端断开连接
func (m *MetricsCollector) RecordClientDisconnected() {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	if m.currentWindow != nil {
		atomic.AddInt64(&m.currentWindow.ClientsDisconnected, 1)
	}
}

// GetGlobalMetrics 获取全局指标
func (m *MetricsCollector) GetGlobalMetrics() (metrics *MetricsData) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	return &MetricsData{
		StartTime:            m.metricsHistory[0].StartTime,
		EndTime:              time.Now(),
		ConnectionsAttempted: m.TotalConnectionsAttempted,
		ConnectionsSucceeded: m.TotalConnectionsSucceeded,
		ConnectionsFailed:    m.TotalConnectionsFailed,
		Reconnections:        m.TotalReconnections,
		ConnectionLost:       m.TotalConnectionLost,
		MessagesSent:         m.TotalMessagesSent,
		MessagesFailed:       m.TotalMessagesFailed,
		TotalMessageLatency:  m.TotalMessageLatency,
		ClientsDisconnected:  m.TotalClientsDisconnected,
	}
}

// GetMetricsHistory 获取指标历史
func (m *MetricsCollector) GetMetricsHistory() []*MetricsData {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	// 返回历史数据的副本
	history := make([]*MetricsData, len(m.metricsHistory))
	copy(history, m.metricsHistory)
	return history
}

// CalculateGlobalMetrics 计算全局统计指标
func (m *MetricsCollector) CalculateGlobalMetrics() map[string]interface{} {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	var totalDuration time.Duration
	if len(m.metricsHistory) > 0 {
		startTime := m.metricsHistory[0].StartTime
		endTime := time.Now()
		totalDuration = endTime.Sub(startTime)
	} else {
		totalDuration = time.Second // 避免除以零
	}

	throughput := float64(m.TotalMessagesSent) / totalDuration.Seconds()

	avgLatency := time.Duration(0)
	if m.TotalMessagesSent > 0 {
		avgLatency = time.Duration(m.TotalMessageLatency) / time.Duration(m.TotalMessagesSent)
	}

	connSuccessRate := 0.0
	if m.TotalConnectionsAttempted > 0 {
		connSuccessRate = float64(m.TotalConnectionsSucceeded) / float64(m.TotalConnectionsAttempted) * 100
	}

	msgSuccessRate := 0.0
	msgTotal := m.TotalMessagesSent + m.TotalMessagesFailed
	if msgTotal > 0 {
		msgSuccessRate = float64(m.TotalMessagesSent) / float64(msgTotal) * 100
	}

	msgLossRate := 100.0 - msgSuccessRate

	return map[string]interface{}{
		"total_duration":          totalDuration,
		"total_connections":       m.TotalConnectionsAttempted,
		"successful_connections":  m.TotalConnectionsSucceeded,
		"failed_connections":      m.TotalConnectionsFailed,
		"connection_success_rate": fmt.Sprintf("%.2f%%", connSuccessRate),
		"reconnections":           m.TotalReconnections,
		"connection_losses":       m.TotalConnectionLost,
		"total_messages":          msgTotal,
		"successful_messages":     m.TotalMessagesSent,
		"failed_messages":         m.TotalMessagesFailed,
		"message_success_rate":    fmt.Sprintf("%.2f%%", msgSuccessRate),
		"message_loss_rate":       fmt.Sprintf("%.2f%%", msgLossRate),
		"throughput":              fmt.Sprintf("%.2f条/秒", throughput),
		"avg_latency":             avgLatency,
		"disconnected_clients":    m.TotalClientsDisconnected,
	}
}
