package config

// 各指标的评价阈值，供所有 checker 统一使用
const (
	// CPU
	CpuLoadPerCore    = 1.4 // 每核负载阈值（load1/5/15 与核心数的比值）
	CpuLoadTrendDelta = 0.1 // 每核 load1 与 load15 的差值阈值，超过才判定为上升/下降趋势
	CpuBlockedRatio   = 0.5 // 负载偏高时，D 状态任务占 R+D 总数的比例阈值，超过说明瓶颈更可能在 IO/网络
	CpuSoftirqPercent = 10  // 软中断占用 CPU 百分比
	CpuHardirqPercent = 10  // 硬中断占用 CPU 百分比
	CpuIowaitPercent  = 10  // IO 等待占用 CPU 百分比
	CpuUsagePercent   = 85  // CPU 总使用率百分比
	CpuUserPercent    = 85  // 用户态 CPU 使用率百分比
	CpuSystemPercent  = 20  // 内核态 CPU 使用率百分比

	// 磁盘
	DiskUsagePercent  = 85           // 磁盘空间使用率百分比
	DiskUsageHighDesc = "使用率偏高"      // 磁盘空间使用率超过阈值时的描述
	DiskInodePercent  = 85           // 磁盘 inode 使用率百分比
	DiskInodeHighDesc = "Inode使用率偏高" // 磁盘 inode 使用率超过阈值时的描述

	// 磁盘 IO
	DiskIOUtilPercent = 90     // 磁盘 IO 利用率百分比
	DiskIOBusyDesc    = "IO繁忙" // 磁盘 IO 利用率超过阈值时的描述

	// 内存
	MemUsagePercent  = 85          // 内存使用率百分比
	MemUsageHighDesc = "内存使用率超过阈值" // 内存使用率超过阈值时的描述
	MemTopProcN      = 5           // 内存告警时展示的进程数
	MemTopProcRatio  = 0.5         // TOP 进程 RSS 合计占已用内存的比例，低于该值说明主因不在用户进程
)

// 采集路径
const (
	DataDiskPath = "/data" // 数据盘挂载点，用于统计数据盘总量
)
