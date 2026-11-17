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
)
