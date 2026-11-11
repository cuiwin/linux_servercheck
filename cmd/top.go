/*
Copyright © 2024 NAME HERE <EMAIL ADDRESS>

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package cmd

import (
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/cuiwin/linux_server_check/checker/diskchecker"
	"github.com/cuiwin/linux_server_check/checker/loadchecker"
	"github.com/cuiwin/linux_server_check/checker/memchecker"
	"github.com/cuiwin/linux_server_check/checker/threadchecker"
	"github.com/spf13/cobra"
)

// topCmd represents the top command
var topCmd = &cobra.Command{
	Use:   "top",
	Short: "实时多区域监控",
	Long:  `类似top命令，同时显示CPU、内存、磁盘、线程等多个监控区域`,
	Run: func(cmd *cobra.Command, args []string) {
		duration, _ := cmd.Flags().GetInt("duration")
		runTopMonitor(duration)
	},
}

func init() {
	topCmd.Flags().IntP("duration", "d", 0, "运行时长（秒），0表示一直运行直到手动退出")
	checkCmd.AddCommand(topCmd)
}

const screenWidth = 76
const screenLines = 30

func runTopMonitor(duration int) {
	// 初始化 checker 实例
	loadChecker := loadchecker.LoadChecker{}
	memChecker := memchecker.MemChecker{}
	diskChecker := diskchecker.DiskChecker{}
	threadChecker := threadchecker.ThreadChecker{}

	// 创建退出通道
	stopChan := make(chan struct{})

	// 处理系统信号（Ctrl+C等）
	sigChan := make(chan os.Signal, 2)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// 信号处理 goroutine
	go func() {
		<-sigChan
		close(stopChan)
	}()

	// 定时更新数据
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	// 设置超时（如果指定了duration）
	var timeoutChan <-chan time.Time
	if duration > 0 {
		timeoutChan = time.After(time.Duration(duration) * time.Second)
	}

	// 主循环
	for {
		select {
		case <-stopChan:
			clearScreen()
			fmt.Println("已退出监控")
			return
		case <-timeoutChan:
			clearScreen()
			fmt.Println("已退出监控")
			return
		case <-ticker.C:
			// 获取数据
			cpuDetail, _ := loadChecker.GetDetail()
			memDetail, _ := memChecker.GetDetail()
			disks, _ := diskChecker.GetDiskUsageInfo()
			kernelParams, _ := threadChecker.GetThreadUsageInfo()
			processThreads, _ := threadChecker.GetProcessThreads()

			// 清除屏幕
			clearScreen()

			// 打印屏幕内容
			printScreen(cpuDetail, memDetail, disks, kernelParams, processThreads)

			// 强制刷新
			os.Stdout.Sync()
		}
	}
}

// 清除屏幕
func clearScreen() {
	// 使用多种方法确保清屏效果
	fmt.Print("\033[2J")   // 清除整个屏幕
	fmt.Print("\033[H")    // 移动光标到左上角
	fmt.Print("\033[?25l") // 隐藏光标
}

// 打印屏幕内容
func printScreen(cpuDetail loadchecker.CpuLoadDetail, memDetail *memchecker.MemDetail,
	disks []diskchecker.DiskUsage, kernelParams []threadchecker.KernelParams,
	processThreads []threadchecker.ProcessThreads) {

	// 使用 ANSI 转义码设置文字颜色
	const (
		colorYellow = "\033[33m"
		colorGreen  = "\033[32m"
		colorReset  = "\033[0m"
	)

	// 标题栏
	fmt.Printf("%s╔%s╗%s\n", colorYellow, strings.Repeat("═", screenWidth), colorReset)
	fmt.Printf("%s║                    🖥️  Linux Server Monitor                              ║%s\n", colorYellow, colorReset)
	fmt.Printf("%s╚%s╝%s\n", colorYellow, strings.Repeat("═", screenWidth), colorReset)

	// CPU/负载区域
	fmt.Println()
	fmt.Printf("%s【CPU/负载】%s\n", colorYellow, colorReset)
	fmt.Printf("  Load: %s%.2f %.2f %.2f%s | CPU核心: %s%d%s | 总使用率: %s%6.2f%%%s\n",
		colorGreen, cpuDetail.Load.Load1, cpuDetail.Load.Load5, cpuDetail.Load.Load15, colorReset,
		colorGreen, cpuDetail.Core, colorReset,
		colorGreen, cpuDetail.CpuUsage.Total, colorReset)
	fmt.Printf("  用户态: %s%6.2f%%%s | 系统态: %s%6.2f%%%s | 空闲: %s%6.2f%%%s\n",
		colorGreen, cpuDetail.CpuUsage.User, colorReset,
		colorGreen, cpuDetail.CpuUsage.System, colorReset,
		colorGreen, cpuDetail.CpuUsage.Idle, colorReset)
	fmt.Printf("  IO等待: %s%6.2f%%%s | 软中断: %s%6.2f%%%s\n",
		colorGreen, cpuDetail.CpuUsage.Iowait, colorReset,
		colorGreen, cpuDetail.CpuUsage.Softirq, colorReset)
	fmt.Printf("  任务: %s%d%s | 运行: %s%d%s | 阻塞: %s%d%s | 僵尸: %s%d%s\n",
		colorGreen, cpuDetail.Task.Total, colorReset,
		colorGreen, cpuDetail.Task.Running, colorReset,
		colorGreen, cpuDetail.Task.Blocked, colorReset,
		colorGreen, cpuDetail.Task.Zombie, colorReset)

	// 内存区域
	fmt.Println()
	fmt.Printf("%s【内存】%s\n", colorYellow, colorReset)
	fmt.Printf("  总内存: %s%s%s | 已用: %s%s%s | 空闲: %s%s%s\n",
		colorGreen, memDetail.MemUsage.Total, colorReset,
		colorGreen, memDetail.MemUsage.Used, colorReset,
		colorGreen, memDetail.MemUsage.Free, colorReset)
	fmt.Printf("  可用: %s%s%s | 用户进程: %s%s%s | 内核使用: %s%s%s\n",
		colorGreen, memDetail.MemUsage.Available, colorReset,
		colorGreen, memDetail.MemUsage.ProcessUserd, colorReset,
		colorGreen, memDetail.MemUsage.KernelUsed, colorReset)
	fmt.Printf("  缓冲区: %s%s%s | 缓存: %s%s%s | 使用率: %s%s%s\n",
		colorGreen, memDetail.MemUsage.Buffers, colorReset,
		colorGreen, memDetail.MemUsage.Cached, colorReset,
		colorGreen, memDetail.MemUsage.UserdPercent, colorReset)

	// 磁盘区域
	fmt.Println()
	fmt.Printf("%s【磁盘】%s\n", colorYellow, colorReset)
	for _, disk := range disks {
		fmt.Printf("  %s (%s) %s/%s (%s%s%s)\n",
			disk.Device, disk.Mountpoint, disk.Used, disk.Total,
			colorGreen, disk.UsedPercent+"%", colorReset)
	}

	// 线程区域
	fmt.Println()
	fmt.Printf("%s【线程】%s\n", colorYellow, colorReset)
	if len(kernelParams) > 0 {
		params := kernelParams[0]
		fmt.Printf("  PID最大值: %s%d%s | 线程最大值: %s%d%s | 当前线程数: %s%d%s\n",
			colorGreen, params.PidMax, colorReset,
			colorGreen, params.ThreadsMax, colorReset,
			colorGreen, params.CurrentThreads, colorReset)
		fmt.Printf("  最大映射数: %s%d%s | 用户进程限制: %s%d%s | 使用率: %s%s%s\n",
			colorGreen, params.MaxMapCount, colorReset,
			colorGreen, params.UlimitUser, colorReset,
			colorGreen, params.UsedPercent, colorReset)
	}

	// Top进程区域
	fmt.Println()
	fmt.Printf("%s【Top 进程线程】%s\n", colorYellow, colorReset)
	for i, pt := range processThreads {
		if i >= 5 {
			break
		}
		fmt.Printf("  %s%d%s  %-15s %s%d%s\n",
			colorGreen, pt.Pid, colorReset,
			pt.Name,
			colorGreen, pt.Threads, colorReset)
	}

	// 底部提示
	fmt.Println()
	fmt.Println(strings.Repeat("-", screenWidth))
	fmt.Println("  按 Ctrl+C 退出")
	fmt.Println(strings.Repeat("-", screenWidth))
}
