// hash_password.go
package main

import (
	"fmt"
	"log"

	"golang.org/x/crypto/bcrypt"
)

func main() {
	// !!! ВПИШИТЕ ВАШ НОВЫЙ ПАРОЛЬ СЮДА !!!
	password := []byte("MyNewStrongPassword123")

	// Генерируем хеш. DefaultCost = 10, это стандарт.
	hashedPassword, err := bcrypt.GenerateFromPassword(password, bcrypt.DefaultCost)
	if err != nil {
		log.Fatalf("Ошибка при генерации хеша: %v", err)
	}

	// Печатаем готовый хеш в консоль
	fmt.Println(string(hashedPassword))
}
