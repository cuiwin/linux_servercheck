package loadchecker

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/shirou/gopsutil/v4/load"
)

type CpuLoadDetail struct {
	Load     *load.AvgStat
	CpuUsage CpuUsage
	Task     Tasks
	IP       string
	Core     int
	Status   string
	Desc     string
}

type Tasks struct {
	Total     int
	Running   int
	Blocked   int
	Zombie    int
	SinglePro int // 单进程数
	Threads   int // 线程数
}

type CpuUsage struct {
	Total   float64
	User    float64
	System  float64
	Nice    float64
	Idle    float64
	Iowait  float64
	Hardirq float64
	Softirq float64
	Steal   float64
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

// 来自top命令,获取CPU使用率
func getCpuPercent() (CpuUsage, error) {

	var cpu_usage CpuUsage
	// 结果示例： 0.0 us,  0.0 sy,  0.0 ni,100.0 id,  0.0 wa,  0.0 hi,  0.0 si,  0.0 st
	cmd := exec.Command("sh", "-c", "top -bn 1  | grep Cpu | awk -F':' '{print $2}'")
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Fatalf("Error executing top command: %v", err)
		return cpu_usage, err
	}

	// 将输出转换为字符串
	outputStr := string(output)

	cpu_usage = parseCpuLine(outputStr)
	// fmt.Println("cpu_usage", cpu_usage)
	return cpu_usage, nil
}

func parseCpuLine(line string) CpuUsage {
	parts := strings.Split(line, ",")

	usageMap := make(map[string]float64)

	for _, part := range parts {
		part = strings.TrimSpace(part)
		values := strings.Fields(part)
		if len(values) == 2 {
			value, err := strconv.ParseFloat(values[0], 64)
			if err != nil {
				fmt.Printf("Error parsing value: %v\n", err)
			}
			usageMap[values[1]] = value
		}
	}

	var cpuusage CpuUsage
	cpuusage.User = usageMap["us"]
	cpuusage.System = usageMap["sy"]
	cpuusage.Nice = usageMap["ni"]
	cpuusage.Iowait = usageMap["wa"]
	cpuusage.Hardirq = usageMap["hi"]
	cpuusage.Idle = usageMap["id"]
	cpuusage.Steal = usageMap["st"]
	cpuusage.Softirq = usageMap["si"]
	cpuusage.Total = 100 - usageMap["id"]

	return cpuusage
}

// countAllTaskStates 统计所有任务（进程和线程）状态
func countAllTaskStates() (Tasks, error) {
	var stats Tasks

	// 正则表达式匹配 /proc 目录下的数字子目录（进程 ID）
	re := regexp.MustCompile(`^[0-9]+$`)

	// 遍历 /proc 目录
	procDirEntries, err := os.ReadDir("/proc")
	if err != nil {
		return stats, err
	}

	for _, entry := range procDirEntries {
		if entry.IsDir() && re.MatchString(entry.Name()) {
			pid := entry.Name()
			taskDirPath := filepath.Join("/proc", pid, "task")
			taskDirEntries, err := os.ReadDir(taskDirPath)
			if err != nil {
				// 忽略打开目录失败的情况，可能是进程已经结束
				continue
			}
			if len(taskDirEntries) == 1 {
				// 单进程（没有线程）
				statusFilePath := filepath.Join("/proc", pid, "status")
				processStatusFile(statusFilePath, &stats, false)
				stats.SinglePro++
			} else {
				// 有线程的进程，统计线程
				for _, taskEntry := range taskDirEntries {
					if taskEntry.IsDir() && re.MatchString(taskEntry.Name()) {
						statusFilePath := filepath.Join(taskDirPath, taskEntry.Name(), "status")
						processStatusFile(statusFilePath, &stats, true)
						stats.Threads++
					}
				}
			}
		}
	}
	stats.Total = stats.SinglePro + stats.Threads
	return stats, nil
}

