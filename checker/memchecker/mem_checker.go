package memchecker

import (
	"errors"
	"fmt"
	"log"
	"os/exec"
	"reflect"
	"regexp"
	"strconv"
	"time"

	"github.com/cuiwin/linux_server_check/config"
	"github.com/cuiwin/linux_server_check/utils"
)

// MemInfo 保存从 /proc/meminfo 解析出的原始内存数据，单位为 kB
type MemInfo struct {
	MemTotal          int64 // 系统可用的物理内存总量
	MemFree           int64 // 完全空闲的内存
	MemAvailable      int64 // 应用可用的内存估算值（含可回收的缓存）
	Buffers           int64 // 块设备缓冲占用
	Cached            int64 // 页缓存占用（不含 SwapCached）
	Active            int64 // 活跃页内存（不易被回收）
	Inactive          int64 // 非活跃页内存（易被回收）
	Unevictable       int64 // 不可回收的内存（如被 mlock 锁定）
	Slab              int64 // 内核 Slab 分配器占用总量
	SReclaimable      int64 // Slab 中可回收的部分
	SUnreclaim        int64 // Slab 中不可回收的部分
	KernelStack       int64 // 内核协议栈占用
	PageTables        int64 // 页表占用
	Bounce            int64 // 弹跳缓冲区占用
	VmallocUsed       int64 // 已使用的 vmalloc 虚拟内存
	HardwareCorrupted int64 // 内核标记为损坏、不可用的内存
	HugePages_Total   int64 // 大页总数
	Hugepagesize      int64 // 单个大页的大小
}

// MemUsage 保存格式化后用于展示的内存使用数据（人类可读的大小字符串）
type MemUsage struct {
	Total        string // 内存总量
	ProcessUserd string // 进程实际使用量（total - free - buffers - cached）
	Free         string // 空闲内存
	Available    string // 可用内存
	Buffers      string // 缓冲区占用
	Cached       string // 页缓存占用
	Used         string // 已使用内存（total - free）
	KernelUsed   string // 内核占用（slab + vmalloc + 页表 + 内核栈 + 损坏 + 弹跳区）
	Slab         string // Slab 占用
	SUnreclaim   string // Slab 中不可回收部分
	VmallocUsed  string // vmalloc 使用量
	BlackHole    string // 无法归类、统计不出的内存（已使用内存减去用户与内核占用）
	UserdPercent string // 内存使用率百分比
}

// MemDetail 是内存检查的完整结果，包含内存使用情况、主机信息与检查结论
type MemDetail struct {
	MemUsage MemUsage // 详细的内存使用数据
	IP       string   // 主机 IP 地址
	Status   string   // 检查状态（如 info、warning）
	Desc     string   // 状态描述

	// 内存占用最高的前 N 个进程，用于直接定位内存热点
	TopMemProcs []ProcMemUsage
	// TOP 进程的内存归因结论
	ProcSummary ProcMemSummary
}

// 获取内存信息
func getMeminfo() (string, error) {
	cmdstr := "cat /proc/meminfo"
	cmd := exec.Command("sh", "-c", cmdstr)
	ret_meminfo, err1 := cmd.CombinedOutput()
	if err1 != nil {
		// fmt.Println(err1)
		return "", err1
	}
	return string(ret_meminfo), nil
}

// 获取ret_vmalloc内存信息
func getVmallocinfo() (string, error) {
	cmdstr := "grep vmalloc  /proc/vmallocinfo |awk '{total += $2};END {print total/1024}'"
	cmd := exec.Command("sh", "-c", cmdstr)
	ret_vmalloc, err1 := cmd.CombinedOutput()
	if err1 != nil {
		// fmt.Println(err1)
		return "", err1
	}
	return string(ret_vmalloc), nil
}

// Checker 结构体定义
type MemChecker struct {
	// 跳过进程级下钻采集。
	// 连续多次采样时（如 report 命令）只在最后一次采集，避免重复执行 ps。
	SkipTopProcs bool
	// AlwaysTopProcs 无论是否触发阈值都采集进程 TOP N。
	// TUI 仪表盘需要始终展示内存热点，命令行报告仍保持按阈值触发。
	AlwaysTopProcs bool
}

// 正则匹配
func findNum(raw string, key string) (string, error) {
	re := regexp.MustCompile(key + `:\s+(\d+)\s`)
	matches := re.FindStringSubmatch(raw)
	if len(matches) != 2 {
		//解析的有问题
		return "", errors.New("find key" + key + "error")
	} else {
		return matches[1], nil
	}
}

func formatMemSize(size int64) string {
	// /proc/meminfo 中的数值单位为 kB，换算成字节后统一格式化
	return utils.FormatSize(uint64(size) * 1024)
}

