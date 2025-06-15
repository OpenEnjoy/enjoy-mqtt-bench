package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync/atomic"
	"text/tabwriter"
	"time"
)

// ResultDisplay 结果展示器，用于展示和导出测试结果
type ResultDisplay struct {
	config   *Config
	metrics  *MetricsCollector
	running  int32
	stopChan chan struct{}
}

// NewResultDisplay 创建新的结果展示器
func NewResultDisplay(config *Config, metrics *MetricsCollector) *ResultDisplay {
	return &ResultDisplay{
		config:   config,
		metrics:  metrics,
		stopChan: make(chan struct{}),
	}
}

// Start 启动结果展示器
func (r *ResultDisplay) Start() {
	atomic.StoreInt32(&r.running, 1)

	// 如果输出方式是控制台，启动实时展示
	if r.config.ResultOutput == "console" {
		go r.startRealTimeDisplay()
	}
}

// Stop 停止结果展示器
func (r *ResultDisplay) Stop() {
	if !atomic.CompareAndSwapInt32(&r.running, 1, 0) {
		return
	}

	close(r.stopChan)
}

// 启动实时结果展示
func (r *ResultDisplay) startRealTimeDisplay() {
	// 每秒更新一次实时指标
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			history := r.metrics.GetMetricsHistory()
			if len(history) > 0 {
				latestWindow := history[len(history)-1]
				r.displayRealTimeMetrics(latestWindow)
			}
		case <-r.stopChan:
			return
		}
	}
}

// 显示实时指标
func (r *ResultDisplay) displayRealTimeMetrics(window *MetricsData) {
	duration := window.EndTime.Sub(window.StartTime).Seconds()
	if duration <= 0 {
		duration = 1
	}

	throughput := float64(window.MessagesSent) / duration
	avgLatency := time.Duration(0)
	if window.MessagesSent > 0 {
		avgLatency = time.Duration(window.TotalMessageLatency) / time.Duration(window.MessagesSent)
	}

	connSuccessRate := 0.0
	if window.ConnectionsAttempted > 0 {
		connSuccessRate = float64(window.ConnectionsSucceeded) / float64(window.ConnectionsAttempted) * 100
	}

	msgSuccessRate := 0.0
	msgTotal := window.MessagesSent + window.MessagesFailed
	if msgTotal > 0 {
		msgSuccessRate = float64(window.MessagesSent) / float64(msgTotal) * 100
	}

	// 清空当前行并显示实时指标
	fmt.Printf("\r实时指标: 吞吐量=%.2f条/秒, 平均延迟=%v, 连接成功率=%.2f%%, 消息成功率=%.2f%% ",
		throughput, avgLatency, connSuccessRate, msgSuccessRate)
}

// ShowFinalResults 显示最终测试结果
func (r *ResultDisplay) ShowFinalResults() {
	fmt.Println() // 用于换行，避免覆盖实时指标行
	log.Println("测试已完成，正在生成最终报告...")

	// 获取全局统计指标
	globalMetrics := r.metrics.CalculateGlobalMetrics()

	// 根据配置的输出方式展示结果
	switch r.config.ResultOutput {
	case "console":
		r.displayConsoleOutput(globalMetrics)
	case "json":
		r.exportJSONOutput(globalMetrics)
	case "csv":
		r.exportCSVOutput(globalMetrics)
	default:
		r.displayConsoleOutput(globalMetrics)
	}
}

// 在控制台显示结果
func (r *ResultDisplay) displayConsoleOutput(metrics map[string]interface{}) {
	fmt.Println("\n==================== MQTT性能测试报告 ====================")

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)

	fmt.Fprintf(w, "测试持续时间:\t%v\n", metrics["total_duration"])
	fmt.Fprintf(w, "并发客户端数量:\t%d\n", r.config.ConcurrentClients)
	fmt.Fprintf(w, "目标消息速率:\t%d条/秒\n", r.config.MessageRate)
	fmt.Fprintf(w, "消息大小:\t%d字节\n", r.config.MessageSize)
	fmt.Fprintf(w, "QoS级别:\t%d\n", r.config.QoS)
	fmt.Fprintf(w, "MQTT版本:\t%s\n", r.config.MQTTVersion)
	fmt.Fprintf(w, "Broker地址:\t%s\n", r.config.BrokerURL)
	fmt.Fprintf(w, "测试主题:\t%s\n\n", r.config.Topic)

	fmt.Fprintf(w, "总连接尝试次数:\t%d\n", metrics["total_connections"])
	fmt.Fprintf(w, "成功连接次数:\t%d\n", metrics["successful_connections"])
	fmt.Fprintf(w, "失败连接次数:\t%d\n", metrics["failed_connections"])
	fmt.Fprintf(w, "连接成功率:\t%s\n", metrics["connection_success_rate"])
	fmt.Fprintf(w, "重连次数:\t%d\n", metrics["reconnections"])
	fmt.Fprintf(w, "连接丢失次数:\t%d\n\n", metrics["connection_losses"])

	fmt.Fprintf(w, "总消息数量:\t%d\n", metrics["total_messages"])
	fmt.Fprintf(w, "成功消息数量:\t%d\n", metrics["successful_messages"])
	fmt.Fprintf(w, "失败消息数量:\t%d\n", metrics["failed_messages"])
	fmt.Fprintf(w, "消息成功率:\t%s\n", metrics["message_success_rate"])
	fmt.Fprintf(w, "消息丢失率:\t%s\n", metrics["message_loss_rate"])
	fmt.Fprintf(w, "平均吞吐量:\t%s\n", metrics["throughput"])
	fmt.Fprintf(w, "平均消息延迟:\t%v\n", metrics["avg_latency"])
	fmt.Fprintf(w, "客户端断开次数:\t%d\n", metrics["disconnected_clients"])

	w.Flush()

	fmt.Println("=========================================================")
}

