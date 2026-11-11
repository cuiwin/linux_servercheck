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

	"github.com/cuiwin/linux_server_check/checker/loadchecker"
	"github.com/spf13/cobra"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// 颜色与样式
var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#00D4FF")).
			Padding(0, 1).
			MarginBottom(1)

	infoLabelStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#888888")).
			Bold(true)

	infoValueStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFFFF"))

	panelStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#555555")).
			Padding(1, 2).
			MarginBottom(1)

	statusInfoStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#00FF00")).
			Bold(true)

	statusWarnStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFAA00")).
			Bold(true)

	statusErrorStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#FF4444")).
				Bold(true)

	footerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#666666")).
			MarginTop(1)

	tableHeaderStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#AAAAAA")).
				Bold(true)

	tableRowStyle = lipgloss.NewStyle()

	highCPUStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF4444")).Bold(true)
	warnCPUStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFAA00")).Bold(true)
	goodCPUStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#00FF00"))

	barFull  = "\u2588"
	barEmpty = "\u2591"
)

// tickMsg 定时器消息
type tickMsg struct{}

// dataMsg 新数据消息
type dataMsg struct {
	detail loadchecker.CpuLoadDetail
}

// timestampedRecord 带时间戳的记录
type timestampedRecord struct {
	time   time.Time
	detail loadchecker.CpuLoadDetail
}

// model TUI 数据模型
type model struct {
	quit       bool
	records    []timestampedRecord
	maxRecords int
	core       int
	ip         string
	width      int
	height     int
}

