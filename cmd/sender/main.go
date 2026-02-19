package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"BackUper/internal/receiver"
	"BackUper/internal/sender"
)

func main() {
	reader := bufio.NewReader(os.Stdin)
	var stopServer func()
	serverRunning := false

	for {
		fmt.Println()
		fmt.Println("╔══════════════════════════════════════╗")
		fmt.Println("║           BackUper — Меню            ║")
		fmt.Println("╠══════════════════════════════════════╣")
		fmt.Println("║ 1) Клиент  — запустить приёмник      ║")
		fmt.Println("║ 2) Клиент  — запустить в фоне        ║")
		fmt.Println("║ 3) Клиент  — остановить (если в фоне)║")
		fmt.Println("║ 4) Сервер  — отправить архив         ║")
		fmt.Println("║ 0) Выход                             ║")
		fmt.Println("╚══════════════════════════════════════╝")
		fmt.Print("Выбор: ")

		choice, err := reader.ReadString('\n')
		if err != nil {
			fmt.Println("Ошибка ввода:", err)
			continue
		}
		choice = strings.TrimSpace(choice)

		switch choice {
		case "1":
			fmt.Println()
			fmt.Println("Клиент (приёмник) запускается на localhost:9000")
			fmt.Println("Для остановки нажмите Ctrl+C")
			fmt.Println()
			receiver.Run()
			return

		case "2":
			if serverRunning {
				fmt.Println("Клиент уже запущен в фоне.")
				continue
			}
			stopServer = receiver.RunWithShutdown()
			serverRunning = true
			fmt.Println()
			fmt.Println("Клиент (приёмник) запущен в фоне на localhost:9000")
			fmt.Println("Выберите пункт 3 для остановки.")
			fmt.Println()

		case "3":
			if !serverRunning {
				fmt.Println("Клиент не запущен в фоне.")
				continue
			}
			fmt.Println("Остановка клиента...")
			stopServer()
			serverRunning = false
			fmt.Println("Клиент остановлен.")

		case "4":
			opts := sender.SenderOptions{
				RootFolder:  promptDefault(reader, "Каталог для архивации", "C:\\Users\\...\\test"),
				ArchivePath: promptDefault(reader, "Путь к архиву", "C:\\Users\\...\\backup.zip"),
				Addr:        promptDefault(reader, "Адрес клиента (host:port)", "localhost:9000"),
				APIKey:      promptDefault(reader, "API ключ", "test_key"),
				Name:        promptDefault(reader, "Имя задания", "backup"),
				MaxRetries:  mustAtoi(promptDefault(reader, "Число попыток", "5")),
			}

			if err := sender.BuildAndSendArchive(opts); err != nil {
				log.Printf("Ошибка отправки: %v", err)
			} else {
				fmt.Println("Отправлено успешно.")
			}
			fmt.Println("Enter — вернуться в меню")
			reader.ReadString('\n')

		case "0":
			if serverRunning {
				fmt.Println("Остановка клиента...")
				stopServer()
			}
			fmt.Println("Выход.")
			return

		default:
			fmt.Println("Неверный выбор. Введите 1, 2, 3, 4 или 0.")
		}
	}
}

func promptDefault(r *bufio.Reader, label, def string) string {
	fmt.Printf("  %s [%s]: ", label, def)
	text, _ := r.ReadString('\n')
	text = strings.TrimSpace(text)
	if text == "" {
		return def
	}
	return text
}

func mustAtoi(s string) int {
	v, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		return 5
	}
	return v
}
