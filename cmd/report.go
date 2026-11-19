/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"sync"
	"text/template"
	"time"

	"github.com/cuiwin/linux_server_check/checker/diskchecker"
	"github.com/cuiwin/linux_server_check/checker/hostchecker"
	"github.com/cuiwin/linux_server_check/checker/kernelchecker"
	"github.com/cuiwin/linux_server_check/checker/loadchecker"
	"github.com/cuiwin/linux_server_check/checker/memchecker"
	"github.com/cuiwin/linux_server_check/checker/netchecker"

	"github.com/spf13/cobra"
)

//go:embed templates/*.html
var templateFS embed.FS

type SummaryInfo struct {
	SectionId string
	Message   string
}

// 报告数据结构
type ReportData struct {
	HostInfo                  hostchecker.HostInfo               `json:"HostInfo"`
	Timestamp                 string                             `json:"timestamp"`
	DiskUsageList             []diskchecker.DiskUsage            `json:"diskUsageList"`
	DiskIOStatsList           []diskchecker.DiskIOStats          `json:"diskIOStatsList"`
	CpuLoadDetailList         []loadchecker.CpuLoadDetail        `json:"cpuLoadDetailList"`
	CtxtSummary               loadchecker.CtxtSwitchStat         `json:"ctxtSummary"`
	MemDetailList             []*memchecker.MemDetail            `json:"memDetailList"`
	KernelParamList           []kernelchecker.KernelParam        `json:"kernelParamList"`
	KernelComparisonSummary   kernelchecker.ComparisonSummary    `json:"kernelComparisonSummary"`
	NetworkInterfaceList      []netchecker.NetworkInterface      `json:"networkInterfaceList"`
	NetworkInterfaceStatsList []netchecker.NetworkInterfaceStats `json:"networkInterfaceStatsList"`
	NormalSummaryList         []SummaryInfo                      `json:"normalSummaryList"`
	WarningSummaryList        []SummaryInfo                      `json:"warningSummaryList"`
	ErrorSummaryList          []SummaryInfo                      `json:"errorSummaryList"`
}

// 自定义函数：格式化
func FormatFloat(value float64) string {
	return strconv.FormatFloat(value, 'f', 2, 64)
}

