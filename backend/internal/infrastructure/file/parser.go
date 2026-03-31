package file

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"strings"

	"encoding/xml"
)

// FileParser 文件解析器
type FileParser struct{}

// NewFileParser 创建文件解析器
func NewFileParser() *FileParser {
	return &FileParser{}
}

// Parse 解析文件并提取文本内容
func (fp *FileParser) Parse(fileName string, data []byte) (string, error) {
	return ParseFile(fileName, data)
}

// ParsePDF 解析 PDF 文件（使用简单方法提取文本）
// 注：这是一个简化版本，仅提取可见的文本流
func ParsePDF(data []byte) (string, error) {
	// PDF 文本提取是复杂的，这里使用一个简单的方法
	// 查找文本对象并提取字符串
	content := string(data)
	
	// 移除 PDF 头尾标记
	content = strings.TrimPrefix(content, "%PDF-")
	
	// 提取 BT...ET 之间的文本（基本的 PDF 文本流）
	var result strings.Builder
	inTextBlock := false
	
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		
		if line == "BT" {
			inTextBlock = true
		} else if line == "ET" {
			inTextBlock = false
		} else if inTextBlock {
			// 提取字符串操作 (Tj, TJ 等)
			if strings.Contains(line, "Tj") || strings.Contains(line, "TJ") {
				// 简单的字符串提取
				if idx := strings.LastIndex(line, "("); idx != -1 {
					if jdx := strings.LastIndex(line, ")"); jdx > idx {
						text := line[idx+1 : jdx]
						// 解码转义序列
						text = decodeTextString(text)
						result.WriteString(text)
						result.WriteString(" ")
					}
				}
			}
		}
	}
	
	finalText := result.String()
	if strings.TrimSpace(finalText) == "" {
		// 如果没有提取到文本，返回原始内容（过滤二进制数据后）
		return filterBinaryContent(content), nil
	}
	return finalText, nil
}

// ParseDOCX 解析 DOCX 文件
func ParseDOCX(data []byte) (string, error) {
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", fmt.Errorf("failed to read docx as zip: %w", err)
	}

	var result strings.Builder

	// 查找 document.xml
	for _, file := range reader.File {
		if file.Name == "word/document.xml" {
			rc, err := file.Open()
			if err != nil {
				return "", err
			}
			defer rc.Close()

			content, err := io.ReadAll(rc)
			if err != nil {
				return "", err
			}

			// 解析 XML 并提取文本
			text, err := extractTextFromWordXML(content)
			if err != nil {
				return "", err
			}
			result.WriteString(text)
			break
		}
	}

	return result.String(), nil
}

// WordDocument 结构用于解析 Word XML
type WordDocument struct {
	Body struct {
		Paragraphs []struct {
			Text string `xml:"text"`
			Runs []struct {
				Text string `xml:"t"`
			} `xml:"r>t"`
		} `xml:"p"`
	} `xml:"body"`
}

func extractTextFromWordXML(xmlData []byte) (string, error) {
	var doc WordDocument
	err := xml.Unmarshal(xmlData, &doc)
	if err != nil {
		// 如果结构化解析失败，尝试简单的文本提取
		return extractPlainTextFromWordXML(xmlData)
	}

	var result strings.Builder
	for _, p := range doc.Body.Paragraphs {
		for _, run := range p.Runs {
			result.WriteString(run.Text)
		}
		result.WriteString("\n")
	}

	return result.String(), nil
}

func extractPlainTextFromWordXML(xmlData []byte) (string, error) {
	// 简单的正则表达式方法：查找 <w:t>...</w:t> 标签
	content := string(xmlData)
	var result strings.Builder

	// 查找所有 <w:t>text</w:t> 标签
	for {
		start := strings.Index(content, "<w:t>")
		if start == -1 {
			break
		}
		end := strings.Index(content, "</w:t>")
		if end == -1 {
			break
		}

		text := content[start+5 : end]
		result.WriteString(text)

		start = end + 6
		// 查找下一个段落标签 </w:p> 来确定何时添加换行
		if idx := strings.Index(content[start:], "</w:p>"); idx > 0 {
			// 检查在下一个 </w:p> 之前是否还有其他 <w:t>
			nextT := strings.Index(content[start:idx], "<w:t>")
			if nextT == -1 {
				result.WriteString("\n")
			}
		}

		content = content[start:]
	}

	return result.String(), nil
}

