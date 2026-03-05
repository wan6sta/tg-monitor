package services

import (
	"fmt"
	"time"

	"github.com/wan6sta/tg-monitor/internal/repo"
	"github.com/xuri/excelize/v2"
)

// ToXLSX генерирует xlsx-файл из списка сообщений и сохраняет по пути dst.
func ToXLSX(messages []repo.Message, dst string) error {
	f := excelize.NewFile()
	defer f.Close()

	sheet := "Результаты"
	f.SetSheetName("Sheet1", sheet)

	// --- Стили ---
	headerStyle, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Color: "FFFFFF", Size: 11, Family: "Arial"},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"2E75B6"}, Pattern: 1},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center", WrapText: true},
		Border: []excelize.Border{
			{Type: "bottom", Color: "1F4E79", Style: 2},
		},
	})

	evenStyle, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Size: 10, Family: "Arial"},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"EBF3FB"}, Pattern: 1},
		Alignment: &excelize.Alignment{Vertical: "top", WrapText: true},
	})

	oddStyle, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Size: 10, Family: "Arial"},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"FFFFFF"}, Pattern: 1},
		Alignment: &excelize.Alignment{Vertical: "top", WrapText: true},
	})

	dateStyle, _ := f.NewStyle(&excelize.Style{
		Font:         &excelize.Font{Size: 10, Family: "Arial"},
		Fill:         excelize.Fill{Type: "pattern", Color: []string{"FFFFFF"}, Pattern: 1},
		Alignment:    &excelize.Alignment{Vertical: "top"},
		CustomNumFmt: strPtr("DD.MM.YYYY HH:MM"),
	})

	dateEvenStyle, _ := f.NewStyle(&excelize.Style{
		Font:         &excelize.Font{Size: 10, Family: "Arial"},
		Fill:         excelize.Fill{Type: "pattern", Color: []string{"EBF3FB"}, Pattern: 1},
		Alignment:    &excelize.Alignment{Vertical: "top"},
		CustomNumFmt: strPtr("DD.MM.YYYY HH:MM"),
	})

	// --- Заголовки ---
	headers := []struct {
		col   string
		title string
		width float64
	}{
		{"A", "Дата и время", 20},
		{"B", "Чат", 25},
		{"C", "Топик", 25},
		{"D", "Отправитель", 22},
		{"E", "Ключевое слово", 18},
		{"F", "Сообщение", 60},
	}

	for _, h := range headers {
		cell := h.col + "1"
		f.SetCellValue(sheet, cell, h.title)
		f.SetCellStyle(sheet, cell, cell, headerStyle)
		f.SetColWidth(sheet, h.col, h.col, h.width)
	}
	f.SetRowHeight(sheet, 1, 30)

	// --- Данные ---
	for i, msg := range messages {
		row := i + 2
		isEven := i%2 == 0

		textStyle := oddStyle
		dtStyle := dateStyle
		if isEven {
			textStyle = evenStyle
			dtStyle = dateEvenStyle
		}

		topicLabel := msg.TopicTitle
		if topicLabel == "" && msg.TopicID != 0 {
			topicLabel = fmt.Sprintf("топик #%d", msg.TopicID)
		}

		// Дата
		dateCell := fmt.Sprintf("A%d", row)
		f.SetCellValue(sheet, dateCell, msg.SentAt.In(time.Local))
		f.SetCellStyle(sheet, dateCell, dateCell, dtStyle)

		// Остальные колонки
		values := []struct {
			col string
			val string
		}{
			{"B", msg.ChatTitle},
			{"C", topicLabel},
			{"D", msg.SenderName},
			{"E", msg.Keyword},
			{"F", msg.Text},
		}
		for _, v := range values {
			cell := fmt.Sprintf("%s%d", v.col, row)
			f.SetCellValue(sheet, cell, v.val)
			f.SetCellStyle(sheet, cell, cell, textStyle)
		}

		// Высота строки — примерно по длине текста
		lines := max(1, len([]rune(msg.Text))/80+1)
		if lines > 10 {
			lines = 10
		}
		f.SetRowHeight(sheet, row, float64(lines)*15+5)
	}

	// --- Фильтры и закрепление шапки ---
	f.AutoFilter(sheet, fmt.Sprintf("A1:F%d", len(messages)+1), nil)
	f.SetPanes(sheet, &excelize.Panes{
		Freeze:      true,
		Split:       false,
		XSplit:      0,
		YSplit:      1,
		TopLeftCell: "A2",
		ActivePane:  "bottomLeft",
	})

	// --- Лист со статистикой ---
	statsSheet := "Статистика"
	f.NewSheet(statsSheet)
	writeStats(f, statsSheet, messages)

	f.SetActiveSheet(0)

	return f.SaveAs(dst)
}

func writeStats(f *excelize.File, sheet string, messages []repo.Message) {
	titleStyle, _ := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true, Size: 13, Family: "Arial"},
	})
	boldStyle, _ := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true, Size: 10, Family: "Arial"},
		Fill: excelize.Fill{Type: "pattern", Color: []string{"D9E1F2"}, Pattern: 1},
	})
	numStyle, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Size: 10, Family: "Arial"},
		Alignment: &excelize.Alignment{Horizontal: "right"},
	})

	f.SetCellValue(sheet, "A1", "Статистика парсинга")
	f.SetCellStyle(sheet, "A1", "A1", titleStyle)
	f.SetCellValue(sheet, "A2", fmt.Sprintf("Всего сообщений: %d", len(messages)))

	// По ключевым словам
	kwCounts := make(map[string]int)
	chatCounts := make(map[string]int)
	for _, m := range messages {
		kwCounts[m.Keyword]++
		chatCounts[m.ChatTitle]++
	}

	row := 4
	f.SetCellValue(sheet, fmt.Sprintf("A%d", row), "По ключевым словам")
	f.SetCellStyle(sheet, fmt.Sprintf("A%d", row), fmt.Sprintf("B%d", row), boldStyle)
	row++
	for kw, cnt := range kwCounts {
		f.SetCellValue(sheet, fmt.Sprintf("A%d", row), kw)
		f.SetCellValue(sheet, fmt.Sprintf("B%d", row), cnt)
		f.SetCellStyle(sheet, fmt.Sprintf("B%d", row), fmt.Sprintf("B%d", row), numStyle)
		row++
	}

	row++
	f.SetCellValue(sheet, fmt.Sprintf("A%d", row), "По чатам")
	f.SetCellStyle(sheet, fmt.Sprintf("A%d", row), fmt.Sprintf("B%d", row), boldStyle)
	row++
	for chat, cnt := range chatCounts {
		f.SetCellValue(sheet, fmt.Sprintf("A%d", row), chat)
		f.SetCellValue(sheet, fmt.Sprintf("B%d", row), cnt)
		f.SetCellStyle(sheet, fmt.Sprintf("B%d", row), fmt.Sprintf("B%d", row), numStyle)
		row++
	}

	f.SetColWidth(sheet, "A", "A", 30)
	f.SetColWidth(sheet, "B", "B", 12)
}

func strPtr(s string) *string { return &s }

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
