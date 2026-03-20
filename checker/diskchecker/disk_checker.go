package diskchecker

import (
	"fmt"
	"strconv"

	"github.com/shirou/gopsutil/v4/disk"
)

type DiskUsage struct {
	Device            string
	Mountpoint        string
	Fstype            string
	Total             string
	Free              string
	Used              string
	UsedPercent       string
	InodesUsedPercent string
	Status            string
	Desc              string
}

// Checker 结构体定义
type DiskChecker struct {
	// 这里可以添加 Checker 结构体的字段
}

func formatSize(size uint64) string {
	const gb = 1024 * 1024 * 1024
	const mb = 1024 * 1024
	if size > gb {
		return fmt.Sprintf("%.2fGB", float64(size)/gb)
	} else if size > mb {
		return fmt.Sprintf("%.2fMB", float64(size)/mb)
	} else {
		return fmt.Sprintf("%.2fB", float64(size)/(1024))
	}
}

// 获取磁盘大小使用率
func (c *DiskChecker) GetDiskUsageInfo() ([]DiskUsage, error) {

	// 获取所有磁盘分区信息
	partitions, err := disk.Partitions(false)
	if err != nil {
		return nil, err
	}

	var diskUsages []DiskUsage
	for _, partition := range partitions {
		usage, err := disk.Usage(partition.Mountpoint)
		if err != nil {
			fmt.Printf("Error getting usage for %s: %v\n", partition.Mountpoint, err)
			continue
		}

		var status string
		var desc string
		if usage.UsedPercent > 85 {
			status = "warning"
			desc = usage.Path + " 使用率偏高"
		} else {
			status = "info"
			desc = usage.Path + " 使用率正常"
		}

		if usage.InodesUsedPercent > 85 {
			status = "warning"
			desc += ";" + usage.Path + " Inode使用率偏高"
		}

		diskUsage := DiskUsage{
			Device:            partition.Device,
			Mountpoint:        partition.Mountpoint,
			Fstype:            partition.Fstype,
			Total:             formatSize(usage.Total),
			Free:              formatSize(usage.Free),
			Used:              formatSize(usage.Used),
			UsedPercent:       strconv.FormatFloat(usage.UsedPercent, 'f', 2, 64),
			InodesUsedPercent: strconv.FormatFloat(usage.InodesUsedPercent, 'f', 2, 64),
			Status:            status,
			Desc:              desc,
		}
		diskUsages = append(diskUsages, diskUsage)
	}

	return diskUsages, nil

}

// 获取磁盘大小使用率打印
func (c *DiskChecker) GetDiskUsagePrint(disks []DiskUsage) [][]string {

	var items [][]string
	for _, disk := range disks {

		item := []string{disk.Device, disk.Mountpoint, disk.Fstype, disk.Total, disk.Used, disk.UsedPercent, disk.InodesUsedPercent, disk.Status, disk.Desc}
		items = append(items, item)
	}
	return items
}
