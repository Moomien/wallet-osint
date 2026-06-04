package fyneapp

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// toolbarWidget представляет панель инструментов
type toolbarWidget struct {
	app *App
}

// newToolbar создает новую панель инструментов
func newToolbar(app *App) *toolbarWidget {
	return &toolbarWidget{
		app: app,
	}
}

// render создает и возвращает виджет панели инструментов
func (tw *toolbarWidget) render() *fyne.Container {
	newBtn := widget.NewButton("New", tw.onNew)
	openBtn := widget.NewButton("Open", tw.onOpen)
	saveBtn := widget.NewButton("Save", tw.onSave)
	saveAsBtn := widget.NewButton("Save As", tw.onSaveAs)

	return container.NewHBox(
		newBtn,
		openBtn,
		saveBtn,
		saveAsBtn,
	)
}

// onNew обработчик кнопки New
func (tw *toolbarWidget) onNew() {
	// TODO: Реализовать создание новой сессии
	tw.app.Clear()
}

// onOpen обработчик кнопки Open
func (tw *toolbarWidget) onOpen() {
	// TODO: Реализовать открытие сессии
	// sessions, err := GetSessions()
	// if err != nil {
	//     dialog.ShowError(err, tw.app.window)
	//     return
	// }
	// Показать диалог выбора сессии
}

// onSave обработчик кнопки Save
func (tw *toolbarWidget) onSave() {
	// TODO: Реализовать сохранение текущей сессии
	// session := &Session{
	//     Name: "current",
	//     Rows: tw.app.GetRows(),
	// }
	// err := SaveSession("current.json", session)
	// if err != nil {
	//     dialog.ShowError(err, tw.app.window)
	// }
}

// onSaveAs обработчик кнопки Save As
func (tw *toolbarWidget) onSaveAs() {
	// TODO: Реализовать сохранение сессии с новым именем
	// Показать диалог ввода имени файла
}
