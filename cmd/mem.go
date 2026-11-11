/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/cuiwin/linux_server_check/checker/memchecker"
	"github.com/spf13/cobra"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// 颜色与样式
var (
	memTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#00D4FF")).
			Padding(0, 1).
			MarginBottom(1)

	memInfoLabelStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#888888")).
				Bold(true)

	memInfoValueStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#FFFFFF"))

	memPanelStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#555555")).
			Padding(1, 2).
			MarginBottom(1)

	memStatusInfoStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#00FF00")).
				Bold(true)

	memStatusWarnStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#FFAA00")).
				Bold(true)

	memStatusErrorStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#FF4444")).
				Bold(true)

	memFooterStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#666666")).
			MarginTop(1)

	memTableHeaderStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#AAAAAA")).
				Bold(true)

	memHighMemStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF4444")).Bold(true)
	memWarnMemStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFAA00")).Bold(true)
	memGoodMemStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#00FF00"))

	memBarFull  = "\u2588"
	memBarEmpty = "\u2591"
)

// memTickMsg 定时器消息
type memTickMsg struct{}

// memDataMsg 新数据消息
type memDataMsg struct {
	detail memchecker.MemDetail
}

// memTimestampedRecord 带时间戳的记录
type memTimestampedRecord struct {
	time   time.Time
	detail memchecker.MemDetail
}

// memModel TUI 数据模型
type memModel struct {
	quit       bool
	records    []memTimestampedRecord
	totalCount int
	maxRecords int
	ip         string
	width      int
	height     int
}

func (m memModel) Init() tea.Cmd {
	return tea.Batch(
		tea.Tick(time.Second, func(t time.Time) tea.Msg { return memTickMsg{} }),
	)
}

func (m memModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case memTickMsg:
		// 触发数据请求
		return m, func() tea.Msg {
			checker := memchecker.MemChecker{}
			detail, err := checker.GetDetail()
			if err != nil {
				return nil
			}
			return memDataMsg{detail: *detail}
		}

	case memDataMsg:
		d := msg.detail
		if m.ip == "" {
			m.ip = d.IP
		}
		m.records = append(m.records, memTimestampedRecord{
			time:   time.Now(),
			detail: d,
		})
		m.totalCount++
		if len(m.records) > m.maxRecords {
			m.records = m.records[len(m.records)-m.maxRecords:]
		}
		// 继续定时
		return m, tea.Tick(time.Second, func(t time.Time) tea.Msg { return memTickMsg{} })

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q", "esc":
			m.quit = true
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m memModel) View() string {
	if m.quit {
		return ""
	}

	var sections []string

	// ===== 标题栏 =====
	title := memTitleStyle.Render(" Linux Server Memory Monitor ")
	var serverInfo string
	if m.ip != "" {
		serverInfo = memInfoLabelStyle.Render("Server: ") + memInfoValueStyle.Render(m.ip)
	} else {
		serverInfo = memInfoLabelStyle.Render("Initializing...")
	}
	sections = append(sections, title)
	sections = append(sections, serverInfo)

	// ===== 当前状态面板 =====
	if len(m.records) > 0 {
		cur := m.records[len(m.records)-1].detail
		sections = append(sections, m.renderCurrentPanel(cur))
	}

	// ===== 历史表格 =====
	if len(m.records) > 0 {
		sections = append(sections, m.renderHistoryTable())
	}

	// ===== 底部提示 =====
	sections = append(sections, memFooterStyle.Render(fmt.Sprintf(" q: 退出  |  共 %d 条记录  |  每秒刷新", m.totalCount)))

	return lipgloss.NewStyle().Padding(1, 2).Render(lipgloss.JoinVertical(lipgloss.Left, sections...))
}

// renderCurrentPanel 渲染当前状态面板
func (m memModel) renderCurrentPanel(cur memchecker.MemDetail) string {
	memVal := cur.MemUsage.UserdPercent
	// 解析百分比数值
	pctStr := strings.TrimSuffix(memVal, "%")
	pctNum, _ := strconv.ParseFloat(pctStr, 64)

	bars := 40
	filled := int(pctNum / 100 * float64(bars))
	if filled < 0 {
		filled = 0
	}
	if filled > bars {
		filled = bars
	}
	var barStyle lipgloss.Style
	if pctNum >= 85 {
		barStyle = memHighMemStyle
	} else if pctNum >= 60 {
		barStyle = memWarnMemStyle
	} else {
		barStyle = memGoodMemStyle
	}
	var memBar string
	for i := 0; i < bars; i++ {
		if i < filled {
			memBar += barStyle.Render(memBarFull)
		} else {
			memBar += memGoodMemStyle.Render(memBarEmpty)
		}
	}

	// 状态标签
	var statusTag lipgloss.Style
	switch cur.Status {
	case "warning":
		statusTag = memStatusWarnStyle
	case "error":
		statusTag = memStatusErrorStyle
	default:
		statusTag = memStatusInfoStyle
	}
	statusLabel := statusTag.Render(" " + cur.Status + " ")
	descLabel := memInfoValueStyle.Render(cur.Desc)

	// 固定 label 宽度，保证 value 起始列一致
	labelW := 14
	label1 := memInfoLabelStyle.Render(padToWidth("Used Percent:", labelW))
	label2 := memInfoLabelStyle.Render(padToWidth("Total:", labelW))
	label3 := memInfoLabelStyle.Render(padToWidth("Available:", labelW))
	label4 := memInfoLabelStyle.Render(padToWidth("Status:", labelW))

	line1 := label1 + "  " + memInfoValueStyle.Render(memVal) + "  " + memBar
	line2 := label2 + "  " + memInfoValueStyle.Render(cur.MemUsage.Total) +
		memInfoLabelStyle.Render("  |  Used: ") + memInfoValueStyle.Render(cur.MemUsage.Used) +
		memInfoLabelStyle.Render("  |  Free: ") + memInfoValueStyle.Render(cur.MemUsage.Free)
	line3 := label3 + "  " + memInfoValueStyle.Render(cur.MemUsage.Available) +
		memInfoLabelStyle.Render("  |  Buffers: ") + memInfoValueStyle.Render(cur.MemUsage.Buffers) +
		memInfoLabelStyle.Render("  |  Cached: ") + memInfoValueStyle.Render(cur.MemUsage.Cached)
	line4 := label4 + "  " + statusLabel + "      " + descLabel

	return memPanelStyle.Render(lipgloss.JoinVertical(lipgloss.Left, line1, line2, line3, line4))
}

