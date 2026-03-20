# Linux Server Check

## 项目简介

Linux Server Check 是一个用 Go 语言开发的 Linux 服务器异常分析工具，用于检查和分析服务器的系统负载、CPU、内存、磁盘、网络和线程等参数，帮助运维人员快速发现和排查服务器问题。

## 项目结构

```
linux_server_check/
├── checker/          # 业务逻辑（各种检查器）
│   ├── loadchecker/  # 负载检查器
│   ├── memchecker/   # 内存检查器
│   ├── diskchecker/  # 磁盘检查器
│   └── threadchecker/# 线程检查器
├── cmd/              # CLI 命令（Cobra）
│   ├── root.go       # 根命令
│   ├── check.go      # 检查命令
│   ├── load.go       # 负载检查子命令
│   ├── mem.go        # 内存检查子命令
│   ├── disk.go       # 磁盘检查子命令
│   ├── threads.go    # 线程检查子命令
│   └── report.go     # 报告生成命令
├── reporter/         # 输出格式化（各种报告生成器）
│   ├── hostreporter/ # 主机报告生成器
│   ├── kernelreporter/ # 内核报告生成器
│   ├── diskioreporter/ # 磁盘IO报告生成器
│   └── netreporter/  # 网络报告生成器
├── utils/            # 共享工具
├── reports/          # 生成的报告存储目录
├── main.go           # 入口点
├── go.mod            # Go 模块文件
└── go.sum            # 依赖校验文件
```

## 安装和构建

### 前提条件

- Go 1.21.11 或更高版本

### 构建步骤

1. 克隆项目：
   ```bash
   git clone https://github.com/cuiwin/linux_server_check.git
   cd linux_server_check
   ```

2. 安装依赖：
   ```bash
   go mod download
   go mod tidy
   ```

3. 构建二进制文件：
   ```bash
   go build -o server_check
   ```
   GOOS=linux GOARCH=amd64 go build -o server_check
   ```

## 使用方法

### 基本用法

```bash
# 运行所有检查
./server_check check

# 运行特定检查
./server_check check load    # 服务器负载和CPU检查
./server_check check mem     # 服务器内存检查
./server_check check disk    # 服务器磁盘检查
./server_check check threads # 服务器线程数检查

# 生成报告
./server_check report
```

### 命令行选项

- `-v, --detail`：显示更多详情

## 功能特性

### 1. 负载和CPU检查
- 检查系统负载（1分钟、5分钟、15分钟）
- 检查CPU使用率（用户、系统、空闲等）
- 检查任务状态（运行、阻塞、僵尸进程）
- 检测异常：负载偏高、软中断偏高、IO使用率偏高、CPU使用率偏高

### 2. 内存检查
- 检查内存总量、可用内存、已用内存
- 检查缓冲区、缓存使用情况
- 检查内核使用内存情况
- 检测异常：内存使用率超过85%

### 3. 磁盘检查
- 检查磁盘分区和使用率
- 检查磁盘IO情况
- 检测异常：磁盘使用率过高

### 4. 线程检查
- 检查线程总数
- 检查内核线程限制
- 检测异常：线程数接近或超过限制

### 5. 网络检查
- 检查网络接口信息
- 检查网络IO情况
- 检查网络连接状态

### 6. 内核参数检查
- 检查重要内核参数的配置情况
- 评估内核参数是否合理

## 报告生成

工具可以生成 HTML 格式的服务器检查报告，包含以下内容：

- 概览
- 主机信息（主机名、时间、CPU架构、内存大小、操作系统等）
- 负载信息
- 内存信息
- 磁盘信息
- 网络信息
- 内核配置

生成的报告将存储在 `reports/` 目录中，文件名格式为 `server_report_<IP>_<时间戳>.html`。

## 依赖项

- [github.com/spf13/cobra](https://github.com/spf13/cobra) - CLI 框架
- [github.com/shirou/gopsutil/v4](https://github.com/shirou/gopsutil/v4) - 系统信息获取
- [github.com/olekukonko/tablewriter](https://github.com/olekukonko/tablewriter) - 表格输出
- [github.com/fatih/color](https://github.com/fatih/color) - 彩色终端输出

## 贡献指南

1. Fork 项目
2. 创建功能分支
3. 提交更改
4. 推送到分支
5. 打开 Pull Request

## 许可证

本项目使用 MIT 许可证。详见 [LICENSE](LICENSE) 文件。

## 版本历史

- 2025年7月2日：初始版本，添加报告生成功能
- 2025年9月：添加更多检查项和报告内容
- 2026年3月：优化代码结构，添加更多异常检测功能

## 联系方式

如有问题或建议，请通过以下方式联系：

- GitHub Issues: https://github.com/cuiwin/linux_server_check/issues
- 邮箱: your.email@example.com
