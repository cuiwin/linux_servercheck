/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/cuiwin/linux_server_check/checker/hostchecker"
	"github.com/cuiwin/linux_server_check/checker/loadchecker"
	"github.com/cuiwin/linux_server_check/config"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

// analysisCmd represents the analysis command
var analysisCmd = &cobra.Command{
	Use:   "analysis",
	Short: "服务器根因分析",
	Long:  `对服务器进行CPU负载根因分析，收集CPU指标并输出结构化诊断结论`,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("开始服务器根因分析...")
		fmt.Println()

		// ===== 并行采集各指标 =====
		startTime := time.Now()
		color.New(color.Bold, color.FgCyan).Println("【数据采集】")
		fmt.Println()

		var (
			wg sync.WaitGroup
			mu sync.Mutex

			hostInfo       hostchecker.HostInfo
			hostErr        error
			cpuLoadResults []loadchecker.CpuLoadDetail
		)

		collect := func(name string, fn func()) {
			wg.Add(1)
			go func() {
				defer wg.Done()
				fn()
				mu.Lock()
				color.Green("  [完成] %s", name)
				mu.Unlock()
			}()
		}

		// 主机信息
		collect("主机信息", func() {
			hostChecker := hostchecker.HostChecker{}
			hostInfo, hostErr = hostChecker.GetHostInfo()
		})

		// CPU负载
		collect("CPU负载", func() {
			loadChecker := loadchecker.LoadChecker{}
			for i := 0; i < analysisCount; i++ {
				info, err := loadChecker.GetDetail()
				if err != nil {
					log.Printf("第%d次获取CPU负载失败: %v", i+1, err)
					continue
				}
				cpuLoadResults = append(cpuLoadResults, info)

				// 上下文切换是累计值，需要相邻两次采样做差才有速率，因此留出 1 秒采样间隔
				if i < analysisCount-1 {
					time.Sleep(time.Second)
				}
			}
		})

		wg.Wait()

		if hostErr != nil {
			return fmt.Errorf("获取主机信息失败: %w", hostErr)
		}

		color.Green("  采集完成，用时 %.1fs\n", time.Since(startTime).Seconds())

		// ===== 构建分析数据并分析 =====
		reportData := ReportData{
			HostInfo:          hostInfo,
			Timestamp:         time.Now().Format("2006-01-02 15:04:05"),
			CpuLoadDetailList: cpuLoadResults,
		}
		analyzedData := analyzeReportData(reportData)

		// ===== 输出诊断结论 =====
		color.New(color.Bold, color.FgCyan).Println("【诊断结论】")
		fmt.Println()

		// 输出 load-info 摘要项对应的异常采样描述
		printLoadDesc := func(item SummaryInfo) {
			if item.SectionId != "load-info" {
				return
			}
			for i, r := range cpuLoadResults {
				if r.Status == "info" {
					continue
				}
				fmt.Printf("        第%d次采样: %s\n", i+1, r.Desc)
			}
		}

		// Error 级别
		if len(analyzedData.ErrorSummaryList) > 0 {
			color.New(color.Bold, color.FgRed).Println("  错误 (需要立即处理):")
			for _, item := range analyzedData.ErrorSummaryList {
				fmt.Printf("    - [%s] %s\n", item.SectionId, item.Message)
				printLoadDesc(item)
			}
			fmt.Println()
		}

		// Warning 级别
		if len(analyzedData.WarningSummaryList) > 0 {
			color.New(color.Bold, color.FgYellow).Println("  警告 (建议关注):")
			for _, item := range analyzedData.WarningSummaryList {
				fmt.Printf("    - [%s] %s\n", item.SectionId, item.Message)
				printLoadDesc(item)
			}
			fmt.Println()
		}

		// Normal 级别
		if len(analyzedData.NormalSummaryList) > 0 {
			color.New(color.Bold, color.FgGreen).Println("  正常:")
			for _, item := range analyzedData.NormalSummaryList {
				fmt.Printf("    - [%s] %s\n", item.SectionId, item.Message)
			}
			fmt.Println()
		}

		// ===== 输出详情 =====
		color.New(color.Bold, color.FgCyan).Println("【详细信息】")
		fmt.Println()

		// 主机详细信息
		color.Blue("  主机详细信息:")
		fmt.Printf("    IP: %s    Hostname: %s\n", hostInfo.IpAddr, hostInfo.Hostname)
		fmt.Printf("    OS: %s    Kernel: %s    Arch: %s\n", hostInfo.OS, hostInfo.KernelVersion, hostInfo.KernelArch)
		fmt.Printf("    Platform: %s %s    CPU: %s\n", hostInfo.Platform, hostInfo.PlatformVersion, hostInfo.ModelName)
		fmt.Printf("    CPU核心: %d    内存总量: %.2fGB    数据盘总量: %.2fGB\n", hostInfo.Core, hostInfo.MemoryTotal, hostInfo.DataDiskTotal)
		fmt.Printf("    运行时间: %s    启动时间: %s\n\n", hostInfo.Uptime, hostInfo.BootTime)

		// CPU负载详情（仅在 -v/--detail 时输出）
		detail, _ := cmd.Flags().GetBool("detail")
		if detail && len(cpuLoadResults) > 0 {
			color.Blue("  CPU Sampling History (sampled at %s):", time.Now().Format("15:04:05"))
			fmt.Println()

			// cell 先按宽度补齐纯文本再着色，避免颜色转义符破坏对齐
			cell := func(format string, width int, v any, c *color.Color) string {
				return c.Sprintf("%-*s", width, fmt.Sprintf(format, v))
			}
			grade := func(v, threshold float64) *color.Color {
				if v >= threshold {
					return color.New(color.FgRed)
				}
				return color.New(color.FgGreen)
			}
			green := color.New(color.FgGreen)

			// 负载与整体 CPU
			fmt.Printf("  %-3s %-7s %-7s %-7s %-5s %-8s %s\n", "#", "Load1", "Load5", "Load15", "Core", "Status", "Desc")
			for i, r := range cpuLoadResults {
				c := green
				if r.LoadStatus != "info" {
					c = color.New(color.FgRed)
				}
				fmt.Printf("  %-3d %s %s %s %-5d %s %s\n",
					i+1,
					cell("%.2f", 7, r.Load.Load1, c),
					cell("%.2f", 7, r.Load.Load5, c),
					cell("%.2f", 7, r.Load.Load15, c),
					r.Core,
					cell("%s", 8, r.LoadStatus, c),
					c.Sprint(r.LoadDesc))
			}
			fmt.Println()

			// CPU 使用明细
			fmt.Printf("  %-3s %-7s %-7s %-7s %-6s %-7s %-7s %-8s %-8s %-7s %-8s %s\n", "#", "Total", "User", "System", "Nice", "Idle", "IOWait", "HardIRQ", "SoftIRQ", "Steal", "Status", "Desc")
			for i, r := range cpuLoadResults {
				c := green
				if r.CpuStatus != "info" {
					c = color.New(color.FgRed)
				}
				fmt.Printf("  %-3d %s %s %s %s %s %s %s %s %s %s %s\n",
					i+1,
					cell("%.1f", 7, r.CpuUsage.Total, c),
					cell("%.1f", 7, r.CpuUsage.User, grade(r.CpuUsage.User, config.CpuUserPercent)),
					cell("%.1f", 7, r.CpuUsage.System, grade(r.CpuUsage.System, config.CpuSystemPercent)),
					cell("%.1f", 6, r.CpuUsage.Nice, green),
					cell("%.1f", 7, r.CpuUsage.Idle, green),
					cell("%.1f", 7, r.CpuUsage.Iowait, grade(r.CpuUsage.Iowait, config.CpuIowaitPercent)),
					cell("%.1f", 8, r.CpuUsage.Hardirq, grade(r.CpuUsage.Hardirq, config.CpuHardirqPercent)),
					cell("%.1f", 8, r.CpuUsage.Softirq, grade(r.CpuUsage.Softirq, config.CpuSoftirqPercent)),
					cell("%.1f", 7, r.CpuUsage.Steal, green),
					cell("%s", 8, r.CpuStatus, c),
					c.Sprint(r.CpuDesc))
			}
			fmt.Println()

			// 任务与线程
			fmt.Printf("  %-3s %-7s %-8s %-8s %-7s %-10s %-8s %-8s %s\n", "#", "Total", "Running", "Blocked", "Zombie", "SinglePro", "Threads", "Status", "Desc")
			for i, r := range cpuLoadResults {
				c := green
				if r.TaskStatus != "info" {
					c = color.New(color.FgRed)
				}
				fmt.Printf("  %-3d %s %s %s %s %s %s %s %s\n",
					i+1,
					cell("%d", 7, r.Task.Total, green),
					cell("%d", 8, r.Task.Running, green),
					cell("%d", 8, r.Task.Blocked, green),
					cell("%d", 7, r.Task.Zombie, green),
					cell("%d", 10, r.Task.SinglePro, green),
					cell("%d", 8, r.Task.Threads, green),
					cell("%s", 8, r.TaskStatus, c),
					c.Sprint(r.TaskDesc))
			}
			fmt.Println()

			// 上下文切换
			fmt.Printf("  %-3s %-10s %-10s %-10s %-8s\n", "#", "Ctxt/s", "Vol/s", "NVol/s", "NVol%")
			for i, r := range cpuLoadResults {
				if !r.Ctxt.Valid {
					fmt.Printf("  %-3d %-10s %-10s %-10s %-8s\n", i+1, "-", "-", "-", "-")
					continue
				}
				ratio := r.Ctxt.NonVoluntaryRatio * 100
				ratioColor := green
				if ratio >= 50 {
					ratioColor = color.New(color.FgRed)
				}
				fmt.Printf("  %-3d %s %s %s %s\n",
					i+1,
					cell("%.1f", 10, r.Ctxt.Rate, green),
					cell("%.1f", 10, r.Ctxt.VoluntaryRate, green),
					cell("%.1f", 10, r.Ctxt.NonVoluntaryRate, green),
					cell("%.0f%%", 8, ratio, ratioColor))
			}
			color.New(color.Faint).Println("  说明: 主动+非自愿之和略小于 Ctxt/s 属正常，ctxt 还包含进出 idle 及采样期间已退出任务的切换")
			fmt.Println()

			// 最新一帧的切换 TOP5（优先取有基线的那一帧）
			for i := len(cpuLoadResults) - 1; i >= 0; i-- {
				if !cpuLoadResults[i].Ctxt.Valid {
					continue
				}
				printCtxtTop("进程自身（主线程）切换 TOP5", cpuLoadResults[i].Ctxt.TopProcesses, false)
				printCtxtTop("含全部线程切换 TOP5", cpuLoadResults[i].Ctxt.TopThreaded, true)
				break
			}
		}

		// 总体结论
		fmt.Println()
		if len(analyzedData.ErrorSummaryList) > 0 {
			color.New(color.Bold, color.FgRed).Println("总结: 服务器存在异常,请尽快排查处理")
		} else if len(analyzedData.WarningSummaryList) > 0 {
			color.New(color.Bold, color.FgYellow).Println("总结: 服务器存在潜在风险,建议持续关注")
		} else {
			color.New(color.Bold, color.FgGreen).Println("总结: 服务器各项指标正常")
		}

		return nil
	},
}

