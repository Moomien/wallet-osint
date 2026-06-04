package fyneapp

// Этот файл содержит примеры использования GUI
// Он не будет вызываться напрямую, а служит документацией

/*
Пример 1: Простой запуск с данными

func main() {
	wallets := []string{"0x123...", "0x456..."}
	twitters := []string{"@user1", "@user2"}
	tgUsernames := []string{"tg_user1", "tg_user2"}
	texts := []string{"Analysis text 1", "Analysis text 2"}

	rows := fyneapp.BuildRows(wallets, twitters, tgUsernames, texts)
	fyneapp.Run(rows)
}

Пример 2: Использование App напрямую

func main() {
	app := fyneapp.NewApp()

	// Загружаем данные
	rows := fyneapp.BuildRows(
		[]string{"0x789..."},
		[]string{"@user3"},
		[]string{"tg_user3"},
		[]string{"Analysis text 3"},
	)

	app.LoadRows(rows)
	app.ShowAndRun()
}

Пример 3: Динамическое обновление

func main() {
	app := fyneapp.NewApp()

	// Изначально пустая таблица
	app.Clear()

	// Позже добавляем данные
	go func() {
		time.Sleep(2 * time.Second)
		rows := fyneapp.BuildRows(
			[]string{"0xabc..."},
			[]string{"@user4"},
			[]string{"tg_user4"},
			[]string{"Analysis text 4"},
		)
		app.LoadRows(rows)
		app.Refresh()
	}()

	app.ShowAndRun()
}

Пример 4: Работа с сессиями

func main() {
	// Создаем данные
	rows := fyneapp.BuildRows(
		[]string{"0xdef..."},
		[]string{"@user5"},
		[]string{"tg_user5"},
		[]string{"Analysis text 5"},
	)

	// Сохраняем сессию
	session := &fyneapp.Session{
		Name: "my_session",
		Rows: rows,
	}
	err := fyneapp.SaveSession("sessions/my_session.json", session)
	if err != nil {
		panic(err)
	}

	// Загружаем сессию
	loadedSession, err := fyneapp.LoadSession("sessions/my_session.json")
	if err != nil {
		panic(err)
	}

	// Запускаем GUI с загруженными данными
	fyneapp.Run(loadedSession.Rows)
}
*/
