/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/cuiwin/linux_server_check/checker/memchecker"
	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
)

// memCmd represents the mem command
var memCmd = &cobra.Command{
	Use:   "mem",
	Short: "服务器内存检查",
	Long:  `服务器内存总体使用分析`,
	Run: func(cmd *cobra.Command, args []string) {

		// 获取命令行参数
		loopCount, _ := cmd.Flags().GetInt("count")

		// 初始化表格
		table := tablewriter.NewWriter(os.Stdout)
		// 表头
		header := []string{"DateTime", "Total", "Free", "Available", "Buffers", "Cached", "Used(总)", "ProcessUserd(用户)",
			"KernelUsed(内核)", "Slab", "SUnreclaim", "VmallocUsed", "BlackHole", "UserdPercent", "状态", "描述"}
		table.SetHeader(header)
		table.SetBorder(true)
		table.SetAlignment(tablewriter.ALIGN_RIGHT)
		table.SetHeaderAlignment(tablewriter.ALIGN_CENTER)

		// 每秒打印一行新数据
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()

		// 处理中断信号，实现优雅退出
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

		memcheckerInstance := memchecker.MemChecker{}

		for count := 0; count < loopCount; count++ {
			select {
			case <-ticker.C:
				// 生成新数据
				info, err := memcheckerInstance.GetDetail()
				if err != nil {
					fmt.Printf("Error counting task states: %v\n", err)
				}
				data := memcheckerInstance.PrintDetail(*info)
				// 添加新数据行
				table.Append(data)
				// 重新渲染表格，这里我们通过清屏和重新渲染来实现类似追加效果且表头固定
				fmt.Print("\033[H\033[2J")
				// 重新渲染表格
				table.Render()
			case <-sigChan:
				fmt.Println("\n收到中断信号，正在退出...")
				return

			}
		}

	},
}

func init() {
	checkCmd.AddCommand(memCmd)

	// 添加命令行参数
	memCmd.Flags().IntP("count", "c", 20, "检查次数")

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// memCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// memCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
