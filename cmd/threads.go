/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"fmt"

	"github.com/cuiwin/linux_server_check/checker/threadchecker"
	"github.com/cuiwin/linux_server_check/utils"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

// threadsCmd represents the threads command
var threadsCmd = &cobra.Command{
	Use:   "threads",
	Short: "服务器线程相关检查",
	Long:  `服务器线程相关内核参数`,
	Run: func(cmd *cobra.Command, args []string) {
		threadcheckerInstance := threadchecker.ThreadChecker{}

		color.Blue("线程数使用率:")
		data, err := threadcheckerInstance.GetThreadUsageInfo()
		if err != nil {
			fmt.Printf("Error GetThreadUsageInfo: %v\n", err)
		}
		header1 := []string{"kernel.pid_max", "kernel.threads-max", "max user processes", "vm.max_map_count", "当前线程总数", "UsedPercent(%)", "Status", "Desc"}
		data1 := threadcheckerInstance.GetThreadUsagePrint(data)
		// 打印表格
		utils.PrintTable(header1, data1)
		//  文字描述
		color.Green("说明:")
		color.Green("kernel.pid_max = 4194304	用于控制系统中进程ID(PID)的最大值")
		color.Green("kernel.threads-max = 65535	系统允许的最大线程总数（全局限制）")
		color.Green("max user processes(ulimit -u) 	限制单个用户最大进程数")
		color.Green("vm.max_map_count = 262144	限制进程的内存映射区域数量; 例如: 运行Elasticsearch（官方建议≥262144）")

		//  文字描述
		color.Blue("线程数排行列表:")
		header_thread := []string{"PID", "name", "线程数"}
		list, err := threadcheckerInstance.GetProcessThreads()
		if err != nil {
			fmt.Printf("Error GetProcessThreads: %v\n", err)
		}
		data2 := threadcheckerInstance.GetThreadUsageRankPrint(list)
		// 打印表格
		utils.PrintTable(header_thread, data2)
	},
}

func init() {
	checkCmd.AddCommand(threadsCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// threadsCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// threadsCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
