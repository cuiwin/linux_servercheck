package loadchecker

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cuiwin/linux_server_check/config"
	"github.com/shirou/gopsutil/v4/load"
)

type CpuLoadDetail struct {
	Load     *load.AvgStat
	CpuUsage CpuUsage
	Task     Tasks
	Ctxt     CtxtSwitchStat
	IP       string
	Core     int
	Status   string
	Desc     string
	Trend    string // 负载趋势: rising / falling / stable

	// 分维度状态与描述，便于与 Load / CPU / Task 数据对照展示
	LoadStatus string
	LoadDesc   string
	CpuStatus  string
	CpuDesc    string
	TaskStatus string
	TaskDesc   string

	// CPU 阈值触发时，占用 CPU 最高的前 5 个进程，用于直接定位热点
	TopCPUProcs []ProcCpuUsage
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

// ProcCpuUsage 单个进程的 CPU 占用情况
type ProcCpuUsage struct {
	PID        int     // 进程号
	Name       string  // 进程名
	CPUPercent float64 // CPU 占用百分比
}

// ProcCtxtSwitch 单个进程的上下文切换增量
type ProcCtxtSwitch struct {
	PID          int
	Name         string
	Threads      int
	Voluntary    uint64 // 主动切换（sleep、等 IO、等锁）
	NonVoluntary uint64 // 被动切换（被调度器抢占）
	Total        uint64
}

// CtxtSwitchStat 上下文切换统计。
// /proc/stat 的 ctxt 与 /proc/<pid>/task/<tid>/status 的两个字段都是累计值，
// 必须两次采样做差才能得到本间隔的增量与速率。
type CtxtSwitchStat struct {
	SystemTotal  uint64  // 开机至今的系统切换累计值
	Total        uint64  // 本间隔内的切换增量
	Voluntary    uint64  // 本间隔内全机主动切换增量
	NonVoluntary uint64  // 本间隔内全机被动切换增量
	Rate         float64 // 系统切换速率（次/秒）
	Interval     float64 // 采样间隔（秒）
	Valid        bool    // 是否有基线；首帧为 false，速率不可用

	VoluntaryRate     float64 // 主动切换速率（次/秒）
	NonVoluntaryRate  float64 // 被动切换速率（次/秒）
	NonVoluntaryRatio float64 // 被动切换占比（0~1），越高说明 CPU 争抢越明显

	TopProcesses []ProcCtxtSwitch // 进程自身切换最高的 5 个
	TopThreaded  []ProcCtxtSwitch // 含所有线程累计切换最高的 5 个
}

// procCtxtSnap 单个进程的累计值快照。
// volLeader/nonLeader 为进程主线程（tid==pid）自身的计数，
// volSum/nonSum 为该进程所有线程之和，与 pidstat -wt 口径一致。
type procCtxtSnap struct {
	name      string
	threads   int
	volLeader uint64
	nonLeader uint64
	volSum    uint64
	nonSum    uint64
}

// ctxtSnapshot 一次采样的全量快照，用于与上一帧做差
type ctxtSnapshot struct {
	at       time.Time
	sysTotal uint64
	procs    map[int]*procCtxtSnap
}

// taskStatusInfo 从 status 文件中解析出的关注字段
type taskStatusInfo struct {
	state  byte
	name   string
	vol    uint64
	nonVol uint64
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

// 解析top命令输出的CPU使用率行
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

// topCPUProcesses 采集当前 CPU 占用最高的前 5 个进程。
// 参考 ps -efH | sort -k3 -nr 的思路，改为结构化输出后在程序内排序，
// 避免依赖 ps 的 --sort 参数（部分精简发行版的 ps 不支持）。
func topCPUProcesses() []ProcCpuUsage {
	cmd := exec.Command("sh", "-c", "ps -eo pid,pcpu,comm")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil
	}

	var list []ProcCpuUsage
	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	first := true
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if first {
			// 跳过表头 PID %CPU COMMAND
			first = false
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		pid, err := strconv.Atoi(fields[0])
		if err != nil {
			continue
		}
		pcpu, err := strconv.ParseFloat(fields[1], 64)
		if err != nil {
			continue
		}
		list = append(list, ProcCpuUsage{
			PID:        pid,
			Name:       strings.Join(fields[2:], " "),
			CPUPercent: pcpu,
		})
	}

	sort.Slice(list, func(i, j int) bool {
		if list[i].CPUPercent != list[j].CPUPercent {
			return list[i].CPUPercent > list[j].CPUPercent
		}
		return list[i].PID < list[j].PID
	})
	if len(list) > 5 {
		list = list[:5]
	}
	return list
}

// readSysCtxtTotal 读取 /proc/stat 中的 ctxt，即系统开机至今的上下文切换累计次数
func readSysCtxtTotal() (uint64, error) {
	file, err := os.Open("/proc/stat")
	if err != nil {
		return 0, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "ctxt ") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) != 2 {
			return 0, fmt.Errorf("解析 /proc/stat ctxt 失败: %q", line)
		}
		return strconv.ParseUint(fields[1], 10, 64)
	}
	return 0, fmt.Errorf("/proc/stat 中未找到 ctxt 字段")
}

