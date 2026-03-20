package kernelreporter

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"

	_ "embed"
)

// KernelParam 表示一个内核参数及其比较结果
type KernelParam struct {
	Name          string
	TemplateValue string
	SystemValue   string
	Matched       bool
	Desc          string
}

// ComparisonSummary 保存比较结果的汇总信息
type ComparisonSummary struct {
	TotalParams      int
	MatchedParams    int
	MismatchedParams int
	MatchedPercent   float64
}

//go:embed kernel_template.txt
var templateContent []byte

// Checker 结构体定义
type KernelReporter struct {
	// 这里可以添加 Checker 结构体的字段
}

// 获取内核参数列表
func (c *KernelReporter) GetKernelParamList() ([]KernelParam, ComparisonSummary, error) {

	// 解析嵌入的模板内容
	templateParams, err := parseTemplateContent(templateContent)
	if err != nil {
		return nil, ComparisonSummary{}, fmt.Errorf("解析内核模板内容失败: %v", err)
	}

	// 获取系统内核参数
	systemParams, err := getSystemKernelParams()
	if err != nil {
		return nil, ComparisonSummary{}, fmt.Errorf("获取系统内核参数失败: %v", err)
	}

	results := compareParams(templateParams, systemParams)

	// 排序
	sort.Slice(results, func(i, j int) bool {
		return results[i].Name < results[j].Name
	})

	// 生成汇总信息
	summary := generateSummary(results)

	return results, summary, nil

}

// 解析嵌入的模板内容
func parseTemplateContent(content []byte) (map[string]string, error) {
	params := make(map[string]string)
	scanner := bufio.NewScanner(strings.NewReader(string(content)))
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue // 跳过空行和注释
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			// 记录错误但继续处理其他行
			fmt.Printf("警告: 第%d行格式不正确 - %s\n", lineNum, line)
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		params[key] = value
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("解析模板内容失败: %v", err)
	}

	return params, nil
}

// 获取系统内核参数
func getSystemKernelParams() (map[string]string, error) {
	params := make(map[string]string)

	// 遍历/proc/sys目录下的所有文件
	err := filepath.Walk("/proc/sys", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// 只处理文件
		if info.IsDir() {
			return nil
		}

		// 计算内核参数名称（相对路径并替换路径分隔符为点）
		relPath, err := filepath.Rel("/proc/sys", path)
		if err != nil {
			return err
		}
		paramName := strings.ReplaceAll(relPath, string(os.PathSeparator), ".")

		// 读取文件内容
		data, err := os.ReadFile(path)
		if err != nil {
			// 忽略无法读取的文件（权限问题等）
			return nil
		}

		// 去除换行符并保存参数值
		params[paramName] = strings.TrimSpace(string(data))

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("遍历系统内核参数失败: %v", err)
	}

	return params, nil
}

// 比较模板参数和系统参数
func compareParams(template, system map[string]string) []KernelParam {
	var results []KernelParam

	for name, templateValue := range template {
		systemValue, exists := system[name]

		result := KernelParam{
			Name:          name,
			TemplateValue: templateValue,
			SystemValue:   "",
			Matched:       false,
		}

		if exists {
			result.SystemValue = systemValue
			result.Matched = templateValue == strings.TrimSpace(systemValue)
			result.Matched = templateValue == strings.TrimSpace(systemValue) || areValuesEqual(templateValue, systemValue)
			// fmt.Println("templateValue", templateValue)
			// fmt.Println("systemValue", strings.TrimSpace(systemValue))
		} else {
			result.SystemValue = "[不存在]"
		}

		if result.Matched {
			result.Desc = "匹配"
		} else {
			result.Desc = "不匹配"
		}

		results = append(results, result)
	}

	return results
}

func areValuesEqual(template, system string) bool {
	// 处理字符串：将连续空格替换为单个空格，再去除首尾空格
	templateClean := strings.Join(strings.Fields(template), " ")
	systemClean := strings.Join(strings.Fields(system), " ")

	// 拆分为字符串切片
	templateParts := strings.Split(templateClean, " ")
	systemParts := strings.Split(systemClean, " ")

	// 检查长度是否相同
	if len(templateParts) != len(systemParts) {
		return false
	}

	// 逐个比较数值
	for i := range templateParts {
		// 将字符串转换为整数
		tVal, err1 := strconv.Atoi(templateParts[i])
		sVal, err2 := strconv.Atoi(systemParts[i])

		// 若转换失败或数值不相等，则返回 false
		if err1 != nil || err2 != nil || tVal != sVal {
			return false
		}
	}

	return true
}

// 生成比较结果的汇总信息
func generateSummary(results []KernelParam) ComparisonSummary {
	summary := ComparisonSummary{
		TotalParams:   len(results),
		MatchedParams: 0,
	}

	for _, result := range results {
		if result.Matched {
			summary.MatchedParams++
		}
	}

	summary.MismatchedParams = summary.TotalParams - summary.MatchedParams
	summary.MatchedPercent = float64(summary.MatchedParams) / float64(summary.TotalParams) * 100
	return summary
}

// 打印比较结果到控制台
func printResults(results []KernelParam) {
	writer := tabwriter.NewWriter(os.Stdout, 4, 4, 2, ' ', 0)
	fmt.Fprintln(writer, "参数名称\t模板值\t系统值\t是否匹配")
	fmt.Fprintln(writer, "--------\t--------\t--------\t--------")

	for _, result := range results {
		status := "否"
		if result.Matched {
			status = "是"
		}
		fmt.Fprintf(writer, "%s\t%s\t%s\t%s\n",
			result.Name, result.TemplateValue, result.SystemValue, status)
	}

	writer.Flush()
}

// 打印汇总信息
func printSummary(summary ComparisonSummary) {
	fmt.Printf("\n===== 比较汇总 =====\n")
	fmt.Printf("总共比较参数数量: %d\n", summary.TotalParams)
	fmt.Printf("匹配参数数量: %d\n", summary.MatchedParams)
	fmt.Printf("不匹配参数数量: %d\n", summary.MismatchedParams)
	fmt.Printf("匹配率: %.2f%%\n", float64(summary.MatchedParams)/float64(summary.TotalParams)*100)
	fmt.Printf("===================\n")
}
