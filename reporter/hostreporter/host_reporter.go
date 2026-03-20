package hostreporter

import (
	"fmt"
	"log"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/cuiwin/linux_server_check/utils"
	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/host"
	"github.com/shirou/gopsutil/v4/mem"
)

// 主机信息
type HostInfo struct {
	Hostname        string  `json:"hostname"`
	KernelArch      string  `json:"kernelArch"`
	KernelVersion   string  `json:"kernelVersion"`
	Platform        string  `json:"platform"`
	PlatformFamily  string  `json:"platformFamily"`
	PlatformVersion string  `json:"platformVersion"`
	OS              string  `json:"os"`
	Uptime          string  `json:"uptime"`
	BootTime        string  `json:"bootTime"`
	ModelName       string  `json:"modelName"`
	Core            int     `json:"core"`
	MemoryTotal     float64 `json:"memoryTotal"`
	DataDiskTotal   float64 `json:"dataDiskTotal"`
	IpAddr          string  `json:"ipaddr"`
}

// Checker 结构体定义
type HostReporter struct {
	// 这里可以添加 Checker 结构体的字段
}

func secondsToTime(totalSeconds uint64) string {
	hours := totalSeconds / 3600
	remainingSeconds := totalSeconds % 3600
	minutes := remainingSeconds / 60
	seconds := remainingSeconds % 60
	return fmt.Sprintf("%d:%02d:%02d", hours, minutes, seconds)
}

// 获取cpu核心数
func getCoreNum() (int, error) {
	cmdstr := "cat /proc/cpuinfo | grep processor | wc -l"
	cmd := exec.Command("sh", "-c", cmdstr)
	out, err1 := cmd.CombinedOutput()
	if err1 != nil {
		// fmt.Println(err1)
		return 0, err1
	}
	// 获取的结果多了一个'\n', 需要删除
	count, err2 := strconv.Atoi(strings.TrimSuffix(string(out), "\n"))
	if err2 != nil {
		// fmt.Println(err2)
		return 0, err2
	}
	return count, nil
}

// 获取主机信息
func (c *HostReporter) GetHostInfo() (HostInfo, error) {

	// gopsutil host
	tempinfo, _ := host.Info()

	// gopsutils cpu
	tempcpu, _ := cpu.Info()
	modelName := tempcpu[0].ModelName

	cpuCount, err := getCoreNum()
	if err != nil {
		fmt.Println("获取CPU核心数量失败:", err)
	}

	// 获取系统内存信息
	memusage, err := mem.VirtualMemory()
	if err != nil {
		log.Fatalf("获取内存信息失败: %v", err)
	}
	// fmt.Printf("系统内存总量: %.2f GB\n", float64(memusage.Total)/(1024*1024*1024))

	// 获取/data目录挂载磁盘信息
	usage, err := disk.Usage("/data")
	if err != nil {
		log.Fatalf("获取磁盘信息失败: %v", err)
	}
	// fmt.Printf("/data目录挂载磁盘总量: %.2f GB\n", float64(usage.Total)/(1024*1024*1024))

	// 获取IP地址
	ipaddr, err := utils.GetLocalIP()
	if err != nil {
		log.Fatalf("获取IP信息失败: %v", err)
	}

	bootTime := time.Unix(int64(tempinfo.BootTime), 0)

	hostinfo := HostInfo{
		Hostname:        tempinfo.Hostname,
		KernelArch:      tempinfo.KernelArch,
		KernelVersion:   tempinfo.KernelVersion,
		Platform:        tempinfo.Platform,
		PlatformFamily:  tempinfo.PlatformFamily,
		PlatformVersion: tempinfo.PlatformVersion,
		OS:              tempinfo.OS,
		Uptime:          secondsToTime(tempinfo.Uptime),
		BootTime:        bootTime.Format(time.RFC3339),
		ModelName:       modelName,
		Core:            cpuCount,
		MemoryTotal:     float64(memusage.Total) / (1024 * 1024 * 1024),
		DataDiskTotal:   float64(usage.Total) / (1024 * 1024 * 1024),
		IpAddr:          ipaddr,
	}

	return hostinfo, nil
}