// processStatusFile 处理单个 status 文件，更新状态计数
func processStatusFile(statusFilePath string, stats *Tasks, isThread bool) {
	file, err := os.Open(statusFilePath)
	if err != nil {
		// 忽略打开文件失败的情况，可能是任务已经结束
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if len(line) >= 6 && line[:6] == "State:" {
			// 提取状态信息
			state := line[7:]
			// 去除多余的空格
			for len(state) > 0 && state[0] == ' ' {
				state = state[1:]
			}
			// 只取状态的首字母
			firstChar := state[:1]
			if !isThread {
				stats.Total++
			}
			switch firstChar {
			case "R":
				stats.Running++
			case "D":
				stats.Blocked++
			case "Z":
				stats.Zombie++
			}
			break
		}
	}
}

// Checker 结构体定义
type LoadChecker struct {
	// 这里可以添加 Checker 结构体的字段
}

// 获取CpuLoadDetail
func (c *LoadChecker) GetDetail() (CpuLoadDetail, error) {

	// var detail CpuLoadDetail
	var err error

	cpuCount, err := getCoreNum()
	if err != nil {
		fmt.Println("获取CPU核心数量失败:", err)
		return CpuLoadDetail{}, err
	}

	loadinfo, err := load.Avg()
	if err != nil {
		fmt.Println("获取Load.Avg()失败:", err)
		return CpuLoadDetail{}, err
	}

	ret_cpu_usage, err := getCpuPercent()
	if err != nil {
		fmt.Println("获取CPU使用率失败:", err)
		return CpuLoadDetail{}, err
	}

	tasks, err := countAllTaskStates()
	if err != nil {
		fmt.Printf("Error counting task states: %v\n", err)
		return CpuLoadDetail{}, err
	}

	var status string
	var desc string
	if loadinfo.Load1 >= 1.2*float64(cpuCount) {
		status = "warning"
		desc = "负载偏高"
	} else {
		status = "info"
		desc = "正常"
	}

	if ret_cpu_usage.Softirq >= 10 {
		status = "warning"
		desc += ";系统软中断偏高"
	}

	if ret_cpu_usage.Iowait >= 10 {
		status = "warning"
		desc += ";IO使用率偏高"
	}

	if ret_cpu_usage.Total >= 85 {
		status = "warning"
		desc += ";CPU使用率偏高"
	}

	// 初始化 CpuLoadDetail 结构体
	detail := CpuLoadDetail{
		Load:     loadinfo,
		CpuUsage: ret_cpu_usage,
		Task:     tasks,
		IP:       "192.168.1.100",
		Core:     cpuCount,
		Status:   status,
		Desc:     desc,
	}
	return detail, nil
}

func (c *LoadChecker) PrintDetail(detail CpuLoadDetail) []string {
	format := "2006-01-02 15:04:05"
	return []string{
		time.Now().Format(format),
		strconv.Itoa(detail.Task.Total),
		strconv.Itoa(detail.Task.Running),
		strconv.Itoa(detail.Task.Blocked),
		strconv.Itoa(detail.Task.Zombie),
		strconv.Itoa(detail.Core),
		strconv.FormatFloat(detail.Load.Load1, 'f', 2, 64),
		strconv.FormatFloat(detail.Load.Load5, 'f', 2, 64),
		strconv.FormatFloat(detail.Load.Load15, 'f', 2, 64),
		strconv.FormatFloat(detail.CpuUsage.Total, 'f', 2, 64),
		strconv.FormatFloat(detail.CpuUsage.User, 'f', 2, 64),
		strconv.FormatFloat(detail.CpuUsage.System, 'f', 2, 64),
		strconv.FormatFloat(detail.CpuUsage.Nice, 'f', 2, 64),
		strconv.FormatFloat(detail.CpuUsage.Idle, 'f', 2, 64),
		strconv.FormatFloat(detail.CpuUsage.Iowait, 'f', 2, 64),
		strconv.FormatFloat(detail.CpuUsage.Hardirq, 'f', 2, 64),
		strconv.FormatFloat(detail.CpuUsage.Softirq, 'f', 2, 64),
		strconv.FormatFloat(detail.CpuUsage.Steal, 'f', 2, 64),
		detail.Status,
		detail.Desc,
	}
}