func (c *MemChecker) GetDetail() (*MemDetail, error) {

	var err error

	str_meminfo, err := getMeminfo()
	if err != nil {
		fmt.Println("获取/proc/meminfo失败:", err)
		return nil, err
	}

	ret_vmalloc, err := getVmallocinfo()
	if err != nil {
		fmt.Println("获取/proc/vmallocinfo失败:", err)
		return nil, err
	}

	info := MemInfo{
		MemTotal:          0,
		MemFree:           0,
		MemAvailable:      0,
		Buffers:           0,
		Cached:            0,
		Active:            0,
		Inactive:          0,
		Unevictable:       0,
		Slab:              0,
		SReclaimable:      0,
		SUnreclaim:        0,
		KernelStack:       0,
		PageTables:        0,
		Bounce:            0,
		VmallocUsed:       0,
		HardwareCorrupted: 0,
		HugePages_Total:   0,
		Hugepagesize:      0,
	}

	t := reflect.TypeOf(info)
	v := reflect.ValueOf(&info).Elem()
	for k := 0; k < t.NumField(); k++ {
		fieldvalue := v.Field(k)
		ret, err := findNum(str_meminfo, t.Field(k).Name)
		if err != nil {
			log.Printf("find key "+t.Field(k).Name+" err : ", err)
		}
		i, err := strconv.ParseInt(ret, 10, 64)
		fieldvalue.SetInt(i)
	}

	if info.VmallocUsed == 0 {
		numVmallocUsed, _ := strconv.ParseInt(ret_vmalloc, 10, 64)
		info.VmallocUsed = numVmallocUsed
	}

	tempUsed := info.MemTotal - info.MemFree
	tempKernelUsed := info.Slab + info.VmallocUsed + info.PageTables + info.KernelStack + info.HardwareCorrupted + info.Bounce
	tempUserUserd := info.Active + info.Inactive + info.Unevictable
	tempBlackHole := tempUsed - tempUserUserd - tempKernelUsed
	if tempBlackHole < 0 {
		tempBlackHole = 0
	}

	// man free  Used memory (calculated as total - free - buffers - cache)
	ProcessMem := info.MemTotal - info.MemFree - info.Buffers - info.Cached

	memUserdPercent := (1 - float64(info.MemAvailable)/float64(info.MemTotal)) * 100
	// fmt.Println("memUserdPercent:", memUserdPercent)
	// fmt.Printf("Memory Usage: %.2f%%\n", memUserdPercent)
	tempmemUserdPercent := fmt.Sprintf("%.2f%%", memUserdPercent)

	usage := MemUsage{
		Total:        formatMemSize(info.MemTotal),
		ProcessUserd: formatMemSize(ProcessMem),
		Free:         formatMemSize(info.MemFree),
		Available:    formatMemSize(info.MemAvailable),
		Buffers:      formatMemSize(info.Buffers),
		Cached:       formatMemSize(info.Cached),
		Used:         formatMemSize(tempUsed),
		KernelUsed:   formatMemSize(tempKernelUsed),
		Slab:         formatMemSize(info.Slab),
		SUnreclaim:   formatMemSize(info.SUnreclaim),
		VmallocUsed:  formatMemSize(info.VmallocUsed),
		BlackHole:    formatMemSize(tempBlackHole),
		UserdPercent: tempmemUserdPercent,
	}

	var tempStatus string
	var tempDescribe string
	if memUserdPercent > float64(config.MemUsagePercent) {
		tempStatus = "warning"
		tempDescribe = config.MemUsageHighDesc
	} else {
		tempStatus = "info"
		tempDescribe = "正常"
	}

	// 进程级下钻：TUI 始终采集；命令行报告仅阈值触发时采集
	var topProcs []ProcMemUsage
	var procSummary ProcMemSummary
	if (tempStatus != "info" || c.AlwaysTopProcs) && !c.SkipTopProcs {
		procs, summary, perr := (&ProcMemChecker{}).GetTopN(config.MemTopProcN, info.MemTotal, tempUsed)
		if perr != nil {
			log.Printf("获取内存占用高的进程失败: %v", perr)
		} else {
			topProcs = procs
			procSummary = summary
		}
	}

	// 初始化 MemDetail 结构体
	detail := &MemDetail{
		MemUsage:    usage,
		IP:          "192.168.1.100",
		Status:      tempStatus,
		Desc:        tempDescribe,
		TopMemProcs: topProcs,
		ProcSummary: procSummary,
	}
	return detail, nil

}

func (c *MemChecker) PrintDetail(detail MemDetail) []string {
	format := "2006-01-02 15:04:05"
	return []string{
		time.Now().Format(format),
		detail.MemUsage.Total,
		detail.MemUsage.Free,
		detail.MemUsage.Available,
		detail.MemUsage.Buffers,
		detail.MemUsage.Cached,
		detail.MemUsage.Used,
		detail.MemUsage.ProcessUserd,
		detail.MemUsage.KernelUsed,
		detail.MemUsage.Slab,
		detail.MemUsage.SUnreclaim,
		detail.MemUsage.VmallocUsed,
		detail.MemUsage.BlackHole,
		detail.MemUsage.UserdPercent,
		detail.Status,
		detail.Desc,
	}
}