// readTaskStatus 解析单个 status 文件，取出状态、名称与上下文切换累计值
func readTaskStatus(statusFilePath string) (taskStatusInfo, bool) {
	var info taskStatusInfo
	file, err := os.Open(statusFilePath)
	if err != nil {
		return info, false
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "Name:") {
			info.name = strings.TrimSpace(line[len("Name:"):])
			continue
		}
		// State 行带括号说明（如 "S (sleeping)"），不能按两列解析
		if strings.HasPrefix(line, "State:") {
			state := strings.TrimSpace(line[len("State:"):])
			if len(state) > 0 {
				info.state = state[0]
			}
			continue
		}
		fields := strings.Fields(line)
		if len(fields) != 2 {
			continue
		}
		switch fields[0] {
		case "voluntary_ctxt_switches:":
			info.vol, _ = strconv.ParseUint(fields[1], 10, 64)
		case "nonvoluntary_ctxt_switches:":
			info.nonVol, _ = strconv.ParseUint(fields[1], 10, 64)
		}
	}
	return info, true
}

// countTaskState 按状态首字母累加任务计数
func countTaskState(stats *Tasks, state byte) {
	switch state {
	case 'R':
		stats.Running++
	case 'D':
		stats.Blocked++
	case 'Z':
		stats.Zombie++
	}
}

// readComm 主线程 status 读取失败时用 comm 补全进程名
func readComm(pid int) string {
	data, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(pid), "comm"))
	if err != nil {
		return strconv.Itoa(pid)
	}
	return strings.TrimSpace(string(data))
}

// scanTasks 一次遍历 /proc，同时产出任务状态统计与上下文切换快照。
// 主线程（tid==pid）的切换计数直接复用该 task 的 status，无需额外读 /proc/<pid>/status；
// 进程维度的切换数则是该进程下所有线程之和，与 pidstat -wt 口径一致。
func scanTasks() (Tasks, *ctxtSnapshot, error) {
	var stats Tasks

	re := regexp.MustCompile(`^[0-9]+$`)

	sysTotal, err := readSysCtxtTotal()
	if err != nil {
		return stats, nil, err
	}
	snap := &ctxtSnapshot{
		at:       time.Now(),
		sysTotal: sysTotal,
		procs:    make(map[int]*procCtxtSnap),
	}

	procDirEntries, err := os.ReadDir("/proc")
	if err != nil {
		return stats, nil, err
	}

	for _, entry := range procDirEntries {
		if !entry.IsDir() || !re.MatchString(entry.Name()) {
			continue
		}
		pid, err := strconv.Atoi(entry.Name())
		if err != nil {
			continue
		}

		taskDirPath := filepath.Join("/proc", entry.Name(), "task")
		taskDirEntries, err := os.ReadDir(taskDirPath)
		if err != nil {
			// 忽略打开目录失败的情况，可能是进程已经结束
			continue
		}

		ps := &procCtxtSnap{}
		var statuses []taskStatusInfo
		threadCount := 0

		for _, taskEntry := range taskDirEntries {
			if !taskEntry.IsDir() || !re.MatchString(taskEntry.Name()) {
				continue
			}
			threadCount++
			info, ok := readTaskStatus(filepath.Join(taskDirPath, taskEntry.Name(), "status"))
			if !ok {
				continue
			}
			statuses = append(statuses, info)
			ps.volSum += info.vol
			ps.nonSum += info.nonVol
			if taskEntry.Name() == entry.Name() {
				ps.volLeader = info.vol
				ps.nonLeader = info.nonVol
				ps.name = info.name
			}
		}

		if threadCount == 0 {
			continue
		}
		ps.threads = threadCount
		if ps.name == "" {
			ps.name = readComm(pid)
		}
		snap.procs[pid] = ps

		if threadCount == 1 {
			// 单进程（没有线程）
			stats.SinglePro++
			stats.Total++
		} else {
			// 有线程的进程，统计线程
			stats.Threads += threadCount
		}
		for _, info := range statuses {
			countTaskState(&stats, info.state)
		}
	}
	stats.Total = stats.SinglePro + stats.Threads
	return stats, snap, nil
}