func (m model) Init() tea.Cmd {
	return tea.Batch(
		tea.Tick(time.Second, func(t time.Time) tea.Msg { return tickMsg{} }),
	)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tickMsg:
		// 触发数据请求
		return m, func() tea.Msg {
			checker := loadchecker.LoadChecker{}
			detail, err := checker.GetDetail()
			if err != nil {
				return nil
			}
			return dataMsg{detail: detail}
		}

	case dataMsg:
		d := msg.detail
		if m.core == 0 {
			m.core = d.Core
			m.ip = d.IP
		}
		m.records = append(m.records, timestampedRecord{
			time:   time.Now(),
			detail: d,
		})
		if len(m.records) > m.maxRecords {
			m.records = m.records[len(m.records)-m.maxRecords:]
		}
		// 继续定时
		return m, tea.Tick(time.Second, func(t time.Time) tea.Msg { return tickMsg{} })

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q", "esc":
			m.quit = true
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m model) View() string {
	if m.quit {
		return ""
	}

	var sections []string

	// ===== 标题栏 =====
	title := titleStyle.Render(" Linux Server Load Monitor ")
	var serverInfo string
	if m.core > 0 {
		serverInfo = infoLabelStyle.Render("Server: ") + infoValueStyle.Render(m.ip) +
			infoLabelStyle.Render("  Cores: ") + infoValueStyle.Render(strconv.Itoa(m.core))
	} else {
		serverInfo = infoLabelStyle.Render("Initializing...")
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
	count := len(m.records)
	sections = append(sections, footerStyle.Render(fmt.Sprintf(" q: 退出  |  共 %d 条记录  |  每秒刷新", count)))

	return lipgloss.NewStyle().Padding(1, 2).Render(lipgloss.JoinVertical(lipgloss.Left, sections...))
}

// renderCurrentPanel 渲染当前状态面板
func (m model) renderCurrentPanel(cur loadchecker.CpuLoadDetail) string {
	cpuVal := cur.CpuUsage.Total
	var cpuBar string
	bars := 40
	filled := int(cpuVal / 100 * float64(bars))
	if filled < 0 {
		filled = 0
	}
	if filled > bars {
		filled = bars
	}
	var barStyle lipgloss.Style
	if cpuVal >= 80 {
		barStyle = highCPUStyle
	} else if cpuVal >= 50 {
		barStyle = warnCPUStyle
	} else {
		barStyle = goodCPUStyle
	}
	for i := 0; i < bars; i++ {
		if i < filled {
			cpuBar += barStyle.Render(barFull)
		} else {
			cpuBar += goodCPUStyle.Render(barEmpty)
		}
	}

	// 状态标签
	var statusTag lipgloss.Style
	switch cur.Status {
	case "warning":
		statusTag = statusWarnStyle
	case "error":
		statusTag = statusErrorStyle
	default:
		statusTag = statusInfoStyle
	}
	statusLabel := statusTag.Render(" " + cur.Status + " ")
	descLabel := infoValueStyle.Render(cur.Desc)

	// 负载信息
	loadStr := fmt.Sprintf("%.2f  /  %.2f  /  %.2f", cur.Load.Load1, cur.Load.Load5, cur.Load.Load15)

	// 固定 label 宽度，保证 value 起始列一致
	labelW := 14
	label1 := infoLabelStyle.Render(padToWidth("Load 1/5/15:", labelW))
	label2 := infoLabelStyle.Render(padToWidth("CPU Usage:", labelW))
	label3 := infoLabelStyle.Render(padToWidth("Tasks:", labelW))
	label4 := infoLabelStyle.Render(padToWidth("Status:", labelW))

	valTotal := fmt.Sprintf("Total = %-4d", cur.Task.Total)
	valTasks := fmt.Sprintf("Running = %-3d", cur.Task.Running) +
		infoLabelStyle.Render(" | ") +
		fmt.Sprintf("Blocked = %-3d", cur.Task.Blocked) +
		infoLabelStyle.Render(" | ") +
		fmt.Sprintf("Zombie = %-3d", cur.Task.Zombie)

	line1 := label1 + "  " + infoValueStyle.Render(loadStr)
	line2 := label2 + "  " + infoValueStyle.Render(fmt.Sprintf("%.1f%%", cpuVal)) + "  " + cpuBar
	line3 := label3 + "  " + infoValueStyle.Render(valTotal) + "    " + infoValueStyle.Render(valTasks)
	line4 := label4 + "  " + statusLabel + "      " + descLabel

	return panelStyle.Render(lipgloss.JoinVertical(lipgloss.Left, line1, line2, line3, line4))
}

// renderHistoryTable 渲染历史表格
func (m model) renderHistoryTable() string {
	// 列定义（固定宽度）
	type col struct {
		title string
		width int
		align int // 0=left, 1=center, 2=right
	}
	cols := []col{
		{"TIME", 10, 1},
		{"TOTAL", 7, 2},
		{"RUNNING", 9, 2},
		{"BLOCKED", 9, 2},
		{"ZOMBIE", 8, 2},
		{"LOAD 1", 8, 2},
		{"LOAD 5", 8, 2},
		{"LOAD 15", 9, 2},
		{"CPU %", 8, 2},
		{"USER", 7, 2},
		{"SYSTEM", 8, 2},
		{"IDLE", 7, 2},
		{"IOWAIT", 8, 2},
		{"SOFTIRQ", 9, 2},
		{"STATUS", 10, 1},
	}

	// 列间距
	sep := tableHeaderStyle.Render("   ")

	// 渲染表头
	var headerCells []string
	for i, c := range cols {
		if i > 0 {
			headerCells = append(headerCells, sep)
		}
		cell := tableHeaderStyle.Render(padStr(c.title, c.width, c.align))
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
		r := rec.detail
		cells := []string{
			padStr(rec.time.Format("15:04:05"), 10, 1),
			padStr(strconv.Itoa(r.Task.Total), 7, 2),
			padStr(strconv.Itoa(r.Task.Running), 9, 2),
			padStr(strconv.Itoa(r.Task.Blocked), 9, 2),
			padStr(strconv.Itoa(r.Task.Zombie), 8, 2),
			padStr(fmt.Sprintf("%.2f", r.Load.Load1), 8, 2),
			padStr(fmt.Sprintf("%.2f", r.Load.Load5), 8, 2),
			padStr(fmt.Sprintf("%.2f", r.Load.Load15), 9, 2),
			padCPUCell(fmt.Sprintf("%.1f", r.CpuUsage.Total)),
			padStr(fmt.Sprintf("%.1f", r.CpuUsage.User), 7, 2),
			padStr(fmt.Sprintf("%.1f", r.CpuUsage.System), 8, 2),
			padStr(fmt.Sprintf("%.1f", r.CpuUsage.Idle), 7, 2),
			padStr(fmt.Sprintf("%.1f", r.CpuUsage.Iowait), 8, 2),
			padStr(fmt.Sprintf("%.1f", r.CpuUsage.Softirq), 9, 2),
			padStatusCell(r.Status, 10),
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

func padStr(s string, width int, align int) string {
	if len(s) > width {
		return s[:width]
	}
	padding := width - len(s)
	switch align {
	case 0: // left
		return s + spaces(padding)
	case 2: // right
		return spaces(padding) + s
	default: // center
		left := padding / 2
		right := padding - left
		return spaces(left) + s + spaces(right)
	}
}

func spaces(n int) string {
	if n <= 0 {
		return ""
	}
	return string(make([]byte, n))
}

func padToWidth(s string, width int) string {
	if len(s) >= width {
		return s
	}
	return s + strings.Repeat(" ", width-len(s))
}

func padCPUCell(s string) string {
	val, _ := strconv.ParseFloat(s, 64)
	var style lipgloss.Style
	if val >= 80 {
		style = highCPUStyle
	} else if val >= 50 {
		style = warnCPUStyle
	} else {
		style = goodCPUStyle
	}
	return style.Render(padStr(s, 8, 2))
}

func padStatusCell(status string, width int) string {
	var style lipgloss.Style
	switch status {
	case "warning":
		style = statusWarnStyle
	case "error":
		style = statusErrorStyle
	default:
		style = statusInfoStyle
	}
	return style.Render(padStr(status, width, 1))
}

// loadCmd represents the load command
var loadCmd = &cobra.Command{
	Use:   "load",
	Short: "服务器负载检查 (TUI 模式)",
	Long:  `服务器负载检查 - 使用 TUI 仪表盘实时展示`,
	RunE: func(cmd *cobra.Command, args []string) error {
		loopCount, _ := cmd.Flags().GetInt("count")

		p := tea.NewProgram(model{
			maxRecords: loopCount,
			records:    make([]timestampedRecord, 0, loopCount),
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
	checkCmd.AddCommand(loadCmd)
	loadCmd.Flags().IntP("count", "c", 20, "最大记录数")
}
