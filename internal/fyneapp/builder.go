package fyneapp

// BuildRows собирает строки из отдельных массивов данных
func BuildRows(
	wallets []string,
	twitters []string,
	tgUsernames []string,
	texts []string,
) []Row {
	// Определяем максимальную длину
	maxLen := len(wallets)
	if len(twitters) > maxLen {
		maxLen = len(twitters)
	}
	if len(tgUsernames) > maxLen {
		maxLen = len(tgUsernames)
	}
	if len(texts) > maxLen {
		maxLen = len(texts)
	}

	rows := make([]Row, maxLen)

	for i := 0; i < maxLen; i++ {
		row := Row{
			PasteTitle: "View",
		}

		if i < len(wallets) {
			row.Wallet = wallets[i]
		}

		if i < len(twitters) {
			row.Twitter = twitters[i]
		}

		if i < len(tgUsernames) {
			row.TgUsername = tgUsernames[i]
		}

		if i < len(texts) {
			row.PasteText = texts[i]
		}

		rows[i] = row
	}

	return rows
}
