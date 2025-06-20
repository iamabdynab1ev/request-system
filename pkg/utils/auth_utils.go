package utils

import (
	"golang.org/x/crypto/bcrypt"
	"fmt" 
)


func HashPassword(password string) (string, error) {
    bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
    if err != nil {
        return "", fmt.Errorf("не удалось хешировать пароль: %w", err)
    }
    return string(bytes), nil
}


func ComparePasswords(hashedPassword string, plainPassword string) error {
    return bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(plainPassword))
}