var (
	ctxtMu   sync.Mutex
	ctxtPrev *ctxtSnapshot
)

// collectCtxtSwitch 用当前快照与上一帧做差，得到本间隔的切换增量与 TopN。
// 计数器是累计值，首帧没有基线时返回 Valid=false；
// 计数器回退（线程退出后 TID 被复用）时该进程本帧增量记为 0，避免出现巨大负数。
func collectCtxtSwitch(cur *ctxtSnapshot) CtxtSwitchStat {
	var stat CtxtSwitchStat
	if cur == nil {
		return stat
	}

	ctxtMu.Lock()
	defer ctxtMu.Unlock()

	prev := ctxtPrev
	ctxtPrev = cur
	stat.SystemTotal = cur.sysTotal
	if prev == nil {
		return stat
	}

	interval := cur.at.Sub(prev.at).Seconds()
	if interval <= 0 {
		return stat
	}
	stat.Interval = interval
	stat.Valid = true
	stat.Total = subUint64(cur.sysTotal, prev.sysTotal)
	stat.Rate = float64(stat.Total) / interval

	var selfList, threadList []ProcCtxtSwitch
	for pid, cp := range cur.procs {
		pp, ok := prev.procs[pid]
		if !ok {
			// 本间隔内新出现的进程：生命周期完全落在区间内，
			// 其累计值就是本间隔的增量，按 0 基线计算，避免漏掉刚启动的高频切换进程
			pp = &procCtxtSnap{}
		}

		volSelf := subUint64(cp.volLeader, pp.volLeader)
		nonSelf := subUint64(cp.nonLeader, pp.nonLeader)
		volAll := subUint64(cp.volSum, pp.volSum)
		nonAll := subUint64(cp.nonSum, pp.nonSum)

		stat.Voluntary += volAll
		stat.NonVoluntary += nonAll

		if volSelf+nonSelf > 0 {
			selfList = append(selfList, ProcCtxtSwitch{
				PID: pid, Name: cp.name, Threads: cp.threads,
				Voluntary: volSelf, NonVoluntary: nonSelf, Total: volSelf + nonSelf,
			})
		}
		if volAll+nonAll > 0 {
			threadList = append(threadList, ProcCtxtSwitch{
				PID: pid, Name: cp.name, Threads: cp.threads,
				Voluntary: volAll, NonVoluntary: nonAll, Total: volAll + nonAll,
			})
		}
	}

	stat.TopProcesses = topCtxtSwitch(selfList)
	stat.TopThreaded = topCtxtSwitch(threadList)

	stat.VoluntaryRate = float64(stat.Voluntary) / interval
	stat.NonVoluntaryRate = float64(stat.NonVoluntary) / interval
	if total := stat.Voluntary + stat.NonVoluntary; total > 0 {
		stat.NonVoluntaryRatio = float64(stat.NonVoluntary) / float64(total)
	}
	// 注意 Voluntary+NonVoluntary 会略小于 Total：/proc/stat 的 ctxt 还包含
	// 进出 idle 任务的切换，以及本间隔内已退出任务无法回收的计数。
	return stat
}

// subUint64 累计值做差，回退时返回 0
func subUint64(cur, prev uint64) uint64 {
	if cur < prev {
		return 0
	}
	return cur - prev
}

