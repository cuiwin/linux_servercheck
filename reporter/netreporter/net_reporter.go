package netreporter

import (
	"bufio"
	"fmt"
	"net"
	"os/exec"
	"strconv"
	"strings"
)

// NetworkInterface 定义网卡信息结构体
type NetworkInterface struct {
	Name        string   // 网卡名称
	MACAddress  string   // MAC 地址
	IPAddresses []string // IP 地址列表
	Status      string   // 网卡状态
	IsRunning   bool     // 是否正在运行
}

// NetworkInterfaceStats 表示网络接口的统计信息
type NetworkInterfaceStats struct {
	Interface             string  // 网卡名称
	ReceivePackets        float64 // 接收包数/秒
	TransmitPackets       float64 // 发送包数/秒
	ReceiveKBytes         float64 // 接收KB/秒
	TransmitKBytes        float64 // 发送KB/秒
	ReceiveCompressed     float64 // 接收压缩包数/秒
	TransmitCompressed    float64 // 发送压缩包数/秒
	ReceiveMulticast      float64 // 接收多播包数/秒
	ReceiveAvgPacketSize  float64 // 接收数据包平均大小(字节)
	TransmitAvgPacketSize float64 // 发送数据包平均大小(字节)
	Status                string
	Desc                  string
}

// Checker 结构体定义
type NetReporter struct {
	// 这里可以添加 Checker 结构体的字段
}

// 获取网卡信息
func (c *NetReporter) GetNetworkInterfaceList() ([]NetworkInterface, error) {

	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, fmt.Errorf("获取网络接口失败: %v", err)
	}

	var networkInterfaces []NetworkInterface

	// 遍历网卡并收集信息到结构体
	for _, iface := range ifaces {
		ifaceInfo := NetworkInterface{
			Name:       iface.Name,
			MACAddress: iface.HardwareAddr.String(),
			Status:     "已禁用",
			IsRunning:  false,
		}

		// 设置网卡状态
		if iface.Flags&net.FlagUp != 0 {
			ifaceInfo.Status = "已启用"
		}

		// 设置运行状态
		if iface.Flags&net.FlagRunning != 0 {
			ifaceInfo.IsRunning = true
		}

		// 获取 IP 地址
		addrs, err := iface.Addrs()
		if err == nil {
			for _, addr := range addrs {
				ifaceInfo.IPAddresses = append(ifaceInfo.IPAddresses, addr.String())
			}
		}

		networkInterfaces = append(networkInterfaces, ifaceInfo)

	}

	return networkInterfaces, nil
}

// GetNetworkStats 执行sar命令并解析结果
func (c *NetReporter) GetNetworkStatsList() ([]NetworkInterfaceStats, error) {
	// 执行sar命令获取网络接口统计信息
	cmd := exec.Command("sar", "-n", "DEV", "1", "1")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("执行sar命令失败: %v，输出: %s", err, string(output))
	}

	// 解析输出
	return parseSarOutput(string(output))
}

// parseSarOutput 解析sar命令的输出
func parseSarOutput(output string) ([]NetworkInterfaceStats, error) {
	var stats []NetworkInterfaceStats
	scanner := bufio.NewScanner(strings.NewReader(output))

	// 标记是否找到数据起始行
	foundData := false

	// 定义可能的平均值前缀，支持中英文系统
	avgPrefixes := []string{"平均时间:", "Average:"}

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// 跳过空行
		if line == "" {
			continue
		}

		// 查找数据起始行
		if !foundData && strings.Contains(line, "IFACE") {
			foundData = true
			continue
		}

		// 解析数据行 - 处理不同语言的平均值前缀
		if foundData {
			var fields []string
			var prefixFound bool

			// 检查是否包含平均值前缀
			for _, prefix := range avgPrefixes {
				if strings.Contains(line, prefix) {
					// 移除前缀并处理剩余部分
					fields = strings.Fields(strings.TrimPrefix(line, prefix))
					prefixFound = true
					break
				}
			}

			// 如果未找到匹配的前缀或字段不足，跳过此行
			if !prefixFound || len(fields) < 8 {
				continue
			}

			// 排除标题行（如果第一个字段是"IFACE"）
			if fields[0] == "IFACE" {
				continue
			}

			// 解析各字段
			iface := fields[0]

			rxpck, err := strconv.ParseFloat(fields[1], 64)
			if err != nil {
				rxpck = 0
			}

			txpck, err := strconv.ParseFloat(fields[2], 64)
			if err != nil {
				txpck = 0
			}

			rxkb, err := strconv.ParseFloat(fields[3], 64)
			if err != nil {
				rxkb = 0
			}

			txkb, err := strconv.ParseFloat(fields[4], 64)
			if err != nil {
				txkb = 0
			}

			rxcmp, err := strconv.ParseFloat(fields[5], 64)
			if err != nil {
				rxcmp = 0
			}

			txcmp, err := strconv.ParseFloat(fields[6], 64)
			if err != nil {
				txcmp = 0
			}

			rxmcst, err := strconv.ParseFloat(fields[7], 64)
			if err != nil {
				rxmcst = 0
			}

			// 计算平均包大小(字节)，避免除零错误
			rxAvgSize := 0.0
			if rxpck > 0 {
				rxAvgSize = (rxkb * 1024) / rxpck
			}

			txAvgSize := 0.0
			if txpck > 0 {
				txAvgSize = (txkb * 1024) / txpck
			}

			var tempStatus string
			var tempDescribe string
			if rxkb > float64(25600) {
				tempStatus = "warning"
				tempDescribe = "接收超过25MB/s"
			} else {
				tempStatus = "info"
				tempDescribe = "接收流量正常"
			}

			if txkb > float64(25600) {
				tempStatus = "warning"
				tempDescribe += ";发送超过25MB/s"
			} else {
				tempStatus = "info"
				tempDescribe += ";发送流量正常"
			}

			// 添加到结果
			stats = append(stats, NetworkInterfaceStats{
				Interface:             iface,
				ReceivePackets:        rxpck,
				TransmitPackets:       txpck,
				ReceiveKBytes:         rxkb,
				TransmitKBytes:        txkb,
				ReceiveCompressed:     rxcmp,
				TransmitCompressed:    txcmp,
				ReceiveMulticast:      rxmcst,
				ReceiveAvgPacketSize:  rxAvgSize,
				TransmitAvgPacketSize: txAvgSize,
				Status:                tempStatus,
				Desc:                  tempDescribe,
			})
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("扫描输出时出错: %v", err)
	}

	return stats, nil
}