// renderHistoryTable 渲染历史表格
func (m memModel) renderHistoryTable() string {
	// 列定义（固定宽度）
	type col struct {
		title string
		width int
		align int // 0=left, 1=center, 2=right
	}
	cols := []col{
		{"TIME", 50, 1},
		{"TOTAL", 10, 2},
		{"USED", 10, 2},
		{"FREE", 10, 2},
		{"AVAILABLE", 11, 2},
		{"BUFFERS", 10, 2},
		{"CACHED", 10, 2},
		{"PROC_USED", 11, 2},
		{"KERN_USED", 11, 2},
		{"SLAB", 10, 2},
		{"SUNRECLAIM", 12, 2},
		{"VMALLOC", 10, 2},
		{"BLACKHOLE", 11, 2},
		{"USED%", 10, 2},
		{"STATUS", 12, 1},
	}

	// 列间距
	sep := memTableHeaderStyle.Render("   ")

	// 渲染表头
	var headerCells []string
	for i, c := range cols {
		if i > 0 {
			headerCells = append(headerCells, sep)
		}
		cell := memTableHeaderStyle.Render(padStr(c.title, c.width, c.align))
		headerCells = append(headerCells, cell)
	}
	headerRow := lipgloss.JoinHorizontal(lipgloss.Top, headerCells...)
	headerBorder := lipgloss.NewStyle().
		BorderStyle(lipgloss.NormalBorder()).
		BorderBottom(true).
		Foreground(lipgloss.Color("#555555")).
		Render(headerRow)

	// 渲染数据行（最后 N 行）
	maxRows := 12
	start := 0
	if len(m.records) > maxRows {
		start = len(m.records) - maxRows
	}

	var rows []string
	rows = append(rows, headerBorder)

	// 数据行间距
	dataSep := "   "

	for i := start; i < len(m.records); i++ {
		rec := m.records[i]
		r := rec.detail.MemUsage
		cells := []string{
			padStr(rec.time.Format("15:04:05"), 50, 1),
			padStr(r.Total, 10, 2),
			padStr(r.Used, 10, 2),
			padStr(r.Free, 10, 2),
			padStr(r.Available, 11, 2),
			padStr(r.Buffers, 10, 2),
			padStr(r.Cached, 10, 2),
			padStr(r.ProcessUserd, 11, 2),
			padStr(r.KernelUsed, 11, 2),
			padStr(r.Slab, 10, 2),
			padStr(r.SUnreclaim, 12, 2),
			padStr(r.VmallocUsed, 10, 2),
			padStr(r.BlackHole, 11, 2),
			padMemPercentCell(r.UserdPercent),
			padStatusCell(rec.detail.Status, 12),
		}

		var rowParts []string
		for j, cell := range cells {
			if j > 0 {
				rowParts = append(rowParts, dataSep)
			}
			rowParts = append(rowParts, cell)
		}
		rowStr := lipgloss.JoinHorizontal(lipgloss.Top, rowParts...)
		rows = append(rows, rowStr)
	}

	return lipgloss.JoinVertical(lipgloss.Left, rows...)
}

func padMemPercentCell(s string) string {
	pctStr := strings.TrimSuffix(s, "%")
	val, _ := strconv.ParseFloat(pctStr, 64)
	var style lipgloss.Style
	if val >= 85 {
		style = memHighMemStyle
	} else if val >= 60 {
		style = memWarnMemStyle
	} else {
		style = memGoodMemStyle
	}
	return style.Render(padStr(s, 10, 2))
}

// memCmd represents the mem command
var memCmd = &cobra.Command{
	Use:   "mem",
	Short: "服务器内存检查 (TUI 模式)",
	Long:  `服务器内存总体使用分析 - 使用 TUI 仪表盘实时展示`,
	RunE: func(cmd *cobra.Command, args []string) error {
		loopCount, _ := cmd.Flags().GetInt("count")

		p := tea.NewProgram(memModel{
			maxRecords: loopCount,
			records:    make([]memTimestampedRecord, 0, loopCount),
		}, tea.WithAltScreen())

		// 处理中断信号
		go func() {
			sigChan := make(chan os.Signal, 1)
			signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
			<-sigChan
			p.Quit()
		}()

		if _, err := p.Run(); err != nil {
			return fmt.Errorf("TUI 运行失败: %w", err)
		}
		return nil
	},
}

func init() {
	checkCmd.AddCommand(memCmd)

	// 添加命令行参数
	memCmd.Flags().IntP("count", "c", 20, "最大记录数")
}