// topCtxtSwitch 按切换总数降序取前 5 个
func topCtxtSwitch(list []ProcCtxtSwitch) []ProcCtxtSwitch {
	sort.Slice(list, func(i, j int) bool {
		if list[i].Total != list[j].Total {
			return list[i].Total > list[j].Total
		}
		return list[i].PID < list[j].PID
	})
	if len(list) > 5 {
		return list[:5]
	}
	return list
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

	tasks, snap, err := scanTasks()
	if err != nil {
		fmt.Printf("Error counting task states: %v\n", err)
		return CpuLoadDetail{}, err
	}

	ctxtStat := collectCtxtSwitch(snap)

	// 换算成每核负载，1/5/15 分钟分别评估，避免核心数不同的机器无法横向比较
	load1PerCore := loadinfo.Load1 / float64(cpuCount)
	load5PerCore := loadinfo.Load5 / float64(cpuCount)
	load15PerCore := loadinfo.Load15 / float64(cpuCount)

	// 1/5/15 分钟任一超阈值即告警，但文案区分“瞬时峰值”和“持续负载”
	loadStatus := "info"
	var loadIssues []string
	high1 := load1PerCore >= config.CpuLoadPerCore
	high5 := load5PerCore >= config.CpuLoadPerCore
	high15 := load15PerCore >= config.CpuLoadPerCore
	switch {
	case high1 && (high5 || high15):
		loadStatus = "warning"
		loadIssues = append(loadIssues, fmt.Sprintf("负载持续偏高(每核1/5/15分钟=%.2f/%.2f/%.2f)", load1PerCore, load5PerCore, load15PerCore))
	case high1:
		loadStatus = "warning"
		loadIssues = append(loadIssues, fmt.Sprintf("瞬时负载偏高(每核1分钟=%.2f，5/15分钟未超阈值)", load1PerCore))
	case high5 || high15:
		loadStatus = "warning"
		loadIssues = append(loadIssues, fmt.Sprintf("中长期负载偏高(每核5/15分钟=%.2f/%.2f)，当前1分钟已回落至%.2f", load5PerCore, load15PerCore, load1PerCore))
	}

	// CPU 使用情况
	cpuStatus := "info"
	var cpuIssues []string

	if ret_cpu_usage.Softirq >= config.CpuSoftirqPercent {
		cpuStatus = "warning"
		cpuIssues = append(cpuIssues, "系统软中断偏高")
	}

	if ret_cpu_usage.Hardirq >= config.CpuHardirqPercent {
		cpuStatus = "warning"
		cpuIssues = append(cpuIssues, "系统硬中断偏高")
	}

	if ret_cpu_usage.Iowait >= config.CpuIowaitPercent {
		cpuStatus = "warning"
		cpuIssues = append(cpuIssues, "IO使用率偏高")
	}

	if ret_cpu_usage.Total >= config.CpuUsagePercent {
		cpuStatus = "warning"
		cpuIssues = append(cpuIssues, "CPU使用率偏高")
	}

	if ret_cpu_usage.User >= config.CpuUserPercent {
		cpuStatus = "warning"
		cpuIssues = append(cpuIssues, fmt.Sprintf("用户态CPU使用率偏高(%.1f%%)", ret_cpu_usage.User))
	}

	if ret_cpu_usage.System >= config.CpuSystemPercent {
		cpuStatus = "warning"
		cpuIssues = append(cpuIssues, fmt.Sprintf("内核态CPU使用率偏高(%.1f%%)", ret_cpu_usage.System))
	}

	// CPU 阈值触发时定位占用最高的进程，便于直接排查热点
	var topProcs []ProcCpuUsage
	if cpuStatus != "info" {
		topProcs = topCPUProcesses()
	}

	// 任务情况：负载偏高时，再看 R/D 状态任务数：R 多说明在抢 CPU，D 多说明卡在 IO/网络
	taskStatus := "info"
	taskDesc := "运行/阻塞任务正常"
	if high1 || high5 || high15 {
		taskDesc = describeLoadSource(tasks, ret_cpu_usage)
		if tasks.Running+tasks.Blocked > 0 &&
			float64(tasks.Blocked)/float64(tasks.Running+tasks.Blocked) >= config.CpuBlockedRatio {
			taskStatus = "warning"
		}
	}

	trend, trendDesc := judgeLoadTrend(load1PerCore, load5PerCore, load15PerCore)

	// 分维度描述
	loadDesc := "负载正常"
	if len(loadIssues) > 0 {
		loadDesc = strings.Join(loadIssues, ";")
	}
	loadDesc += "；趋势" + trendDesc

	cpuDesc := "CPU使用率正常"
	if len(cpuIssues) > 0 {
		cpuDesc = strings.Join(cpuIssues, ";")
	}

	// 汇总描述
	var issues []string
	issues = append(issues, loadIssues...)
	issues = append(issues, cpuIssues...)
	if high1 || high5 || high15 {
		issues = append(issues, taskDesc)
	}

	status := "info"
	if loadStatus != "info" || cpuStatus != "info" || taskStatus != "info" {
		status = "warning"
	}

	desc := "正常"
	if len(issues) > 0 {
		desc = strings.Join(issues, ";")
	}
	desc += "；趋势" + trendDesc

	// 初始化 CpuLoadDetail 结构体
	detail := CpuLoadDetail{
		Load:       loadinfo,
		CpuUsage:   ret_cpu_usage,
		Task:       tasks,
		Ctxt:       ctxtStat,
		IP:         "192.168.1.100",
		Core:       cpuCount,
		Status:     status,
		Desc:       desc,
		Trend:      trend,
		LoadStatus: loadStatus,
		LoadDesc:   loadDesc,
		CpuStatus:  cpuStatus,
		CpuDesc:    cpuDesc,
		TaskStatus: taskStatus,
		TaskDesc:   taskDesc,

		TopCPUProcs: topProcs,
	}
	return detail, nil
}

