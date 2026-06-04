package fyneapp

import (
	"fmt"
	"path/filepath"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

// toolbarWidget представляет панель инструментов
type toolbarWidget struct {
	app             *App
	currentFilePath string // Путь к текущему открытому файлу
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
	if len(tw.app.rows) > 0 {
		// Спрашиваем подтверждение если есть данные
		dialog.ShowConfirm("Clear Data", "Are you sure you want to clear all data?", func(confirmed bool) {
			if confirmed {
				tw.app.Clear()
				tw.currentFilePath = ""
				tw.app.window.SetTitle("Wallet Checker")
			}
		}, tw.app.window)
	} else {
		tw.app.Clear()
		tw.currentFilePath = ""
	}
}

// onOpen обработчик кнопки Open
func (tw *toolbarWidget) onOpen() {
	sessions, err := GetSessions()
	if err != nil {
		dialog.ShowError(fmt.Errorf("Error loading sessions: %w", err), tw.app.window)
		return
	}

	if len(sessions) == 0 {
		dialog.ShowInformation("No Sessions", "No saved sessions found in data/app-sessions/", tw.app.window)
		return
	}

	// Создаем список для выбора
	list := widget.NewList(
		func() int { return len(sessions) },
		func() fyne.CanvasObject {
			return widget.NewLabel("")
		},
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			obj.(*widget.Label).SetText(sessions[id])
		},
	)

	// Обработчик выбора сессии
	list.OnSelected = func(id widget.ListItemID) {
		sessionFile := sessions[id]

		session, err := LoadSession(sessionFile)
		if err != nil {
			dialog.ShowError(fmt.Errorf("Error loading session: %w", err), tw.app.window)
			return
		}

		tw.app.LoadRows(session.Rows)
		tw.currentFilePath = sessionFile
		tw.app.window.SetTitle(fmt.Sprintf("Wallet Checker - %s", session.Name))

		dialog.ShowInformation("Success", fmt.Sprintf("Loaded session: %s", session.Name), tw.app.window)
	}

	// Показываем диалог со списком
	dialogWindow := dialog.NewCustom("Open Session", "Cancel", container.NewBorder(
		widget.NewLabel("Select a session to load:"),
		nil, nil, nil,
		container.NewScroll(list),
	), tw.app.window)

	dialogWindow.Resize(fyne.NewSize(400, 300))
	dialogWindow.Show()
}

// onSave обработчик кнопки Save
func (tw *toolbarWidget) onSave() {
	if len(tw.app.rows) == 0 {
		dialog.ShowInformation("No Data", "No data to save", tw.app.window)
		return
	}

	// Если уже есть путь к файлу, сохраняем туда
	if tw.currentFilePath != "" {
		session, err := LoadSession(tw.currentFilePath)
		if err != nil {
			// Создаем новую сессию если не удалось загрузить
			session = &Session{
				Name: filepath.Base(tw.currentFilePath),
				Rows: tw.app.GetRows(),
			}
		} else {
			// Обновляем существующую
			session.Rows = tw.app.GetRows()
		}

		if err := SaveSession(tw.currentFilePath, session); err != nil {
			dialog.ShowError(fmt.Errorf("Error saving session: %w", err), tw.app.window)
			return
		}

		dialog.ShowInformation("Success", fmt.Sprintf("Session saved: %s", session.Name), tw.app.window)
	} else {
		// Если пути нет, вызываем Save As
		tw.onSaveAs()
	}
}

// onSaveAs обработчик кнопки Save As
func (tw *toolbarWidget) onSaveAs() {
	if len(tw.app.rows) == 0 {
		dialog.ShowInformation("No Data", "No data to save", tw.app.window)
		return
	}

	// Создаем поле ввода для имени сессии
	nameEntry := widget.NewEntry()
	nameEntry.SetPlaceHolder("Enter session name")

	// Генерируем имя по умолчанию
	defaultName := fmt.Sprintf("session_%s", time.Now().Format("2006-01-02_15-04-05"))
	nameEntry.SetText(defaultName)

	// Создаем форму
	formItems := []*widget.FormItem{
		widget.NewFormItem("Session Name", nameEntry),
	}

	dialog.ShowForm("Save Session As", "Save", "Cancel", formItems, func(confirmed bool) {
		if !confirmed {
			return
		}

		sessionName := nameEntry.Text
		if sessionName == "" {
			dialog.ShowError(fmt.Errorf("Session name cannot be empty"), tw.app.window)
			return
		}

		// Добавляем расширение .json если его нет
		if filepath.Ext(sessionName) != ".json" {
			sessionName += ".json"
		}

		session := &Session{
			Name: sessionName,
			Rows: tw.app.GetRows(),
		}

		if err := SaveSession(sessionName, session); err != nil {
			dialog.ShowError(fmt.Errorf("Error saving session: %w", err), tw.app.window)
			return
		}

		tw.currentFilePath = sessionName
		tw.app.window.SetTitle(fmt.Sprintf("Wallet Checker - %s", sessionName))

		dialog.ShowInformation("Success", fmt.Sprintf("Session saved: %s", sessionName), tw.app.window)
	}, tw.app.window)
}
