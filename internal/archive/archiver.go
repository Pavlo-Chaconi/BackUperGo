package archive

import (
	"archive/zip"
	"io"
	"os"
	"path/filepath"

	"BackUper/internal/hash"
)

func CreateZipArchive(archivePath string, filesToArchive []string) (size int64, sha256Hex string, err error) {
	if err := os.MkdirAll(filepath.Dir(archivePath), 0755); err != nil {
		return 0, "", err
	}
	//Тут сжатие данных для ZIP, в будущем попробуем другие форматы сжатия и архивации
	zipFile, err := os.Create(archivePath)
	if err != nil {
		return 0, "", err
	}
	defer zipFile.Close()

	zipWriter := zip.NewWriter(zipFile)
	defer zipWriter.Close()

	for _, filePath := range filesToArchive {
		file, err := os.Open(filePath)
		if err != nil {
			return 0, "", err
		}

		fileInfo, err := file.Stat()
		if err != nil {
			return 0, "", err
		}

		header, err := zip.FileInfoHeader(fileInfo)
		if err != nil {
			return 0, "", err
		}
		header.Name = fileInfo.Name()

		writer, err := zipWriter.CreateHeader(header)
		if err != nil {
			return 0, "", err
		}

		_, err = io.Copy(writer, file)
		file.Close()
		if err != nil {
			return 0, "", err

		}
	}

	info, err := os.Stat(archivePath)
	if err != nil {
		return 0, "", err
	}

	size = info.Size()

	sha256Hex, err = hash.SHA256File(archivePath)
	if err != nil {
		return 0, "", err
	}

	return size, sha256Hex, nil
}
