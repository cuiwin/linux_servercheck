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
			Padding(0, 1)

	memInfoLabelStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#888888")).
				Bold(true)

	memInfoValueStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#FFFFFF"))

	memPanelStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#555555")).
			Padding(0, 2)

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

	memBarEmptyStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#444444"))

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
			// TUI 需要始终展示进程 TOP，不依赖告警阈值
			checker := memchecker.MemChecker{AlwaysTopProcs: true}
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

	// 终端可用内容宽度（外层左右 padding 各 2）
	contentW := m.width - 4
	if contentW < 40 {
		contentW = 100
	}

	var sections []string

	// ===== 标题栏 =====
	sections = append(sections, memTitleStyle.Render(" Linux Server Memory Monitor "))
	if m.ip != "" {
		sections = append(sections, memInfoLabelStyle.Render("Server: ")+memInfoValueStyle.Render(m.ip))
	} else {
		sections = append(sections, memInfoLabelStyle.Render("Initializing..."))
	}
	sections = append(sections, "")

	if len(m.records) > 0 {
		cur := m.records[len(m.records)-1].detail

		// ===== 上半区：左侧内存概览 + 右侧进程 TOP N =====
		// 宽终端左右并排，窄终端自动上下堆叠，避免表格被挤压
		leftW := contentW * 45 / 100
		rightW := contentW - leftW
		if rightW < 44 {
			sections = append(sections,
				m.renderCurrentPanel(cur, contentW),
				m.renderTopMemProcsPanel(cur, contentW))
		} else {
			sections = append(sections, lipgloss.JoinHorizontal(lipgloss.Top,
				m.renderCurrentPanel(cur, leftW),
				m.renderTopMemProcsPanel(cur, rightW)))
		}

		// ===== 历史表格 =====
		sections = append(sections, "", m.renderHistoryTable(contentW))
	}

	// ===== 底部提示 =====
	sections = append(sections, "", memFooterStyle.Render(fmt.Sprintf(" q: 退出  |  共 %d 条记录  |  每秒刷新", m.totalCount)))

	return lipgloss.NewStyle().Padding(1, 2).Render(lipgloss.JoinVertical(lipgloss.Left, sections...))
}

