# enjoy-mqtt-bench

#### 介绍
基于GO语言实现的MQTT性能测工具

### 核心功能模块
1. 配置管理 ：通过 `config.go` 和 `config.yaml` 实现，支持命令行参数和配置文件双重配置方式
2. 客户端工厂 ：在 `client.go` 中实现，支持创建多个MQTT客户端，处理连接和重连逻辑
3. 负载生成 ：在 `load.go` 中实现，可按预设速率生成测试消息
4. 指标收集 ：在 `metrics.go` 中实现，采用滑动窗口算法统计吞吐量、延迟等关键指标
5. 结果展示 ：在 `result.go` 中实现，支持控制台、JSON和CSV三种输出格式
### 关键实现特点
- 高并发设计 ：使用Goroutines和Channels实现高并发客户端管理
- 多版本支持 ：兼容MQTT 3.1.1和5.0协议
- 实时指标 ：采用时间窗口算法统计实时性能数据
- 资源控制 ：实现优雅的启动和关闭机制，避免资源泄漏


#### 软件架构
软件架构说明


#### 安装教程

1.  xxxx
2.  xxxx
3.  xxxx

#### 使用说明

1. 安装依赖：
   ```bash
   # 设置Go模块代理（国内用户推荐）
   go env -w GOPROXY=https://goproxy.cn,direct
   
   # 下载依赖
   go mod download
   ```
2. 构建可执行文件： go build -o mqtt-bench.exe
3. 运行测试： ./mqtt-bench.exe --config config.yaml
配置文件中可调整并发客户端数量、消息速率、测试时长等参数，满足不同场景的测试需求。

#### 参与贡献

1.  Fork 本仓库
2.  新建 Feat_xxx 分支
3.  提交代码
4.  新建 Pull Request


#### 特技

1.  使用 Readme\_XXX.md 来支持不同的语言，例如 Readme\_en.md, Readme\_zh.md
2.  Gitee 官方博客 [blog.gitee.com](https://blog.gitee.com)
3.  你可以 [https://gitee.com/explore](https://gitee.com/explore) 这个地址来了解 Gitee 上的优秀开源项目
4.  [GVP](https://gitee.com/gvp) 全称是 Gitee 最有价值开源项目，是综合评定出的优秀开源项目
5.  Gitee 官方提供的使用手册 [https://gitee.com/help](https://gitee.com/help)
6.  Gitee 封面人物是一档用来展示 Gitee 会员风采的栏目 [https://gitee.com/gitee-stars/](https://gitee.com/gitee-stars/)
