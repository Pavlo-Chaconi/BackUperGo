package hash

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
)

// SHA256File вычисляет SHA256 хеш файла и возвращает его в hex-формате.
func SHA256File(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return "", err
	}

	return hex.EncodeToString(hasher.Sum(nil)), nil
}