// judgeLoadTrend 根据每核 1/5/15 分钟负载判断趋势。
// 三个值本质是同一条负载序列在不同时间窗口的指数滑动平均：
// 单调递增说明负载仍在爬升，单调递减说明负载正在回落；
// 同时要求 1 分钟与 15 分钟的差值超过阈值，避免指数平滑造成的伪趋势。
func judgeLoadTrend(load1PerCore, load5PerCore, load15PerCore float64) (string, string) {
	delta := load1PerCore - load15PerCore

	switch {
	case delta >= config.CpuLoadTrendDelta && load1PerCore > load5PerCore && load5PerCore > load15PerCore:
		return "rising", "持续上升"
	case delta <= -config.CpuLoadTrendDelta && load1PerCore < load5PerCore && load5PerCore < load15PerCore:
		return "falling", "持续下降"
	case delta >= config.CpuLoadTrendDelta:
		return "rising", "上升中"
	case delta <= -config.CpuLoadTrendDelta:
		return "falling", "回落中"
	default:
		return "stable", "平稳"
	}
}

// describeLoadSource 在负载偏高时结合 R/D 状态任务数与 IO 等待定位负载来源。
// R（Running）多说明任务在排队抢 CPU；D（Blocked）多说明任务卡在磁盘/网络 IO，
// 这种情况下 load 会升高但 CPU 使用率不高，需要往 IO 方向排查。
func describeLoadSource(tasks Tasks, usage CpuUsage) string {
	running := tasks.Running
	blocked := tasks.Blocked

	if running+blocked == 0 {
		return fmt.Sprintf("R=%d，D=%d，无运行/阻塞任务，负载来自其他原因", running, blocked)
	}

	blockedRatio := float64(blocked) / float64(running+blocked)

	switch {
	case blockedRatio >= config.CpuBlockedRatio && usage.Iowait >= config.CpuIowaitPercent:
		return fmt.Sprintf("R=%d，D=%d（D占%d%%），IO等待%.1f%%，负载主要来自磁盘/网络IO阻塞",
			running, blocked, int(blockedRatio*100), usage.Iowait)
	case blockedRatio >= config.CpuBlockedRatio:
		return fmt.Sprintf("R=%d，D=%d（D占%d%%），负载主要来自IO/网络阻塞，需检查磁盘与网络",
			running, blocked, int(blockedRatio*100))
	default:
		return fmt.Sprintf("R=%d，D=%d（D占%d%%），负载以CPU争抢为主",
			running, blocked, int(blockedRatio*100))
	}
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
		strconv.FormatUint(detail.Ctxt.SystemTotal, 10),
		strconv.FormatUint(detail.Ctxt.Total, 10),
		strconv.FormatFloat(detail.Ctxt.Rate, 'f', 2, 64),
		strconv.FormatUint(detail.Ctxt.Voluntary, 10),
		strconv.FormatUint(detail.Ctxt.NonVoluntary, 10),
		detail.Status,
		detail.Desc,
	}
}

// PrintCtxtTopProcesses 输出“进程自身切换数”最高的 5 个进程，每行：PID、名称、线程数、主动、被动、总计
func (c *LoadChecker) PrintCtxtTopProcesses(detail CpuLoadDetail) [][]string {
	return ctxtTopRows(detail.Ctxt.TopProcesses)
}

// PrintCtxtTopThreaded 输出“含全部线程累计切换数”最高的 5 个进程，每行：PID、名称、线程数、主动、被动、总计
func (c *LoadChecker) PrintCtxtTopThreaded(detail CpuLoadDetail) [][]string {
	return ctxtTopRows(detail.Ctxt.TopThreaded)
}

func ctxtTopRows(list []ProcCtxtSwitch) [][]string {
	rows := make([][]string, 0, len(list))
	for _, item := range list {
		rows = append(rows, []string{
			strconv.Itoa(item.PID),
			item.Name,
			strconv.Itoa(item.Threads),
			strconv.FormatUint(item.Voluntary, 10),
			strconv.FormatUint(item.NonVoluntary, 10),
			strconv.FormatUint(item.Total, 10),
		})
	}
	return rows
}