// 导出JSON格式结果
func (r *ResultDisplay) exportJSONOutput(metrics map[string]interface{}) {
	// 添加测试配置到结果中
	result := map[string]interface{}{
		"test_config": r.config,
		"metrics":     metrics,
		"timestamp":   time.Now().Format(time.RFC3339),
	}

	// 创建结果目录
	if err := os.MkdirAll("results", 0755); err != nil {
		log.Printf("创建结果目录失败: %v", err)
		return
	}

	// 生成文件名
	filename := fmt.Sprintf("results/mqtt-bench-result-%s.json",
		time.Now().Format("20060102150405"))

	// 写入文件
	file, err := os.Create(filename)
	if err != nil {
		log.Printf("创建JSON文件失败: %v", err)
		return
	}
	defer file.Close()

	// 编码为JSON
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(result); err != nil {
		log.Printf("JSON编码失败: %v", err)
		return
	}

	absPath, _ := filepath.Abs(filename)
	log.Printf("JSON结果已导出至: %s", absPath)
}

// 导出CSV格式结果
func (r *ResultDisplay) exportCSVOutput(metrics map[string]interface{}) {
	// 创建结果目录
	if err := os.MkdirAll("results", 0755); err != nil {
		log.Printf("创建结果目录失败: %v", err)
		return
	}

	// 生成文件名
	filename := fmt.Sprintf("results/mqtt-bench-result-%s.csv",
		time.Now().Format("20060102150405"))

	// 写入文件
	file, err := os.Create(filename)
	if err != nil {
		log.Printf("创建CSV文件失败: %v", err)
		return
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// 写入CSV头部和数据
	headers := []string{"指标名称", "值"}
	rows := [][]string{headers}

	// 添加测试配置
	rows = append(rows, []string{"测试时间", time.Now().Format(time.RFC3339)})
	rows = append(rows, []string{"测试持续时间", fmt.Sprintf("%v", metrics["total_duration"])})
	rows = append(rows, []string{"并发客户端数量", fmt.Sprintf("%d", r.config.ConcurrentClients)})
	rows = append(rows, []string{"目标消息速率", fmt.Sprintf("%d条/秒", r.config.MessageRate)})
	rows = append(rows, []string{"消息大小", fmt.Sprintf("%d字节", r.config.MessageSize)})
	rows = append(rows, []string{"QoS级别", fmt.Sprintf("%d", r.config.QoS)})
	rows = append(rows, []string{"MQTT版本", r.config.MQTTVersion})
	rows = append(rows, []string{"Broker地址", r.config.BrokerURL})
	rows = append(rows, []string{"测试主题", r.config.Topic})
	rows = append(rows, []string{"", ""}) // 空行分隔

	// 添加指标数据
	rows = append(rows, []string{"总连接尝试次数", fmt.Sprintf("%d", metrics["total_connections"])})
	rows = append(rows, []string{"成功连接次数", fmt.Sprintf("%d", metrics["successful_connections"])})
	rows = append(rows, []string{"失败连接次数", fmt.Sprintf("%d", metrics["failed_connections"])})
	rows = append(rows, []string{"连接成功率", fmt.Sprintf("%s", metrics["connection_success_rate"])})
	rows = append(rows, []string{"重连次数", fmt.Sprintf("%d", metrics["reconnections"])})
	rows = append(rows, []string{"连接丢失次数", fmt.Sprintf("%d", metrics["connection_losses"])})
	rows = append(rows, []string{"", ""}) // 空行分隔

	rows = append(rows, []string{"总消息数量", fmt.Sprintf("%d", metrics["total_messages"])})
	rows = append(rows, []string{"成功消息数量", fmt.Sprintf("%d", metrics["successful_messages"])})
	rows = append(rows, []string{"失败消息数量", fmt.Sprintf("%d", metrics["failed_messages"])})
	rows = append(rows, []string{"消息成功率", fmt.Sprintf("%s", metrics["message_success_rate"])})
	rows = append(rows, []string{"消息丢失率", fmt.Sprintf("%s", metrics["message_loss_rate"])})
	rows = append(rows, []string{"平均吞吐量", fmt.Sprintf("%s", metrics["throughput"])})
	rows = append(rows, []string{"平均消息延迟", fmt.Sprintf("%v", metrics["avg_latency"])})
	rows = append(rows, []string{"客户端断开次数", fmt.Sprintf("%d", metrics["disconnected_clients"])})

	// 写入所有行
	if err := writer.WriteAll(rows); err != nil {
		log.Printf("CSV写入失败: %v", err)
		return
	}

	absPath, _ := filepath.Abs(filename)
	log.Printf("CSV结果已导出至: %s", absPath)
}
