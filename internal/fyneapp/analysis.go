package fyneapp

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// showAnalysisWindow открывает окно анализа для указанной строки
func showAnalysisWindow(app *App, row Row) {
	// Создаем новое окно
	analysisWindow := app.app.NewWindow("Analysis")
	analysisWindow.Resize(fyne.NewSize(800, 600))

	// Создаем многострочное текстовое поле в режиме только для чтения
	textEntry := widget.NewMultiLineEntry()
	textEntry.SetText(row.PasteText)
	textEntry.Wrapping = fyne.TextWrapWord

	// Делаем текст доступным для чтения, но не редактирования
	// В Fyne v2 можно использовать RichText для readonly, но MultiLineEntry
	// позволяет выделять и копировать текст

	// Создаем заголовок с информацией о строке
	header := widget.NewLabel("Wallet: " + row.Wallet)
	header.TextStyle = fyne.TextStyle{Bold: true}

	// Компонуем содержимое окна
	content := container.NewBorder(
		header,                         // top
		nil,                            // bottom
		nil,                            // left
		nil,                            // right
		container.NewScroll(textEntry), // center
	)

	analysisWindow.SetContent(content)
	analysisWindow.Show()
}
