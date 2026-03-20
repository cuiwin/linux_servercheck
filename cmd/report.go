/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"embed"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"text/template"
	"time"

	"github.com/cuiwin/linux_server_check/checker/diskchecker"
	"github.com/cuiwin/linux_server_check/checker/loadchecker"
	"github.com/cuiwin/linux_server_check/checker/memchecker"
	"github.com/cuiwin/linux_server_check/reporter/diskioreporter"
	"github.com/cuiwin/linux_server_check/reporter/hostreporter"
	"github.com/cuiwin/linux_server_check/reporter/kernelreporter"
	"github.com/cuiwin/linux_server_check/reporter/netreporter"

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
	HostInfo                  hostreporter.HostInfo               `json:"HostInfo"`
	Timestamp                 string                              `json:"timestamp"`
	DiskUsageList             []diskchecker.DiskUsage             `json:"diskUsageList"`
	CpuLoadDetailList         []loadchecker.CpuLoadDetail         `json:"cpuLoadDetailList"`
	MemDetailList             []*memchecker.MemDetail             `json:"memDetailList"`
	DiskIOStatsList           []diskioreporter.DiskIOStats        `json:"diskIOStatsList"`
	KernelParamList           []kernelreporter.KernelParam        `json:"kernelParamList"`
	KernelComparisonSummary   kernelreporter.ComparisonSummary    `json:"kernelComparisonSummary"`
	NetworkInterfaceList      []netreporter.NetworkInterface      `json:"networkInterfaceList"`
	NetworkInterfaceStatsList []netreporter.NetworkInterfaceStats `json:"networkInterfaceStatsList"`
	NormalSummaryList         []SummaryInfo                       `json:"normalSummaryList"`
	WarningSummaryList        []SummaryInfo                       `json:"warningSummaryList"`
	ErrorSummaryList          []SummaryInfo                       `json:"errorSummaryList"`
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

		reportData := ReportData{
			Timestamp: time.Now().Format("2006-01-02 15:04:05"),
		}

		// -------  主机信息 begin  --------
		fmt.Println("开始检查主机 - 【主机信息】")
		hostreporterInstance := hostreporter.HostReporter{}
		hostreporterInstance.GetHostInfo()
		hostInfo, err := hostreporterInstance.GetHostInfo()
		if err != nil {
			log.Printf("获取主机信息失败: %v", err)
			return
		}
		reportData.HostInfo = hostInfo

		// -------  主机信息 end  --------

		// -------  磁盘信息 begin  --------
		fmt.Println("开始检查主机 - 【磁盘信息】")
		diskcheckerInstance := diskchecker.DiskChecker{}
		disk_usage_list, err := diskcheckerInstance.GetDiskUsageInfo()
		if err != nil {
			fmt.Printf("Error GetDiskUsageInfo: %v\n", err)
		}
		reportData.DiskUsageList = disk_usage_list

		// -------  磁盘信息 end  --------

		// -------  负载信息 begin  --------
		fmt.Println("开始检查主机 - 【负载信息】")
		var results []loadchecker.CpuLoadDetail

		loadcheckerInstance := loadchecker.LoadChecker{}

		// 循环调用函数 5 次，并将结果保存到数组中
		for i := 0; i < checkCount; i++ {

			info, _ := loadcheckerInstance.GetDetail()
			results = append(results, info)
		}
		reportData.CpuLoadDetailList = results

		// -------  负载信息 end  --------

		// -------  内存信息 begin  --------
		fmt.Println("开始检查主机 - 【内存信息】")
		var mem_results []*memchecker.MemDetail
		memcheckerInstance := memchecker.MemChecker{}
		// 循环调用函数 5 次，并将结果保存到数组中
		for i := 0; i < checkCount; i++ {

			info, _ := memcheckerInstance.GetDetail()
			mem_results = append(mem_results, info)
		}
		reportData.MemDetailList = mem_results
		// -------  内存信息 end  --------

		// -------  磁盘IO信息 begin  --------
		fmt.Println("开始检查主机 - 【磁盘IO】")

		var disk_io_results []diskioreporter.DiskIOStats
		diskioreporterInstance := diskioreporter.DiskIOReporter{}

		// 先获取iostat版本
		version, err := diskioreporterInstance.GetIostatVersion()
		if err != nil {
			fmt.Printf("获取iostat版本失败: %v\n", err)
		}
		fmt.Printf("获取iostat命令版本: %v\n", version)

		// 循环调用函数 5 次，并将结果保存到数组中
		for i := 0; i < checkCount; i++ {
			info, _ := diskioreporterInstance.GetDiskStats(version)
			disk_io_results = append(disk_io_results, info...)
		}

		// 排序
		sort.Slice(disk_io_results, func(i, j int) bool {
			return disk_io_results[i].Device < disk_io_results[j].Device
		})

		reportData.DiskIOStatsList = disk_io_results
		// -------  磁盘IO信息  end   --------

		// -------  内核参数信息 begin  --------
		fmt.Println("开始检查主机 - 【内核参数】")
		kernelreporterInstance := kernelreporter.KernelReporter{}

		list, sumary, err := kernelreporterInstance.GetKernelParamList()
		if err != nil {
			fmt.Printf("Error GetKernelParamList: %v\n", err)
		}
		reportData.KernelParamList = list
		reportData.KernelComparisonSummary = sumary

		// -------  内核参数信息 end  --------

		// -------  网络IO begin  --------
		fmt.Println("开始检查主机 - 【网络信息】")

		netreporterInstance := netreporter.NetReporter{}

		ifacelist, err := netreporterInstance.GetNetworkInterfaceList()
		if err != nil {
			fmt.Printf("Error GetNetworkInterfaceList: %v\n", err)
		}
		reportData.NetworkInterfaceList = ifacelist

		var net_io_results []netreporter.NetworkInterfaceStats
		// 循环调用函数 5 次，并将结果保存到数组中
		for i := 0; i < checkCount; i++ {
			info, _ := netreporterInstance.GetNetworkStatsList()
			net_io_results = append(net_io_results, info...)
		}

		// 根据网卡名称排序
		sort.Slice(net_io_results, func(i, j int) bool {
			return net_io_results[i].Interface < net_io_results[j].Interface
		})

		reportData.NetworkInterfaceStatsList = net_io_results
		// -------  网络信息IO end  --------

		// -------  信息汇总 begin  --------
		fmt.Println("开始检查主机 - 【信息汇总】")
		newReportData := analyzeReportData(reportData)
		// -------  信息汇总 end  --------

		// 创建 Template 对象并注册函数
		tmpl := template.New("server_inspection.html").Funcs(template.FuncMap{
			"FormatFloat": FormatFloat, // 自定义时间格式化函数
			// 可以添加更多自定义函数...
		})

		// 解析嵌入的 HTML 模板
		tmpl, err = tmpl.ParseFS(templateFS, "templates/server_inspection.html")
		if err != nil {
			cmd.PrintErrf("解析模板失败: %v\n", err)
			return
		}

		// 获取当前时间戳
		timestamp := time.Now().Format("20060102150405")

		reportDir := "reports"
		// 构建包含IP和时间戳的文件名
		fileName := fmt.Sprintf("server_report_%s_%s.html", reportData.HostInfo.IpAddr, timestamp)
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
		defer outputFile.Close()

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
	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// reportCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// reportCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