// renderCurrentPanel 渲染当前状态面板
func (m memModel) renderCurrentPanel(cur memchecker.MemDetail, panelW int) string {
	inner := panelW - 6 // 边框 2 + 左右 padding 4
	if inner < 30 {
		inner = 30
	}

	pctStr := strings.TrimSuffix(cur.MemUsage.UserdPercent, "%")
	pctNum, _ := strconv.ParseFloat(pctStr, 64)

	var barStyle lipgloss.Style
	switch {
	case pctNum >= 85:
		barStyle = memHighMemStyle
	case pctNum >= 60:
		barStyle = memWarnMemStyle
	default:
		barStyle = memGoodMemStyle
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

	labelW := 8
	valueW := 8
	kv := func(label, value string) string {
		return memInfoLabelStyle.Render(padStr(label, labelW, 0)) + " " +
			memInfoValueStyle.Render(padStr(value, valueW, 0))
	}

	// 进度条长度跟随面板宽度
	pctText := padStr(cur.MemUsage.UserdPercent, 7, 2)
	barW := inner - labelW - 1 - 7 - 1
	if barW < 10 {
		barW = 10
	}
	filled := int(pctNum / 100 * float64(barW))
	if filled < 0 {
		filled = 0
	}
	if filled > barW {
		filled = barW
	}
	var memBar string
	for i := 0; i < barW; i++ {
		if i < filled {
			memBar += barStyle.Render(memBarFull)
		} else {
			memBar += memBarEmptyStyle.Render(memBarEmpty)
		}
	}

	line1 := memInfoLabelStyle.Render(padStr("Used", labelW, 0)) + " " + barStyle.Render(pctText) + " " + memBar

	// 指标成对展示；面板过窄时改为单列，避免内容撑破边框
	pairs := [][2]string{
		{"Total", cur.MemUsage.Total}, {"Used", cur.MemUsage.Used},
		{"Free", cur.MemUsage.Free}, {"Avail", cur.MemUsage.Available},
		{"Buffers", cur.MemUsage.Buffers}, {"Cached", cur.MemUsage.Cached},
		{"Kernel", cur.MemUsage.KernelUsed}, {"Slab", cur.MemUsage.Slab},
		{"Hole", cur.MemUsage.BlackHole},
	}
	var stats []string
	if inner >= 2*(labelW+1+valueW)+2 {
		for i := 0; i < len(pairs); i += 2 {
			line := kv(pairs[i][0], pairs[i][1])
			if i+1 < len(pairs) {
				line += "  " + kv(pairs[i+1][0], pairs[i+1][1])
			}
			stats = append(stats, line)
		}
	} else {
		for _, p := range pairs {
			stats = append(stats, kv(p[0], p[1]))
		}
	}

	// 描述按面板宽度截断，避免撑破布局
	tagWidth := len(cur.Status) + 4
	descW := inner - labelW - 1 - tagWidth - 2
	if descW < 0 {
		descW = 0
	}
	line7 := memInfoLabelStyle.Render(padStr("Status", labelW, 0)) + " " +
		statusTag.Render(" "+cur.Status+" ") + "  " + memInfoValueStyle.Render(truncateWidth(cur.Desc, descW))

	lines := []string{memTableHeaderStyle.Render("内存使用概览"), line1}
	lines = append(lines, stats...)
	lines = append(lines, line7)
	return renderMemPanel(lines, inner)
}

// renderTopMemProcsPanel 渲染内存占用 TOP N 面板
func (m memModel) renderTopMemProcsPanel(cur memchecker.MemDetail, panelW int) string {
	inner := panelW - 6 // 边框 2 + 左右 padding 4
	if inner < 30 {
		inner = 30
	}

	header := "内存占用 TOP"
	if len(cur.TopMemProcs) > 0 {
		header += strconv.Itoa(len(cur.TopMemProcs))
	}
	if cur.Status != "info" {
		header += "（超阈值）"
	}
	lines := []string{memTableHeaderStyle.Render(header)}

	const (
		pidW  = 7
		rssW  = 9
		memPW = 7
		sepW  = 2
	)
	nameW := inner - pidW - rssW - memPW - sepW*3
	if nameW < 6 {
		nameW = 6
	}
	if nameW > 40 {
		nameW = 40
	}

	lines = append(lines, memTableHeaderStyle.Render(
		padStr("PID", pidW, 2)+spaces(sepW)+
			padDisplay("PROCESS", nameW, 0)+spaces(sepW)+
			padStr("RSS", rssW, 2)+spaces(sepW)+
			padStr("MEM%", memPW, 2)))

	if len(cur.TopMemProcs) == 0 {
		lines = append(lines, memInfoLabelStyle.Render("暂无进程数据"))
	}
	for _, p := range cur.TopMemProcs {
		var pctStyle lipgloss.Style
		switch {
		case p.MemPercent >= 50:
			pctStyle = memHighMemStyle
		case p.MemPercent >= 20:
			pctStyle = memWarnMemStyle
		default:
			pctStyle = memGoodMemStyle
		}
		lines = append(lines,
			memInfoValueStyle.Render(padStr(strconv.Itoa(p.PID), pidW, 2))+spaces(sepW)+
				memInfoValueStyle.Render(padDisplay(truncateWidth(p.Name, nameW), nameW, 0))+spaces(sepW)+
				memInfoValueStyle.Render(padStr(p.RSS, rssW, 2))+spaces(sepW)+
				pctStyle.Render(padStr(fmt.Sprintf("%.1f%%", p.MemPercent), memPW, 2)))
	}
	if cur.ProcSummary.Desc != "" {
		for _, l := range wrapWidth(cur.ProcSummary.Desc, inner) {
			lines = append(lines, memInfoLabelStyle.Render(l))
		}
	}

	return renderMemPanel(lines, inner)
}

// renderHistoryTable 渲染历史表格
func (m memModel) renderHistoryTable(maxWidth int) string {
	// 列定义（固定宽度，按重要性排序，宽度不足时丢弃尾部列）
	type col struct {
		key   string
		title string
		width int
		align int // 0=left, 1=center, 2=right
	}
	all := []col{
		{"time", "TIME", 8, 1},
		{"pct", "USED%", 8, 2},
		{"used", "USED", 9, 2},
		{"avail", "AVAIL", 9, 2},
		{"status", "STATUS", 10, 1},
		{"proc", "PROC", 9, 2},
		{"kern", "KERN", 9, 2},
		{"hole", "BLACKHOLE", 10, 2},
		{"cached", "CACHED", 9, 2},
		{"free", "FREE", 9, 2},
		{"buffers", "BUFFERS", 9, 2},
		{"slab", "SLAB", 9, 2},
	}

	// 前 5 列与 STATUS 固定展示，其余列按剩余宽度补充
	const sepW = 2
	cols := make([]col, 0, len(all))
	totalW := 0
	for i, c := range all {
		if i == 4 {
			continue // STATUS 最后追加，避免被可选列挤掉
		}
		add := c.width
		if len(cols) > 0 {
			add += sepW
		}
		reserve := sepW + all[4].width // 给 STATUS 预留宽度
		if totalW+add+reserve > maxWidth && len(cols) >= 4 {
			break
		}
		cols = append(cols, c)
		totalW += add
	}
	cols = append(cols, all[4])

	// 渲染表头
	headerCells := make([]string, 0, len(cols)*2)
	for i, c := range cols {
		if i > 0 {
			headerCells = append(headerCells, spaces(sepW))
		}
		headerCells = append(headerCells, memTableHeaderStyle.Render(padStr(c.title, c.width, c.align)))
	}
	headerRow := lipgloss.JoinHorizontal(lipgloss.Top, headerCells...)
	headerBorder := lipgloss.NewStyle().
		BorderStyle(lipgloss.NormalBorder()).
		BorderBottom(true).
		Foreground(lipgloss.Color("#555555")).
		Render(headerRow)

	// 数据行数量随终端高度调整
	maxRows := m.height - 23
	if maxRows < 4 {
		maxRows = 4
	}
	if maxRows > 20 {
		maxRows = 20
	}
	start := 0
	if len(m.records) > maxRows {
		start = len(m.records) - maxRows
	}

	rows := []string{memTableHeaderStyle.Render("历史趋势"), headerBorder}
	for i := start; i < len(m.records); i++ {
		rec := m.records[i]
		r := rec.detail.MemUsage

		cells := make([]string, 0, len(cols)*2)
		for j, c := range cols {
			if j > 0 {
				cells = append(cells, spaces(sepW))
			}
			var cell string
			switch c.key {
			case "time":
				cell = memInfoValueStyle.Render(padStr(rec.time.Format("15:04:05"), c.width, c.align))
			case "pct":
				cell = padMemPercentCell(r.UserdPercent, c.width)
			case "used":
				cell = memInfoValueStyle.Render(padStr(r.Used, c.width, c.align))
			case "avail":
				cell = memInfoValueStyle.Render(padStr(r.Available, c.width, c.align))
			case "proc":
				cell = memInfoValueStyle.Render(padStr(r.ProcessUserd, c.width, c.align))
			case "kern":
				cell = memInfoValueStyle.Render(padStr(r.KernelUsed, c.width, c.align))
			case "hole":
				cell = memInfoValueStyle.Render(padStr(r.BlackHole, c.width, c.align))
			case "status":
				cell = padStatusCell(rec.detail.Status, c.width)
			case "cached":
				cell = memInfoValueStyle.Render(padStr(r.Cached, c.width, c.align))
			case "free":
				cell = memInfoValueStyle.Render(padStr(r.Free, c.width, c.align))
			case "buffers":
				cell = memInfoValueStyle.Render(padStr(r.Buffers, c.width, c.align))
			case "slab":
				cell = memInfoValueStyle.Render(padStr(r.Slab, c.width, c.align))
			}
			cells = append(cells, cell)
		}
		rows = append(rows, lipgloss.JoinHorizontal(lipgloss.Top, cells...))
	}

	return lipgloss.JoinVertical(lipgloss.Left, rows...)
}

// renderMemPanel 统一面板宽度：内容行补齐到 inner 宽度后再加边框与内边距
func renderMemPanel(lines []string, inner int) string {
	padded := make([]string, 0, len(lines))
	for _, l := range lines {
		padded = append(padded, fitLine(l, inner))
	}
	return memPanelStyle.Render(lipgloss.JoinVertical(lipgloss.Left, padded...))
}

// fitLine 按可见宽度补齐一行
func fitLine(s string, width int) string {
	w := lipgloss.Width(s)
	if w >= width {
		return s
	}
	return s + spaces(width-w)
}

// padDisplay 按可见宽度（中文占 2 列）补齐字符串
func padDisplay(s string, width int, align int) string {
	w := lipgloss.Width(s)
	if w >= width {
		return s
	}
	pad := width - w
	switch align {
	case 2: // right
		return spaces(pad) + s
	case 1: // center
		left := pad / 2
		return spaces(left) + s + spaces(pad-left)
	default: // left
		return s + spaces(pad)
	}
}

// truncateWidth 按可见宽度截断（中文占 2 列），超出部分以省略号结尾
func truncateWidth(s string, width int) string {
	if width <= 0 || lipgloss.Width(s) <= width {
		return s
	}
	var b strings.Builder
	w := 0
	for _, r := range s {
		rw := lipgloss.Width(string(r))
		if w+rw > width-1 {
			break
		}
		b.WriteRune(r)
		w += rw
	}
	return b.String() + "…"
}

// wrapWidth 按可见宽度换行，长描述在面板内折行展示
func wrapWidth(s string, width int) []string {
	if width <= 0 {
		return []string{s}
	}
	var lines []string
	var b strings.Builder
	w := 0
	for _, r := range s {
		rw := lipgloss.Width(string(r))
		if w+rw > width && b.Len() > 0 {
			lines = append(lines, b.String())
			b.Reset()
			w = 0
		}
		b.WriteRune(r)
		w += rw
	}
	if b.Len() > 0 {
		lines = append(lines, b.String())
	}
	return lines
}

func padMemPercentCell(s string, width int) string {
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
	return style.Render(padStr(s, width, 2))
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
