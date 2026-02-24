package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"BackUper/internal/config"
	"BackUper/internal/logging"
	"BackUper/internal/receiver"
	"BackUper/internal/sender"
	"BackUper/internal/tray"
)

func main() {
	logPath, err := logging.Init("BackUper", 7)
	if err != nil {
		// last resort: keep stderr logging
		log.Printf("failed to init file logging: %v", err)
	} else {
		defer logging.Close()
	}

	printStartupBanner(logPath, err == nil)
	printStartupLoading()

	cfg, cfgPath, cfgErr := config.LoadOrCreate("BackUper")
	if cfgErr != nil {
		log.Printf("failed to load config: %v", cfgErr)
	}
	printConfigSummary(cfg, cfgPath, cfgErr == nil)

	reader := bufio.NewReader(os.Stdin)
	var stopServer func()
	serverRunning := false

	for {
		printMainMenu()
		choice, err := reader.ReadString('\n')
		if err != nil {
			fmt.Println("Ошибка ввода:", err)
			continue
		}
		choice = strings.TrimSpace(choice)

		switch choice {
		case "1":
			fmt.Println()
			fmt.Println("Клиент (приемник) запускается на localhost:9000")
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
			fmt.Println("Клиент (приемник) запущен в фоне на localhost:9000")
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
				RootFolder:  promptDefault(reader, "Каталог для архивации", defaultHome(cfg)),
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
			fmt.Println("Enter - вернуться в меню")
			reader.ReadString('\n')

		case "5":
			newCfg, ok := configureSchedule(reader, cfg)
			if !ok {
				fmt.Println("Отменено.")
				fmt.Println("Enter - вернуться в меню")
				reader.ReadString('\n')
				continue
			}
			cfg = newCfg
			if cfgErr == nil {
				if err := config.Save(cfgPath, cfg); err != nil {
					log.Printf("Ошибка сохранения конфигурации: %v", err)
					fmt.Println("Enter - вернуться в меню")
					reader.ReadString('\n')
					continue
				}
				fmt.Println("Настройки сохранены.")
			} else {
				log.Printf("Конфигурация не сохранена из-за ошибки загрузки: %v", cfgErr)
				fmt.Println("Enter - вернуться в меню")
				reader.ReadString('\n')
				continue
			}

			fmt.Println()
			printConfigSummary(cfg, cfgPath, true)
			fmt.Println("Программа сворачивается в трей.")
			fmt.Println("Откройте из трея, чтобы вернуться.")
			tray.HideConsole()
			tray.Run(func() {
				tray.ShowConsole()
			}, func() {
				os.Exit(0)
			})
			fmt.Println()
			fmt.Println("Восстановлено из трея.")
			fmt.Println("Enter - вернуться в меню")
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

func printMainMenu() {
	fmt.Println()
	fmt.Println("+------------------------------------+")
	fmt.Println("|           BackUper - Menu          |")
	fmt.Println("+------------------------------------+")
	fmt.Println("| 1) Клиент  - запустить приемник    |")
	fmt.Println("| 2) Клиент  - запустить в фоне      |")
	fmt.Println("| 3) Клиент  - остановить (в фоне)   |")
	fmt.Println("| 4) Сервер  - отправить архив       |")
	fmt.Println("| 5) Настройки - расписание/путь     |")
	fmt.Println("| 0) Выход                           |")
	fmt.Println("+------------------------------------+")
	fmt.Print("Выбор: ")
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

func printStartupBanner(logPath string, logOk bool) {
	fmt.Println()
	fmt.Println("+------------------------------------+")
	fmt.Println("|         BackUper - Start           |")
	fmt.Println("+------------------------------------+")
	if logOk {
		fmt.Printf("| Log file: %-25s |\n", trimToWidth(logPath, 25))
		fmt.Println("| Log retention: 7 days              |")
	} else {
		fmt.Println("| Log file: initialization failed    |")
	}
	fmt.Println("+------------------------------------+")
	fmt.Println()
}

func printStartupLoading() {
	start := time.Now()
	for time.Since(start) < 1200*time.Millisecond {
		fmt.Print(".")
		time.Sleep(200 * time.Millisecond)
	}
	fmt.Println()
}

func trimToWidth(s string, width int) string {
	if len(s) <= width {
		return s
	}
	if width <= 3 {
		return s[:width]
	}
	return s[:width-3] + "..."
}

func printConfigSummary(cfg config.Config, cfgPath string, ok bool) {
	fmt.Println("+------------------------------------+")
	fmt.Println("|         BackUper - Config          |")
	fmt.Println("+------------------------------------+")
	if ok {
		fmt.Printf("Path: %s\n", cfgPath)
		fmt.Printf("Home: %s\n", defaultHome(cfg))
		fmt.Printf("Time: %s\n", defaultSchedule(cfg))
	} else {
		fmt.Println("Config: failed to load")
	}
	fmt.Println("+------------------------------------+")
	fmt.Println()
}

func configureSchedule(r *bufio.Reader, cfg config.Config) (config.Config, bool) {
	fmt.Println()
	fmt.Println("Настройки:")

	for {
		cfg.HomeDir = promptDefault(r, "Домашний каталог (откуда копировать)", defaultHome(cfg))
		if isValidDir(cfg.HomeDir) {
			break
		}
		fmt.Println("Каталог не найден или недоступен.")
	}

	for {
		cfg.ScheduleTime = promptDefault(r, "Время запуска (HH:MM)", defaultSchedule(cfg))
		if isValidHHMM(cfg.ScheduleTime) {
			break
		}
		fmt.Println("Неверный формат времени. Пример: 03:00")
	}

	fmt.Println()
	fmt.Println("Проверьте данные:")
	fmt.Printf("  Home: %s\n", cfg.HomeDir)
	fmt.Printf("  Time: %s\n", cfg.ScheduleTime)
	if !promptYesNo(r, "Сохранить? (y/n)") {
		return cfg, false
	}

	fmt.Println()
	return cfg, true
}

func defaultHome(cfg config.Config) string {
	if strings.TrimSpace(cfg.HomeDir) == "" {
		return "C:\\Users\\...\\test"
	}
	return cfg.HomeDir
}

func defaultSchedule(cfg config.Config) string {
	if strings.TrimSpace(cfg.ScheduleTime) == "" {
		return "03:00"
	}
	return cfg.ScheduleTime
}

func promptYesNo(r *bufio.Reader, label string) bool {
	fmt.Printf("%s ", label)
	text, _ := r.ReadString('\n')
	text = strings.TrimSpace(strings.ToLower(text))
	return text == "y" || text == "yes" || text == "д" || text == "да"
}

func isValidDir(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}

func isValidHHMM(s string) bool {
	if len(s) != 5 || s[2] != ':' {
		return false
	}
	h, err1 := strconv.Atoi(s[:2])
	m, err2 := strconv.Atoi(s[3:])
	if err1 != nil || err2 != nil {
		return false
	}
	return h >= 0 && h <= 23 && m >= 0 && m <= 59
}
