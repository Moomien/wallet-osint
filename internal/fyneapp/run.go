package fyneapp

// Run создает и запускает GUI приложение с указанными данными
func Run(rows []Row) {
	app := NewApp()
	app.LoadRows(rows)
	app.ShowAndRun()
}
