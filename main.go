package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// 全局配置变量
var config Config

func main() {
	// 解析命令行参数
	parseFlags()

	// 初始化日志
	initLogger()

	// 加载配置文件
	if err := loadConfig(config.ConfigFile); err != nil {
		log.Fatalf("加载配置文件失败: %v", err)
	}

	// 验证配置
	if err := validateConfig(); err != nil {
		log.Fatalf("配置验证失败: %v", err)
	}

	log.Printf("MQTT性能测试工具启动，配置: %+v", config)

	// 创建指标收集器
	metricsCollector := NewMetricsCollector(config.MetricsWindowSize)

	// 创建客户端工厂
	clientFactory := NewMQTTClientFactory(&config, metricsCollector)

	// 创建负载生成器
	loadGenerator := NewLoadGenerator(&config, clientFactory, metricsCollector)

	// 创建结果展示器
	resultDisplay := NewResultDisplay(&config, metricsCollector)

	// 启动指标收集器
	metricsCollector.Start()

	// 启动结果展示器
	resultDisplay.Start()

	// 启动负载测试
	if err := loadGenerator.Start(); err != nil {
		log.Fatalf("启动负载测试失败: %v", err)
	}

	// 等待测试完成或中断
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	select {
	case <-sigChan:
		log.Println("收到中断信号，正在停止测试...")
	case <-time.After(time.Duration(config.TestDuration) * time.Second):
		log.Println("测试时间已结束")
	}

	// 停止负载测试
	loadGenerator.Stop()

	// 停止指标收集器
	metricsCollector.Stop()

	// 显示最终结果
	resultDisplay.ShowFinalResults()

	log.Println("MQTT性能测试工具已退出")
}

// 解析命令行参数
func parseFlags() {
	flag.StringVar(&config.ConfigFile, "config", "config.yaml", "配置文件路径")
	flag.IntVar(&config.ConcurrentClients, "clients", 10, "并发客户端数量")
	flag.IntVar(&config.MessageRate, "rate", 100, "消息发送速率(条/秒)")
	flag.IntVar(&config.TestDuration, "duration", 60, "测试持续时间(秒)")
	flag.StringVar(&config.BrokerURL, "broker", "tcp://localhost:1883", "MQTT broker URL")
	flag.StringVar(&config.Topic, "topic", "test/performance", "测试消息主题")
	flag.IntVar(&config.MessageSize, "size", 256, "消息大小(字节)")
	flag.IntVar(&config.QoS, "qos", 1, "QoS级别(0,1,2)")
	flag.StringVar(&config.MQTTVersion, "version", "3.1.1", "MQTT版本(3.1.1或5.0)")
	flag.IntVar(&config.MetricsWindowSize, "metrics-window", 10, "指标统计窗口大小(秒)")
	flag.StringVar(&config.ResultOutput, "output", "console", "结果输出方式(console,json,csv)")
	flag.Parse()
}

// 初始化日志
func initLogger() {
	log.SetOutput(os.Stdout)
	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds | log.Lshortfile)
}