// 报告数据分析汇总
func analyzeReportData(report ReportData) ReportData {

	// -------  load begin  --------
	load_totalCount := len(report.CpuLoadDetailList)
	load_normalCount := 0
	for _, detail := range report.CpuLoadDetailList {

		// 检查状态
		if detail.Status == "info" {
			load_normalCount++
		}
	}
	// 应用规则
	switch load_normalCount {
	case load_totalCount:
		report.NormalSummaryList = append(report.NormalSummaryList, SummaryInfo{"load-info", "系统负载和CPU使用率正常"})
	case 0:
		report.ErrorSummaryList = append(report.ErrorSummaryList, SummaryInfo{"load-info", "系统负载或CPU使用率异常"})
	default:
		report.WarningSummaryList = append(report.WarningSummaryList, SummaryInfo{"load-info", "系统负载或CPU使用率有异常现象"})
	}

	// -------  load end  --------

	// -------  mem begin  --------
	mem_totalCount := len(report.MemDetailList)
	mem_normalCount := 0
	for _, detail := range report.MemDetailList {

		// 检查状态
		if detail.Status == "info" {
			mem_normalCount++
		}
	}
	// 应用规则
	switch mem_normalCount {
	case mem_totalCount:
		report.NormalSummaryList = append(report.NormalSummaryList, SummaryInfo{"memory-info", "系统内存使用率正常"})
	case 0:
		report.ErrorSummaryList = append(report.ErrorSummaryList, SummaryInfo{"memory-info", "系统内存使用率异常"})
	default:
		report.WarningSummaryList = append(report.WarningSummaryList, SummaryInfo{"memory-info", "系统内存使用率有异常记录"})
	}

	// -------  mem end  --------

	// -------  disk begin  --------
	disk_totalCount := len(report.DiskUsageList)
	disk_normalCount := 0
	for _, detail := range report.DiskUsageList {

		// 检查状态
		if detail.Status == "info" {
			disk_normalCount++
		}
	}
	// 应用规则
	switch disk_normalCount {
	case disk_totalCount:
		report.NormalSummaryList = append(report.NormalSummaryList, SummaryInfo{"disk-info", "磁盘空间使用率正常"})
	case 0:
		report.ErrorSummaryList = append(report.ErrorSummaryList, SummaryInfo{"disk-info", "磁盘空间使用率异常"})
	default:
		report.WarningSummaryList = append(report.WarningSummaryList, SummaryInfo{"disk-info", "磁盘空间使用率有异常记录"})
	}

	// -------  disk end  --------

	// -------  diskio begin  --------
	diskio_totalCount := len(report.DiskIOStatsList)
	diskio_normalCount := 0
	for _, detail := range report.DiskIOStatsList {

		// 检查状态
		if detail.Status == "info" {
			diskio_normalCount++
		}
	}
	// 应用规则
	switch diskio_normalCount {
	case diskio_totalCount:
		report.NormalSummaryList = append(report.NormalSummaryList, SummaryInfo{"diskio-info", "磁盘IO使用率正常"})
	case 0:
		report.ErrorSummaryList = append(report.ErrorSummaryList, SummaryInfo{"diskio-info", "磁盘IO使用率异常"})
	default:
		report.WarningSummaryList = append(report.WarningSummaryList, SummaryInfo{"diskio-info", "磁盘IO使用率有异常记录"})
	}

	// -------  diskio end  --------

	// -------  netio begin  --------
	netio_totalCount := len(report.NetworkInterfaceStatsList)
	netio_normalCount := 0
	for _, detail := range report.NetworkInterfaceStatsList {

		// 检查状态
		if detail.Status == "info" {
			netio_normalCount++
		}
	}
	// 应用规则
	switch netio_normalCount {
	case netio_totalCount:
		report.NormalSummaryList = append(report.NormalSummaryList, SummaryInfo{"netio-info", "网卡流量正常"})
	case 0:
		report.ErrorSummaryList = append(report.ErrorSummaryList, SummaryInfo{"netio-info", "网卡流量异常"})
	default:
		report.WarningSummaryList = append(report.WarningSummaryList, SummaryInfo{"netio-info", "网卡流量有异常记录"})
	}
	// -------  netio end  --------

	// -------  kernel para begin  --------

	if report.KernelComparisonSummary.MatchedPercent < 60.0 {
		report.WarningSummaryList = append(report.WarningSummaryList, SummaryInfo{"kernel-info", "内核优化参数待优化"})
	}

	// -------  kernel para end  --------

	return report
}

var checkCount int