var analysisCount int

func init() {
	rootCmd.AddCommand(analysisCmd)
	analysisCmd.Flags().IntVarP(&analysisCount, "count", "c", 5, "指定CPU指标的检查次数")
}

// printCtxtTop 打印上下文切换 TOP 榜单
// withThreads 为 true 时展示该进程所有线程之和（对应 pidstat -wt），否则只展示主线程自身（对应 pidstat -w）
func printCtxtTop(title string, list []loadchecker.ProcCtxtSwitch, withThreads bool) {
	color.Blue("  %s:", title)
	if len(list) == 0 {
		fmt.Println("    本间隔无切换")
		fmt.Println()
		return
	}
	if withThreads {
		fmt.Printf("    %-7s %-34s %-8s %-9s %-9s %s\n", "PID", "NAME", "THREADS", "VOL", "NVOL", "TOTAL")
	} else {
		fmt.Printf("    %-7s %-34s %-9s %-9s %s\n", "PID", "NAME", "VOL", "NVOL", "TOTAL")
	}
	for _, p := range list {
		if withThreads {
			fmt.Printf("    %-7d %-34s %-8d %-9d %-9d %d\n",
				p.PID, padStr(p.Name, 34, 0), p.Threads, p.Voluntary, p.NonVoluntary, p.Total)
		} else {
			fmt.Printf("    %-7d %-34s %-9d %-9d %d\n",
				p.PID, padStr(p.Name, 34, 0), p.Voluntary, p.NonVoluntary, p.Total)
		}
	}
	fmt.Println()
}
