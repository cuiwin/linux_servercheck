/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// checkCmd represents the check command
var checkCmd = &cobra.Command{
	Use:   "check",
	Short: "服务器检查",
	Long:  `检查服务器负载,CPU,内存,磁盘,网络等异常信息`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Available Commands:")
		fmt.Println("linux_server_check check load   		服务器负载和CPU检查")
		fmt.Println("linux_server_check check mem    		服务器内存检查")
		fmt.Println("linux_server_check check disk   		服务器磁盘检查")
		fmt.Println("linux_server_check check threads		服务器线程数检查")
	},
}

func init() {
	rootCmd.AddCommand(checkCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// checkCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// checkCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
