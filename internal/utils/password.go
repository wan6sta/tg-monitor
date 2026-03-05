package utils

import "fmt"

func PrintPassword(pass string) {
	if pass != "" {
		fmt.Printf("Пароль: %s\n", "****")
	} else {
		fmt.Printf("Пароль: %s\n", "не установлен")
	}
}
