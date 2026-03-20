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
)

type MemInfo struct {
	MemTotal          int64
	MemFree           int64
	MemAvailable      int64
	Buffers           int64
	Cached            int64
	Active            int64
	Inactive          int64
	Unevictable       int64
	Slab              int64
	SReclaimable      int64
	SUnreclaim        int64
	KernelStack       int64
	PageTables        int64
	Bounce            int64
	VmallocUsed       int64
	HardwareCorrupted int64
	HugePages_Total   int64
	Hugepagesize      int64
}

type MemUsage struct {
	Total        string
	ProcessUserd string
	Free         string
	Available    string
	Buffers      string
	Cached       string
	Used         string
	KernelUsed   string
	Slab         string
	SUnreclaim   string
	VmallocUsed  string
	BlackHole    string
	UserdPercent string
}

type MemDetail struct {
	MemUsage MemUsage
	IP       string
	Status   string
	Desc     string
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
	// 这里可以添加 Checker 结构体的字段
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

func formatSize(size int64) string {
	const gb = 1024 * 1024
	if size > gb {
		return fmt.Sprintf("%.2fGB", float64(size)/gb)
	} else {
		return fmt.Sprintf("%.2fMB", float64(size)/(1024))
	}
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
		Total:        formatSize(info.MemTotal),
		ProcessUserd: formatSize(ProcessMem),
		Free:         formatSize(info.MemFree),
		Available:    formatSize(info.MemAvailable),
		Buffers:      formatSize(info.Buffers),
		Cached:       formatSize(info.Cached),
		Used:         formatSize(tempUsed),
		KernelUsed:   formatSize(tempKernelUsed),
		Slab:         formatSize(info.Slab),
		SUnreclaim:   formatSize(info.SUnreclaim),
		VmallocUsed:  formatSize(info.VmallocUsed),
		BlackHole:    formatSize(tempBlackHole),
		UserdPercent: tempmemUserdPercent,
	}

	var tempStatus string
	var tempDescribe string
	if memUserdPercent > float64(85) {
		tempStatus = "warning"
		tempDescribe = "超过90%"
	} else {
		tempStatus = "info"
		tempDescribe = "正常"
	}

	// 初始化 MemDetail 结构体
	detail := &MemDetail{
		MemUsage: usage,
		IP:       "192.168.1.100",
		Status:   tempStatus,
		Desc:     tempDescribe,
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
