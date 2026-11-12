package diskioreporter

import (
	"bytes"
	"fmt"
	"log"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

// DiskIOStats 存储单个磁盘的统计信息
type DiskIOStats struct {
	Device     string  `json:"device"`
	IOrrqmS    float64 `json:"rrqm_s"`
	IOwrqmS    float64 `json:"wrqm_s"`
	IOrS       float64 `json:"r_s"`
	IOwS       float64 `json:"w_s"`
	IOrkBPS    float64 `json:"rkB_s"`
	IOwkBPS    float64 `json:"wkB_s"`
	IOavgrqSz  float64 `json:"avgrq-sz"`
	IOavgquSz  float64 `json:"avgqu-sz"`
	IOawait    float64 `json:"await"`
	IOrAwait   float64 `json:"r_await"`
	IOwAwait   float64 `json:"w_await"`
	IOsvctm    float64 `json:"svctm"`
	IOutil     float64 `json:"util"`
	IOPS       float64 `json:"iops"`
	Throughput float64 `json:"throughput"`
	Status     string  `json:"status"`
	Desc       string  `json:"desc"`
}

// Checker 结构体定义
type DiskIOReporter struct {
	// 这里可以添加 Checker 结构体的字段
}

// getDiskStats 执行iostat命令并解析结果
func (c *DiskIOReporter) GetDiskStats(version string) ([]DiskIOStats, error) {
	// 修改命令为 iostat -d -x 1 2 | awk '/Device:/{count++} count==2'
	// 这里用第二次结果更准确
	cmd := exec.Command("sh", "-c", "iostat -d -x 1 2 | awk '/Device/{count++} count==2' | grep -v Device")

	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out

	err := cmd.Run()
	if err != nil {
		log.Fatalf("命令执行失败: %v\n输出: %s", err, out.String())
	}

	// 打印原始输出用于调试
	// fmt.Println("命令输出:")
	// fmt.Println(out.String())

	// 解析输出
	diskStatsList, err := parseIostatOutput(out.String(), version)
	if err != nil {
		fmt.Printf("解析iostat命令结果失败: %v\n", err)
	}

	return diskStatsList, nil
}

// 检测iostat版本
func (c *DiskIOReporter) GetIostatVersion() (string, error) {
	// 方法1: 尝试 rpm (CentOS/RHEL)
	cmd := exec.Command("rpm", "-q", "sysstat")
	output, err := cmd.CombinedOutput()
	if err == nil {
		re := regexp.MustCompile(`sysstat-(\d+\.\d+\.\d+)-`)
		matches := re.FindStringSubmatch(string(output))
		if len(matches) >= 2 {
			return matches[1], nil
		}
	}

	// 方法2: 尝试 dpkg (Debian/Ubuntu)
	cmd = exec.Command("dpkg", "-s", "sysstat")
	output, err = cmd.CombinedOutput()
	if err == nil {
		re := regexp.MustCompile(`Version:\s*(\d+\.\d+\.\d+)`)
		matches := re.FindStringSubmatch(string(output))
		if len(matches) >= 2 {
			return matches[1], nil
		}
	}

	// 方法3: 尝试 iostat -V 输出 (通用)
	cmd = exec.Command("sh", "-c", "iostat -V 2>&1")
	output, err = cmd.CombinedOutput()
	if err == nil {
		re := regexp.MustCompile(`(\d+\.\d+\.\d+)`)
		matches := re.FindStringSubmatch(string(output))
		if len(matches) >= 2 {
			return matches[1], nil
		}
	}

	return "", fmt.Errorf("无法检测iostat版本")
}

func parseVersion_10(output string) ([]DiskIOStats, error) {

	lines := strings.Split(output, "\n")
	var diskStatsList []DiskIOStats

	for _, line := range lines {
		trimmedLine := strings.TrimSpace(line)
		if trimmedLine == "" {
			continue
		}

		fields := strings.Fields(trimmedLine)

		if len(fields) >= 14 {
			stats := DiskIOStats{
				Device: fields[0],
			}

			// 解析数值字段
			stats.IOrrqmS = parseFloat(fields[1])
			stats.IOwrqmS = parseFloat(fields[2])
			stats.IOrS = parseFloat(fields[3])
			stats.IOwS = parseFloat(fields[4])
			stats.IOrkBPS = parseFloat(fields[5])
			stats.IOwkBPS = parseFloat(fields[6])
			stats.IOavgrqSz = parseFloat(fields[7])
			stats.IOavgquSz = parseFloat(fields[8])
			stats.IOawait = parseFloat(fields[9])
			stats.IOrAwait = parseFloat(fields[10])
			stats.IOwAwait = parseFloat(fields[11])
			stats.IOsvctm = parseFloat(fields[12])
			stats.IOutil = parseFloat(fields[13])
			stats.IOPS = stats.IOrS + stats.IOwS
			stats.Throughput = (stats.IOrkBPS + stats.IOwkBPS) / 1024

			// 设置状态和描述
			stats.Status = getDeviceStatus(stats.IOutil)
			stats.Desc = getDeviceDesc(stats.IOutil)

			diskStatsList = append(diskStatsList, stats)
		}
	}
	return diskStatsList, nil
}

// parseIostatOutput 解析iostat命令的输出，只保留第二次采样
func parseIostatOutput(output string, iostatversion string) ([]DiskIOStats, error) {

	var err error
	var stats []DiskIOStats

	// 根据版本选择不同的解析函数
	switch {
	case iostatversion == "":
		// 版本未知，尝试自动检测格式: 根据列数判断
		fmt.Println("iostat未知版本, 尝试自动检测格式")
		// sysstat 10.x+ 有14列(含rrqm/s, wrqm/s), 旧版12列(不含r_await, w_await)
		lines := strings.Split(output, "\n")
		for _, line := range lines {
			fields := strings.Fields(strings.TrimSpace(line))
			if len(fields) == 0 {
				continue
			}
			// 检查第一行数据是否包含设备名作为起点
			if len(fields) >= 14 {
				// 14+ 列 -> sysstat 10.x 格式
				fmt.Println("自动检测为 sysstat 10.x 格式")
				stats, err = parseVersion_10(output)
			} else {
				fmt.Println("自动检测为 sysstat 旧版格式")
				// 旧版格式也尝试用10的解析器(字段可能错位但能工作)
				stats, err = parseVersion_10(output)
			}
			break
		}

	case strings.HasPrefix(iostatversion, "10."), strings.HasPrefix(iostatversion, "11."), strings.HasPrefix(iostatversion, "12."):
		stats, err = parseVersion_10(output)

	default:
		// 未知格式也尝试解析
		stats, err = parseVersion_10(output)
	}
	if err != nil {
		return nil, err
	}

	return stats, nil
}

// getDeviceStatus 根据利用率获取设备状态
func getDeviceStatus(util float64) string {
	switch {
	case util > 90:
		return "warning"
	default:
		return "info"
	}
}

func getDeviceDesc(util float64) string {
	switch {
	case util > 90:
		return "IO繁忙"
	default:
		return "IO正常"
	}
}

// getDeviceDescription 生成设备描述信息
// func getDeviceDescription(stats DiskIOStats) string {
// 	iops := stats.IOrS + stats.IOwS
// 	throughput := (stats.IOrkBPS + stats.IOwkBPS) / 1024 // MB/s

// 	return fmt.Sprintf("IOPS=%.1f, 吞吐量=%.2f MB/s, 平均等待时间=%.2f ms",
// 		iops, throughput, stats.IOawait)
// }

// parseFloat 安全地将字符串转换为浮点数
func parseFloat(s string) float64 {
	if val, err := strconv.ParseFloat(s, 64); err == nil {
		return val
	}
	return 0
}
