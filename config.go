package main

import (
	"errors"
	"fmt"
	"os"
	"strconv"

	"github.com/spf13/viper"
)

// Config 测试配置结构体
type Config struct {
	ConfigFile           string `mapstructure:"config_file"`
	ConcurrentClients    int    `mapstructure:"concurrent_clients"`
	MessageRate          int    `mapstructure:"message_rate"`
	TestDuration         int    `mapstructure:"test_duration"`
	BrokerURL            string `mapstructure:"broker_url"`
	Topic                string `mapstructure:"topic"`
	MessageSize          int    `mapstructure:"message_size"`
	QoS                  int    `mapstructure:"qos"`
	MQTTVersion          string `mapstructure:"mqtt_version"`
	MetricsWindowSize    int    `mapstructure:"metrics_window_size"`
	ResultOutput         string `mapstructure:"result_output"`
	ClientIDPrefix       string `mapstructure:"client_id_prefix"`
	Username             string `mapstructure:"username"`
	Password             string `mapstructure:"password"`
	CleanSession         bool   `mapstructure:"clean_session"`
	KeepAlive            int    `mapstructure:"keep_alive"`
	MaxReconnectInterval int    `mapstructure:"max_reconnect_interval"`

	Message MessageConfig `mapstructure:"message"`
}

// 消息配置
type MessageConfig struct {
	Type     string `mapstructure:"type"`      // 消息类型: random, file, fixed
	Length   int    `mapstructure:"length"`    // 当 type 为 random 时的消息长度
	FilePath string `mapstructure:"file_path"` // 当 type 为 file 时的文件路径
	Context  string `mapstructure:"context"`   // 当 type 为 fixed 时的消息内容
}

// 加载配置文件
func loadConfig(filePath string) error {
	if filePath == "" {
		filePath = "config.yaml"
	}

	viper.SetConfigFile(filePath)
	viper.SetConfigType("yaml")

	// 设置默认值
	setDefaults()

	// 先读取环境变量, bug: 配置中的username会被环境变量-电脑名覆盖
	// viper.AutomaticEnv()

	// 再读取配置文件（配置文件优先级更高）
	if _, err := os.Stat(filePath); err == nil {
		if err := viper.ReadInConfig(); err != nil {
			return fmt.Errorf("读取配置文件失败: %v", err)
		}
	}

	// 将配置映射到结构体
	if err := viper.Unmarshal(&config); err != nil {
		return fmt.Errorf("解析配置失败: %v", err)
	}

	// 转换QoS字符串为整数
	if qosStr := viper.GetString("qos"); qosStr != "" {
		qos, err := strconv.Atoi(qosStr)
		if err != nil {
			return fmt.Errorf("无效的QoS值: %s", qosStr)
		}
		config.QoS = qos
	}

	return nil
}

// 设置默认配置
func setDefaults() {
	viper.SetDefault("concurrent_clients", 10)
	viper.SetDefault("message_rate", 100)
	viper.SetDefault("test_duration", 60)
	viper.SetDefault("broker_url", "tcp://localhost:1883")
	viper.SetDefault("topic", "test/performance")
	viper.SetDefault("message_size", 256)
	viper.SetDefault("qos", 1)
	viper.SetDefault("mqtt_version", "3.1.1")
	viper.SetDefault("metrics_window_size", 10)
	viper.SetDefault("result_output", "console")
	viper.SetDefault("client_id_prefix", "mqtt-bench-")
	viper.SetDefault("clean_session", true)
	viper.SetDefault("keep_alive", 60)
	viper.SetDefault("max_reconnect_interval", 30)
}

// 验证配置
func validateConfig() error {
	if config.ConcurrentClients <= 0 {
		return errors.New("并发客户端数量必须大于0")
	}

	if config.MessageRate < 0 {
		return errors.New("消息发送速率不能为负数")
	}

	if config.TestDuration <= 0 {
		return errors.New("测试持续时间必须大于0")
	}

	if config.BrokerURL == "" {
		return errors.New("MQTT broker URL不能为空")
	}

	if config.Topic == "" {
		return errors.New("测试消息主题不能为空")
	}

	if config.MessageSize <= 0 {
		return errors.New("消息大小必须大于0")
	}

	if config.QoS < 0 || config.QoS > 2 {
		return errors.New("QoS级别必须为0、1或2")
	}

	if config.MQTTVersion != "3.1.1" && config.MQTTVersion != "5.0" {
		return errors.New("MQTT版本必须为3.1.1或5.0")
	}

	if config.MetricsWindowSize <= 0 {
		return errors.New("指标统计窗口大小必须大于0")
	}

	if config.ResultOutput != "console" && config.ResultOutput != "json" && config.ResultOutput != "csv" {
		return errors.New("结果输出方式必须为console、json或csv")
	}

	return nil
}
