package fyneapp

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
)

// Row представляет одну строку данных для отображения
type Row struct {
	Wallet     string
	Twitter    string
	TgUsername string
	PasteTitle string
	PasteText  string
}

// App содержит главное окно и данные приложения
type App struct {
	app     fyne.App
	window  fyne.Window
	rows    []Row
	table   *tableWidget
	toolbar *toolbarWidget
}

// NewApp создает новое приложение
func NewApp() *App {
	a := &App{
		app:  app.New(),
		rows: []Row{},
	}

	a.window = a.app.NewWindow("Wallet Checker")
	a.window.Resize(fyne.NewSize(1200, 800))

	// Создаем компоненты
	a.toolbar = newToolbar(a)
	a.table = newTableWidget(a)

	// Компонуем интерфейс
	content := container.NewBorder(
		a.toolbar.render(), // top
		nil,                // bottom
		nil,                // left
		nil,                // right
		a.table.render(),   // center
	)

	a.window.SetContent(content)

	return a
}

// LoadRows загружает данные в таблицу
func (a *App) LoadRows(rows []Row) {
	a.rows = rows
	a.table.refresh()
}

// Clear очищает таблицу
func (a *App) Clear() {
	a.rows = []Row{}
	a.table.refresh()
}

// Refresh перерисовывает интерфейс
func (a *App) Refresh() {
	a.table.refresh()
}

// GetRows возвращает текущие строки
func (a *App) GetRows() []Row {
	return a.rows
}

// ShowAndRun показывает окно и запускает приложение
func (a *App) ShowAndRun() {
	a.window.ShowAndRun()
}

// Window возвращает главное окно
func (a *App) Window() fyne.Window {
	return a.window
}
