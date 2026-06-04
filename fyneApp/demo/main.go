package main

import (
	"arkham_checker/fyneapp"
)

func main() {
	// Пример данных
	wallets := []string{
		"0x1234567890abcdef1234567890abcdef12345678",
		"0xabcdefabcdefabcdefabcdefabcdefabcdefabcd",
		"0x9876543210fedcba9876543210fedcba98765432",
	}

	twitters := []string{
		"@cryptouser1",
		"@blockchain_dev",
		"@web3_enthusiast",
	}

	tgUsernames := []string{
		"crypto_user_1",
		"blockchain_developer",
		"web3enthusiast",
	}

	texts := []string{
		"Analysis for wallet 1:\nThis wallet shows significant activity in DeFi protocols.\nTotal transactions: 150\nAverage transaction value: 0.5 ETH",
		"Analysis for wallet 2:\nThis wallet is primarily used for NFT trading.\nTotal NFT purchases: 45\nEstimated portfolio value: 12.5 ETH",
		"Analysis for wallet 3:\nThis wallet has minimal activity.\nTotal transactions: 8\nLast activity: 30 days ago",
	}

	// Собираем строки
	rows := fyneapp.BuildRows(wallets, twitters, tgUsernames, texts)

	// Запускаем GUI
	fyneapp.Run(rows)
}