// ParseXLSX 解析 XLSX 文件
func ParseXLSX(data []byte) (string, error) {
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", fmt.Errorf("failed to read xlsx as zip: %w", err)
	}

	var result strings.Builder

	// 查找 sheet 文件
	for _, file := range reader.File {
		if strings.HasPrefix(file.Name, "xl/worksheets/sheet") && strings.HasSuffix(file.Name, ".xml") {
			rc, err := file.Open()
			if err != nil {
				return "", err
			}
			defer rc.Close()

			content, err := io.ReadAll(rc)
			if err != nil {
				return "", err
			}

			// 提取单元格数据
			text, err := extractTextFromSheetXML(content)
			if err != nil {
				return "", err
			}
			result.WriteString(text)
			result.WriteString("\n---\n")
		}
	}

	return result.String(), nil
}

func extractTextFromSheetXML(xmlData []byte) (string, error) {
	content := string(xmlData)
	var result strings.Builder

	// 查找所有 <v>value</v> 标签（单元格值）
	inRow := false
	for {
		start := strings.Index(content, "<c ")
		if start == -1 {
			break
		}

		// 查找单元格结束
		cellEnd := strings.Index(content[start:], "</c>")
		if cellEnd == -1 {
			break
		}

		// 查找值标签
		valueStart := strings.Index(content[start:], "<v>")
		if valueStart != -1 && valueStart < cellEnd {
			valueEnd := strings.Index(content[start+valueStart:], "</v>")
			if valueEnd != -1 {
				value := content[start+valueStart+3 : start+valueStart+valueEnd]
				result.WriteString(value)
				result.WriteString("\t")
			}
		}

		// 检查是否是行的结尾
		rowEnd := strings.Index(content[start:], "</row>")
		if rowEnd != -1 && rowEnd > cellEnd {
			if !inRow {
				inRow = true
			} else {
				result.WriteString("\n")
			}
		}

		content = content[start+cellEnd+4:]
	}

	return result.String(), nil
}

func decodeTextString(s string) string {
	// 移除转义序列
	s = strings.ReplaceAll(s, "\\n", "\n")
	s = strings.ReplaceAll(s, "\\r", "\r")
	s = strings.ReplaceAll(s, "\\t", "\t")
	s = strings.ReplaceAll(s, "\\\\", "\\")
	return s
}

func filterBinaryContent(content string) string {
	// 移除控制字符和不可打印字符
	var result strings.Builder
	for _, r := range content {
		if r >= 32 && r < 127 || r == '\n' || r == '\r' || r == '\t' {
			result.WriteRune(r)
		}
	}
	return result.String()
}

// DetectFileType 检测文件类型
func DetectFileType(fileName string) string {
	fileName = strings.ToLower(fileName)
	if strings.HasSuffix(fileName, ".pdf") {
		return "pdf"
	} else if strings.HasSuffix(fileName, ".docx") {
		return "docx"
	} else if strings.HasSuffix(fileName, ".xlsx") {
		return "xlsx"
	} else if strings.HasSuffix(fileName, ".txt") || strings.HasSuffix(fileName, ".md") {
		return "text"
	}
	return "unknown"
}

// ParseFile 根据文件类型解析文件内容
func ParseFile(fileName string, data []byte) (string, error) {
	fileType := DetectFileType(fileName)
	switch fileType {
	case "pdf":
		return ParsePDF(data)
	case "docx":
		return ParseDOCX(data)
	case "xlsx":
		return ParseXLSX(data)
	case "text":
		return string(data), nil
	default:
		return "", fmt.Errorf("unsupported file type: %s", fileType)
	}
}
