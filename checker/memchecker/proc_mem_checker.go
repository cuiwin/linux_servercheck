package memchecker

import (
	"bufio"
	"fmt"
	"os/exec"
	"sort"
	"strconv"
	"strings"

	"github.com/cuiwin/linux_server_check/config"
	"github.com/cuiwin/linux_server_check/utils"
)

// ProcMemUsage 单个进程的内存占用情况。
// 使用 RSS（常驻内存）而不是 VSZ，VSZ 是虚拟地址空间，对 JVM 等进程没有参考价值。
type ProcMemUsage struct {
	PID        int     // 进程号
	Name       string  // 进程命令行（comm 会被内核截断到 15 字节，无法区分同类进程）
	RSSBytes   uint64  // 常驻内存
	RSS        string  // 格式化后的可读值
	MemPercent float64 // RSS 占物理内存总量的百分比
}

// ProcMemSummary 进程级内存归因结论。
// 只列 TOP N 不够用：若 TOP N 合计占比很低，说明内存主要被页缓存、Slab 或内核占用。
type ProcMemSummary struct {
	TopRSSBytes uint64  // TOP N 进程 RSS 合计
	TopRSS      string  // 格式化后的可读值
	Ratio       float64 // 占已用内存的比例（0~1）
	Desc        string  // 归因描述
}

// ProcMemChecker 是内存检查的二级 checker：一级发现总量异常后，再下钻到进程维度
type ProcMemChecker struct {
}

// GetTopN 采集内存占用最高的前 N 个进程，并给出归因结论。
// totalKB 为物理内存总量，usedKB 为已用内存，均取自 /proc/meminfo，保证两级口径一致。
func (c *ProcMemChecker) GetTopN(n int, totalKB int64, usedKB int64) ([]ProcMemUsage, ProcMemSummary, error) {
	list, err := topMemProcesses(totalKB)
	if err != nil {
		return nil, ProcMemSummary{}, err
	}
	if n > 0 && len(list) > n {
		list = list[:n]
	}

	summary := buildProcMemSummary(list, usedKB)
	return list, summary, nil
}

// topMemProcesses 采集当前内存占用最高的进程列表。
// 参考 ps -eo pid,rss,args 的思路，结构化输出后在程序内排序，
// 避免依赖 ps 的 --sort 参数（部分精简发行版的 ps 不支持）。
func topMemProcesses(totalKB int64) ([]ProcMemUsage, error) {
	cmd := exec.Command("sh", "-c", "ps -eo pid,rss,args")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, err
	}

	var list []ProcMemUsage
	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	// ps 输出可能带表头，也可能因为参数差异没有表头，这里按解析失败跳过即可
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		fields := strings.Fields(strings.TrimSpace(scanner.Text()))
		if len(fields) < 3 {
			continue
		}
		pid, err := strconv.Atoi(fields[0])
		if err != nil {
			continue
		}
		rssKB, err := strconv.ParseInt(fields[1], 10, 64)
		if err != nil || rssKB <= 0 {
			// rss 为 0 的是内核线程，没有排查价值
			continue
		}

		rssBytes := uint64(rssKB) * 1024
		item := ProcMemUsage{
			PID:      pid,
			Name:     strings.Join(fields[2:], " "),
			RSSBytes: rssBytes,
			RSS:      utils.FormatSize(rssBytes),
		}
		if totalKB > 0 {
			item.MemPercent = float64(rssKB) / float64(totalKB) * 100
		}
		list = append(list, item)
	}

	sort.Slice(list, func(i, j int) bool {
		if list[i].RSSBytes != list[j].RSSBytes {
			return list[i].RSSBytes > list[j].RSSBytes
		}
		return list[i].PID < list[j].PID
	})
	return list, nil
}

// buildProcMemSummary 计算 TOP 进程合计占已用内存的比例，并给出归因描述
func buildProcMemSummary(list []ProcMemUsage, usedKB int64) ProcMemSummary {
	var sum uint64
	for _, item := range list {
		sum += item.RSSBytes
	}

	summary := ProcMemSummary{
		TopRSSBytes: sum,
		TopRSS:      utils.FormatSize(sum),
	}
	if usedKB > 0 {
		summary.Ratio = float64(sum) / (float64(usedKB) * 1024)
	}

	switch {
	case len(list) == 0:
		summary.Desc = "未能获取进程内存占用信息"
	case summary.Ratio < config.MemTopProcRatio:
		summary.Desc = fmt.Sprintf("TOP%d 进程合计 %s，仅占已用内存 %.1f%%，主因可能在内核态（Slab/SUnreclaim）或页缓存",
			len(list), summary.TopRSS, summary.Ratio*100)
	default:
		summary.Desc = fmt.Sprintf("TOP%d 进程合计 %s，占已用内存 %.1f%%，内存主要被用户进程占用",
			len(list), summary.TopRSS, summary.Ratio*100)
	}
	return summary
}
