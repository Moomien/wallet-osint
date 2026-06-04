package fyneapp

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"
)

// tableWidget управляет таблицей данных
type tableWidget struct {
	app   *App
	table *widget.Table
}

// newTableWidget создает новый виджет таблицы
func newTableWidget(app *App) *tableWidget {
	tw := &tableWidget{
		app: app,
	}

	tw.table = widget.NewTable(
		tw.length,
		tw.createCell,
		tw.updateCell,
	)

	// Устанавливаем минимальную ширину для каждой колонки
	// При изменении размера окна колонки будут расширяться пропорционально
	tw.table.SetColumnWidth(0, 300) // Wallet
	tw.table.SetColumnWidth(1, 200) // Twitter
	tw.table.SetColumnWidth(2, 200) // Telegram
	tw.table.SetColumnWidth(3, 100) // Analysis

	return tw
}

// length возвращает количество строк и колонок
func (tw *tableWidget) length() (int, int) {
	rows := len(tw.app.rows)
	if rows == 0 {
		return 1, 4 // Показываем хотя бы заголовки
	}
	return rows + 1, 4 // +1 для заголовка
}

// createCell создает шаблон ячейки
func (tw *tableWidget) createCell() fyne.CanvasObject {
	btn := widget.NewButton("", func() {})
	btn.Importance = widget.LowImportance
	return btn
}

// updateCell обновляет содержимое ячейки
func (tw *tableWidget) updateCell(cell widget.TableCellID, obj fyne.CanvasObject) {
	button := obj.(*widget.Button)

	// Заголовки
	if cell.Row == 0 {
		headers := []string{"Wallet", "Twitter", "Telegram", "Analysis"}
		button.SetText(headers[cell.Col])
		button.Importance = widget.HighImportance
		button.Disable()
		return
	}

	button.Importance = widget.LowImportance

	// Проверяем, есть ли данные
	rowIndex := cell.Row - 1
	if rowIndex >= len(tw.app.rows) {
		button.SetText("")
		button.Disable()
		return
	}

	row := tw.app.rows[rowIndex]
	button.Enable()

	// Обновляем содержимое в зависимости от колонки
	switch cell.Col {
	case 0: // Wallet
		button.SetText(row.Wallet)
		button.OnTapped = func() {
			tw.app.window.Clipboard().SetContent(row.Wallet)
		}

	case 1: // Twitter
		button.SetText(row.Twitter)
		button.OnTapped = func() {
			tw.app.window.Clipboard().SetContent(row.Twitter)
		}

	case 2: // Telegram
		button.SetText(row.TgUsername)
		button.OnTapped = func() {
			tw.app.window.Clipboard().SetContent(row.TgUsername)
		}

	case 3: // Analysis
		button.SetText("View")
		button.OnTapped = func() {
			showAnalysisWindow(tw.app, row)
		}
	}
}

// refresh обновляет таблицу
func (tw *tableWidget) refresh() {
	tw.table.Refresh()
}

// render возвращает виджет для отображения
func (tw *tableWidget) render() fyne.CanvasObject {
	return tw.table
}