// reportCmd represents the report command
var reportCmd = &cobra.Command{
	Use:   "report",
	Short: "服务器检查报告生成",
	Long:  `服务器检查报告生成`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("开始检查分析主机")

		// 统一时间戳，报告内容和文件名共用
		now := time.Now()
		timestamp := now.Format("2006-01-02 15:04:05")
		timestampFile := now.Format("20060102150405")

		reportData := ReportData{
			Timestamp: timestamp,
		}

		// 各检查项之间相互独立，使用 goroutine 并发执行；
		// 每个 goroutine 只写 reportData 的不同字段，互不干扰
		var wg sync.WaitGroup

		// 并发场景下统一输出错误，避免多 goroutine 同时写终端导致内容交错
		var printMu sync.Mutex
		printErr := func(format string, args ...interface{}) {
			printMu.Lock()
			defer printMu.Unlock()
			cmd.PrintErrf(format, args...)
		}

		var hostErr error

		// -------  主机信息 begin  --------
		fmt.Println("开始检查主机 - 【主机信息】")
		wg.Add(1)
		go func() {
			defer wg.Done()
			hostcheckerInstance := hostchecker.HostChecker{}
			hostInfo, err := hostcheckerInstance.GetHostInfo()
			if err != nil {
				hostErr = err
				printErr("获取主机信息失败: %v\n", err)
				return
			}
			reportData.HostInfo = hostInfo
		}()

		// -------  主机信息 end  --------

		// -------  磁盘信息 begin  --------
		fmt.Println("开始检查主机 - 【磁盘信息】")
		wg.Add(1)
		go func() {
			defer wg.Done()
			diskcheckerInstance := diskchecker.DiskChecker{}
			diskUsageList, err := diskcheckerInstance.GetDiskUsageInfo()
			if err != nil {
				printErr("获取磁盘使用信息失败: %v\n", err)
			}
			reportData.DiskUsageList = diskUsageList
		}()

		// -------  磁盘信息 end  --------

		// -------  负载信息 begin  --------
		fmt.Println("开始检查主机 - 【负载信息】")
		wg.Add(1)
		go func() {
			defer wg.Done()

			// 采样必须串行：ctxt 差值依赖上一帧快照，且相邻采样需要 1 秒间隔
			var results []loadchecker.CpuLoadDetail
			loadcheckerInstance := loadchecker.LoadChecker{}

			// 循环调用函数 5 次，并将结果保存到数组中
			for i := 0; i < checkCount; i++ {

				info, err := loadcheckerInstance.GetDetail()
				if err != nil {
					printErr("第%d次获取CPU负载失败: %v\n", i+1, err)
					continue
				}
				results = append(results, info)

				// 上下文切换是累计值，需要相邻两次采样做差才有速率，因此留出 1 秒采样间隔
				if i < checkCount-1 {
					time.Sleep(time.Second)
				}
			}
			reportData.CpuLoadDetailList = results
			if len(results) > 0 {
				reportData.CtxtSummary = results[len(results)-1].Ctxt
			}
		}()

		// -------  负载信息 end  --------

		// -------  内存信息 begin  --------
		fmt.Println("开始检查主机 - 【内存信息】")
		wg.Add(1)
		go func() {
			defer wg.Done()
			var memResults []*memchecker.MemDetail
			// 循环调用函数 5 次，并将结果保存到数组中
			for i := 0; i < checkCount; i++ {

				// 进程级下钻只在最后一次采样时执行，避免连续多次执行 ps
				memcheckerInstance := memchecker.MemChecker{SkipTopProcs: i < checkCount-1}
				info, err := memcheckerInstance.GetDetail()
				if err != nil {
					printErr("第%d次获取内存信息失败: %v\n", i+1, err)
					continue
				}
				memResults = append(memResults, info)
			}
			reportData.MemDetailList = memResults
		}()

		// -------  内存信息 end  --------

		// -------  磁盘IO信息 begin  --------
		fmt.Println("开始检查主机 - 【磁盘IO】")
		wg.Add(1)
		go func() {
			defer wg.Done()

			var diskIOResults []diskchecker.DiskIOStats
			diskiocheckerInstance := diskchecker.DiskIOChecker{}

			// 先获取iostat版本
			version, err := diskiocheckerInstance.GetIostatVersion()
			if err != nil {
				printErr("获取iostat版本失败: %v\n", err)
			}
			fmt.Printf("获取iostat命令版本: %v\n", version)

			// 循环调用函数 5 次，并将结果保存到数组中
			for i := 0; i < checkCount; i++ {
				info, err := diskiocheckerInstance.GetDiskStats(version)
				if err != nil {
					printErr("第%d次获取磁盘IO信息失败: %v\n", i+1, err)
					continue
				}
				diskIOResults = append(diskIOResults, info...)
			}

			// 排序
			sort.Slice(diskIOResults, func(i, j int) bool {
				return diskIOResults[i].Device < diskIOResults[j].Device
			})

			reportData.DiskIOStatsList = diskIOResults
		}()

		// -------  磁盘IO信息  end   --------

		// -------  内核参数信息 begin  --------
		fmt.Println("开始检查主机 - 【内核参数】")
		wg.Add(1)
		go func() {
			defer wg.Done()
			kernelcheckerInstance := kernelchecker.KernelChecker{}

			list, summary, err := kernelcheckerInstance.GetKernelParamList()
			if err != nil {
				printErr("获取内核参数列表失败: %v\n", err)
			}
			reportData.KernelParamList = list
			reportData.KernelComparisonSummary = summary
		}()

		// -------  内核参数信息 end  --------

		// -------  网络IO begin  --------
		fmt.Println("开始检查主机 - 【网络信息】")
		wg.Add(1)
		go func() {
			defer wg.Done()

			netcheckerInstance := netchecker.NetChecker{}

			ifaceList, err := netcheckerInstance.GetNetworkInterfaceList()
			if err != nil {
				printErr("获取网络接口列表失败: %v\n", err)
			}
			reportData.NetworkInterfaceList = ifaceList

			var netIOResults []netchecker.NetworkInterfaceStats
			// 循环调用函数 5 次，并将结果保存到数组中
			for i := 0; i < checkCount; i++ {
				info, err := netcheckerInstance.GetNetworkStatsList()
				if err != nil {
					printErr("第%d次获取网络流量信息失败: %v\n", i+1, err)
					continue
				}
				netIOResults = append(netIOResults, info...)
			}

			// 根据网卡名称排序
			sort.Slice(netIOResults, func(i, j int) bool {
				return netIOResults[i].Interface < netIOResults[j].Interface
			})

			reportData.NetworkInterfaceStatsList = netIOResults
		}()

		// -------  网络信息IO end  --------

		// 等待所有检查项完成
		wg.Wait()

		// 主机信息是报告内容和文件名的基础，获取失败则不生成报告
		if hostErr != nil {
			return
		}

		// -------  信息汇总 begin  --------
		fmt.Println("开始检查主机 - 【信息汇总】")
		newReportData := analyzeReportData(reportData)
		// -------  信息汇总 end  --------

		// 创建 Template 对象并注册函数
		tmpl := template.New("server_inspection.html").Funcs(template.FuncMap{
			"FormatFloat": FormatFloat, // 自定义时间格式化函数
			"Percent": func(value float64) string {
				return strconv.FormatFloat(value*100, 'f', 1, 64) + "%"
			},
			// 取最近一次带进程级下钻结果的内存采样，用于渲染 TOP N 区块
			"MemProcDetail": func(list []*memchecker.MemDetail) *memchecker.MemDetail {
				for i := len(list) - 1; i >= 0; i-- {
					if list[i] != nil && len(list[i].TopMemProcs) > 0 {
						return list[i]
					}
				}
				return nil
			},
			// 可以添加更多自定义函数...
		})

		// 解析嵌入的 HTML 模板
		tmpl, err := tmpl.ParseFS(templateFS, "templates/server_inspection.html")
		if err != nil {
			cmd.PrintErrf("解析模板失败: %v\n", err)
			return
		}

		reportDir := "reports"
		// 构建包含IP和时间戳的文件名
		fileName := fmt.Sprintf("server_report_%s_%s.html", reportData.HostInfo.IpAddr, timestampFile)
		filePath := filepath.Join(reportDir, fileName) // 使用filepath确保路径分隔符正确

		// 检查并创建目录
		if err := os.MkdirAll(reportDir, 0755); err != nil {
			cmd.PrintErrf("创建目录失败: %v\n", err)
			return
		}

		// 生成报告
		outputFile, err := os.Create(filePath)
		if err != nil {
			cmd.PrintErrf("创建文件失败: %v\n", err)
			return
		}
		defer func() {
			if cerr := outputFile.Close(); cerr != nil {
				cmd.PrintErrf("关闭文件失败: %v\n", cerr)
			}
		}()

		// 执行模板并写入文件
		if err := tmpl.Execute(outputFile, newReportData); err != nil {
			cmd.PrintErrf("生成报告失败: %v\n", err)
			return
		}
		cmd.Printf("报告已生成: %v\n", fileName)
	},
}

func init() {
	rootCmd.AddCommand(reportCmd)

	// 添加检查次数参数，默认值为5
	reportCmd.Flags().IntVarP(&checkCount, "count", "c", 5, "指定各项指标的检查次数")
}
