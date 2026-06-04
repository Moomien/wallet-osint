package fyneapp

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// tableWidget управляет таблицей данных
type tableWidget struct {
	app   *App
	table *widget.Table
}

// tappableLabel - Label с поддержкой кликов
type tappableLabel struct {
	widget.Label
	onTapped func()
}

func newTappableLabel(text string, onTapped func()) *tappableLabel {
	label := &tappableLabel{onTapped: onTapped}
	label.SetText(text)
	label.Truncation = fyne.TextTruncateEllipsis
	label.ExtendBaseWidget(label)
	return label
}

func (t *tappableLabel) Tapped(*fyne.PointEvent) {
	if t.onTapped != nil {
		t.onTapped()
	}
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

	// Устанавливаем фиксированную ширину для каждой колонки
	tw.table.SetColumnWidth(0, 350) // Wallet
	tw.table.SetColumnWidth(1, 180) // Twitter
	tw.table.SetColumnWidth(2, 180) // Telegram
	tw.table.SetColumnWidth(3, 80)  // Analysis

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
	// Создаем контейнер с label
	label := widget.NewLabel("")
	label.Truncation = fyne.TextTruncateEllipsis
	return container.NewMax(label)
}

// updateCell обновляет содержимое ячейки
func (tw *tableWidget) updateCell(cell widget.TableCellID, obj fyne.CanvasObject) {
	cont := obj.(*fyne.Container)

	// Заголовки
	if cell.Row == 0 {
		headers := []string{"Wallet", "Twitter", "Telegram", "Analysis"}
		label := widget.NewLabel(headers[cell.Col])
		label.TextStyle = fyne.TextStyle{Bold: true}
		label.Alignment = fyne.TextAlignCenter
		label.Truncation = fyne.TextTruncateEllipsis
		cont.Objects = []fyne.CanvasObject{label}
		cont.Refresh()
		return
	}

	// Проверяем, есть ли данные
	rowIndex := cell.Row - 1
	if rowIndex >= len(tw.app.rows) {
		label := widget.NewLabel("")
		cont.Objects = []fyne.CanvasObject{label}
		cont.Refresh()
		return
	}

	row := tw.app.rows[rowIndex]

	// Обновляем содержимое в зависимости от колонки
	switch cell.Col {
	case 0: // Wallet - кликабельный для копирования
		label := newTappableLabel(row.Wallet, func() {
			tw.app.window.Clipboard().SetContent(row.Wallet)
		})
		cont.Objects = []fyne.CanvasObject{label}

	case 1: // Twitter - кликабельный для копирования
		label := newTappableLabel(row.Twitter, func() {
			tw.app.window.Clipboard().SetContent(row.Twitter)
		})
		cont.Objects = []fyne.CanvasObject{label}

	case 2: // Telegram - кликабельный для копирования
		label := newTappableLabel(row.TgUsername, func() {
			tw.app.window.Clipboard().SetContent(row.TgUsername)
		})
		cont.Objects = []fyne.CanvasObject{label}

	case 3: // Analysis - кнопка для открытия
		if row.PasteText != "" {
			btn := widget.NewButton("View", func() {
				showAnalysisWindow(tw.app, row)
			})
			btn.Importance = widget.LowImportance
			cont.Objects = []fyne.CanvasObject{btn}
		} else {
			label := widget.NewLabel("-")
			label.Alignment = fyne.TextAlignCenter
			cont.Objects = []fyne.CanvasObject{label}
		}
	}

	cont.Refresh()
}

// refresh обновляет таблицу
func (tw *tableWidget) refresh() {
	tw.table.Refresh()
}

// render возвращает виджет для отображения
func (tw *tableWidget) render() fyne.CanvasObject {
	return tw.table
}
