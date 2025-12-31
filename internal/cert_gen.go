package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"log"
	"math/big"
	"net"
	"os"
	"time"

)

func main() {
	// Генерируем современный ключ P-256
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		log.Fatal(err)
	}

	// Настраиваем данные сертификата под твой IP
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{"Bank HelpDesk SSL"},
			CommonName:   "192.168.10.79",
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(10, 0, 0), // на 10 лет
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IPAddresses:           []net.IP{net.ParseIP("192.168.10.79"), net.ParseIP("127.0.0.1")},
	}

	// Создаем сам сертификат
	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		log.Fatal(err)
	}

	// 1. Сохраняем PUBLIC KEY (server.crt)
	certOut, _ := os.Create("server.crt")
	pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
	certOut.Close()

	// 2. Сохраняем PRIVATE KEY (server.key)
	keyOut, _ := os.Create("server.key")
	b, _ := x509.MarshalECPrivateKey(priv)
	pem.Encode(keyOut, &pem.Block{Type: "EC PRIVATE KEY", Bytes: b})
	keyOut.Close()

	log.Println("✅ ШАГ 1 ЗАВЕРШЕН: Файлы server.crt и server.key созданы в папке проекта!")
}
