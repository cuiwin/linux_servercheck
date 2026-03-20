package threadchecker

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// Checker 结构体定义
type ThreadChecker struct {
	// 这里可以添加 Checker 结构体的字段
}

// 线程相关内核参数信息
type KernelParams struct {
	PidMax         int `json:"pid_max"`
	ThreadsMax     int `json:"threads_max"`
	UlimitUser     int `json:"ulimit_user"`
	MaxMapCount    int `json:"max_map_count"`
	CurrentThreads int `json:"current_threads"`
	UsedPercent    string
	Status         string
	Desc           string
}

// 进程线程信息
type ProcessThreads struct {
	Pid     int    `json:"pid"`
	Name    string `json:"name"`
	Threads int    `json:"threads"`
}

// 获取线程相关信息
func (c *ThreadChecker) GetThreadUsageInfo() ([]KernelParams, error) {
	// 获取系统参数
	params := KernelParams{}
	var err error

	params.PidMax, err = getPidMax()
	if err != nil {
		fmt.Printf("获取kernel.pid_max失败: %v\n", err)
	}

	params.ThreadsMax, err = getThreadsMax()
	if err != nil {
		fmt.Printf("获取kernel.threads-max失败: %v\n", err)
	}

	params.UlimitUser, err = getUlimitUser()
	if err != nil {
		fmt.Printf("获取ulimit -u失败: %v\n", err)
	}

	params.MaxMapCount, err = getMaxMapCount()
	if err != nil {
		fmt.Printf("获取vm.max_map_count失败: %v\n", err)
	}

	params.CurrentThreads, err = getCurrentThreadsCount()
	if err != nil {
		fmt.Printf("获取当前线程总数失败: %v\n", err)
	}

	usage := float64(params.CurrentThreads) / float64(params.ThreadsMax)
	if usage > 80 {
		params.Status = "warning"
		params.Desc = " 使用率偏高"
	} else {
		params.Status = "info"
		params.Desc = " 使用率正常"
	}

	params.UsedPercent = strconv.FormatFloat(usage, 'f', 3, 64)
	var KernelParamsList []KernelParams
	KernelParamsList = append(KernelParamsList, params)
	return KernelParamsList, nil
}

// 获取磁盘大小使用率打印
func (c *ThreadChecker) GetThreadUsagePrint(kernelparams []KernelParams) [][]string {

	var items [][]string
	for _, kernel := range kernelparams {

		item := []string{
			strconv.Itoa(kernel.PidMax),
			strconv.Itoa(kernel.ThreadsMax),
			strconv.Itoa(kernel.UlimitUser),
			strconv.Itoa(kernel.MaxMapCount),
			strconv.Itoa(kernel.CurrentThreads),
			kernel.UsedPercent,
			kernel.Status,
			kernel.Desc,
		}
		items = append(items, item)
	}
	return items
}

// 读取/proc/sys/kernel/pid_max
func getPidMax() (int, error) {
	data, err := os.ReadFile("/proc/sys/kernel/pid_max")
	if err != nil {
		return 0, err
	}
	value, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return 0, err
	}
	return value, nil
}

// 读取/proc/sys/kernel/threads-max
func getThreadsMax() (int, error) {
	data, err := os.ReadFile("/proc/sys/kernel/threads-max")
	if err != nil {
		return 0, err
	}
	value, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return 0, err
	}
	return value, nil
}

// 获取ulimit -u的值
func getUlimitUser() (int, error) {
	cmd := exec.Command("sh", "-c", "ulimit -u")
	output, err := cmd.Output()
	if err != nil {
		return 0, err
	}
	value, err := strconv.Atoi(strings.TrimSpace(string(output)))
	if err != nil {
		return 0, err
	}
	return value, nil
}

// 读取/proc/sys/vm/max_map_count
func getMaxMapCount() (int, error) {
	data, err := os.ReadFile("/proc/sys/vm/max_map_count")
	if err != nil {
		return 0, err
	}
	value, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return 0, err
	}
	return value, nil
}

// 获取当前系统线程总数 - 使用/proc/loadavg的第四个数值
func getCurrentThreadsCount() (int, error) {
	// 读取/proc/loadavg文件
	data, err := os.ReadFile("/proc/loadavg")
	if err != nil {
		return 0, err
	}

	// 按空格分割内容
	parts := strings.Fields(string(data))
	if len(parts) < 4 {
		return 0, fmt.Errorf("invalid format in /proc/loadavg")
	}

	// 第四个字段格式为"正在运行的进程数/总线程数"
	threadInfo := parts[3]
	// 分割出总线程数
	threadCountStr := strings.Split(threadInfo, "/")[1]

	// 转换为整数
	return strconv.Atoi(threadCountStr)
}

// 获取每个进程的线程数量
func (c *ThreadChecker) GetProcessThreads() ([]ProcessThreads, error) {
	var processes []ProcessThreads

	// 获取所有进程ID
	pidDirs, err := filepath.Glob("/proc/[0-9]*")
	if err != nil {
		return nil, err
	}

	for _, dir := range pidDirs {
		pidStr := filepath.Base(dir)
		pid, err := strconv.Atoi(pidStr)
		if err != nil {
			continue
		}

		// 获取进程名称
		cmdline, err := os.ReadFile(filepath.Join(dir, "comm"))
		if err != nil {
			continue // 可能进程已退出
		}
		name := strings.TrimSpace(string(cmdline))

		// 获取线程数量
		threadsDir := filepath.Join(dir, "task")
		threadDirs, err := filepath.Glob(filepath.Join(threadsDir, "[0-9]*"))
		if err != nil {
			continue
		}
		threadCount := len(threadDirs)

		processes = append(processes, ProcessThreads{
			Pid:     pid,
			Name:    name,
			Threads: threadCount,
		})
	}

	// 按线程数量降序排序
	sort.Slice(processes, func(i, j int) bool {
		return processes[i].Threads > processes[j].Threads
	})

	// 只返回前10条数据
	if len(processes) > 10 {
		processes = processes[:10]
	}

	return processes, nil
}

// 线程使用排序打印
func (c *ThreadChecker) GetThreadUsageRankPrint(ranks []ProcessThreads) [][]string {

	var items [][]string
	for _, item := range ranks {

		item := []string{
			strconv.Itoa(item.Pid),
			item.Name,
			strconv.Itoa(item.Threads),
		}
		items = append(items, item)
	}
	return items
}
