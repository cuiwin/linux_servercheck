package utils

import (
	"fmt"
	"net"
	"os"

	"github.com/olekukonko/tablewriter"
)

// GetLocalIP 函数用于获取主机的非回环IPv4地址
func GetLocalIP() (string, error) {
	// 获取所有网络接口地址
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "", fmt.Errorf("获取网络接口地址失败：%w", err)
	}

	// 遍历所有地址，筛选出非回环的IPv4地址
	for _, addr := range addrs {
		// 检查地址类型，确保是IPNet类型
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			// 进一步判断是否为IPv4地址
			if ipnet.IP.To4() != nil {
				return ipnet.IP.String(), nil
			}
		}
	}

	return "", fmt.Errorf("未找到非回环IPv4地址")
}

// 打印表格
func PrintTable(header []string, data [][]string) {
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader(header)

	for _, row := range data {
		table.Append(row)
	}
	table.Render()
}
