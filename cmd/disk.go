/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"fmt"

	"github.com/cuiwin/linux_server_check/checker/diskchecker"
	"github.com/cuiwin/linux_server_check/utils"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

// diskCmd represents the disk command
var diskCmd = &cobra.Command{
	Use:   "disk",
	Short: "服务器磁盘检查",
	Long:  `服务器磁盘大小使用率、磁盘IO使用率检查`,
	Run: func(cmd *cobra.Command, args []string) {

		diskcheckerInstance := diskchecker.DiskChecker{}
		data, err := diskcheckerInstance.GetDiskUsageInfo()
		if err != nil {
			fmt.Printf("Error GetDiskUsageInfo: %v\n", err)
		}

		// 开始渲染  磁盘使用率
		color.Blue("磁盘使用率：")
		header1 := []string{"Device", "Mountpoint", "Fstype", "Total", "Used", "UsedPercent(%)", "InodesUsedPercent(%)", "Status", "Desc"}
		data1 := diskcheckerInstance.GetDiskUsagePrint(data)
		utils.PrintTable(header1, data1)
	},
}

// func printTable(header []string, data [][]string) {
// 	table := tablewriter.NewWriter(os.Stdout)
// 	table.SetHeader(header)

// 	for _, row := range data {
// 		table.Append(row)
// 	}
// 	table.Render()
// }

func init() {
	checkCmd.AddCommand(diskCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// diskCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// diskCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
