package main

import (
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	_ "github.com/mattn/go-sqlite3"
)

func main() {
	updateFlag := flag.Bool("update", false, "Обновить курсы валют и сохранить в базу данных")
	// updateCBR := flag.Bool("update-cbr", false, "Обновить курсы валют ЦБРФ и сохранить в базу данных")
	flag.Parse()

	db, err := sql.Open("sqlite3", "data/currency.db")
	if err != nil {
		log.Fatalf("Ошибка подключения к базе данных: %s", err)
	}
	defer db.Close()
	// Создаем таблицу для хранения курсов валют
	createTable(db)

	if *updateFlag {
		err := saveBitcoinPrice(db)
		if err != nil {
			log.Fatalf("Ошибка при обновлении цены BTC: %s", err)
		}
		log.Println("Цена BTC успешно обновлена.")

		// Обновление курсов ЦБРФ
		if err := updateCurrencyRatesFromCBR(db); err != nil {
			log.Fatalf("Ошибка обновления курсов ЦБРФ: %s", err)
		}

		os.Exit(0) // Завершаем выполнение программы после обновления
	}

	// Получаем токен бота из переменной окружения
	token := os.Getenv("TELEGRAM_BOT_TOKEN")
	if token == "" {
		log.Fatal("TELEGRAM_BOT_TOKEN не задан")
	}

	fmt.Println("token:", token)
	// Создаем нового бота
	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		log.Fatalf("Ошибка при создании бота: %s", err)
	}

	// Включаем режим отладки
	bot.Debug = true
	log.Printf("Авторизован как %s", bot.Self.UserName)

	// Настраиваем получение обновлений
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	updates := bot.GetUpdatesChan(u)

	// Обрабатываем входящие обновления
	for update := range updates {
		if update.Message == nil { // Игнорируем не-сообщения
			continue
		}

		// Логируем входящее сообщение
		log.Printf("[%s] %s", update.Message.From.UserName, update.Message.Text)

		// Обрабатываем команды
		command := update.Message.Text
		switch {
		case strings.HasPrefix(command, "/start"):
			msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Привет! Я твой Telegram-бот.")
			bot.Send(msg)
		case strings.HasPrefix(command, "/help"):
			msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Я могу отвечать на команды /start и /help.")
			bot.Send(msg)
		case strings.HasPrefix(command, "/rate"):
			// // Извлекаем код валюты
			// parts := strings.Fields(command)
			// if len(parts) < 2 {
			// 	msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Укажите код валюты. Например: /rate USD.")
			// 	bot.Send(msg)
			// 	continue
			// }

			// currency := strings.ToUpper(parts[1])
			// rate, err := getCurrencyRate(db, currency)
			// if err != nil {
			// 	msg := tgbotapi.NewMessage(update.Message.Chat.ID, fmt.Sprintf("Не удалось найти курс для валюты %s.", currency))
			// 	bot.Send(msg)
			// } else {
			// 	msg := tgbotapi.NewMessage(update.Message.Chat.ID, fmt.Sprintf("Курс %s: %.2f", currency, rate))
			// 	bot.Send(msg)
			// }

			rates, err := getLatestCurrenciesRate(db)
			if err != nil {
				msg := tgbotapi.NewMessage(update.Message.Chat.ID, fmt.Sprintf("Не удалось найти курсы"))
				bot.Send(msg)
			} else {
				message := "Котировки: "
				btc := 0.0
				usd := 0.0
				eur := 0.0
				for code, r := range rates {
					if code == "BTC" {
						btc = r.Rate
					} else if code == "USD" {
						usd = r.Rate
					} else if code == "EUR" {
						eur = r.Rate
					}
				}

				// message += fmt.Sprintf("%s: %.2f (обновлено %s) ", code, r.Rate, r.Date)
				// message += fmt.Sprintf("BTC: %.2f, USD: %.2f, EUR: %.2f", btc, usd, eur)
				message += fmt.Sprintf("₿ $%.2f  💵 %.2f₽  💶 %.2f₽", btc, usd, eur)

				msg := tgbotapi.NewMessage(update.Message.Chat.ID, message)
				bot.Send(msg)
			}

		case strings.HasPrefix(command, "/updatebtc"):
			err := saveBitcoinPrice(db)
			if err != nil {
				msg := tgbotapi.NewMessage(update.Message.Chat.ID, fmt.Sprintf("Ошибка обновления цены BTC: %v", err))
				bot.Send(msg)
			} else {
				msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Цена BTC успешно обновлена.")
				bot.Send(msg)
			}

		case strings.HasPrefix(command, "/updatecbr"):
			err := updateCurrencyRatesFromCBR(db)
			if err != nil {
				msg := tgbotapi.NewMessage(update.Message.Chat.ID, fmt.Sprintf("Ошибка обновления курсов ЦБРФ: %v", err))
				bot.Send(msg)
			} else {
				msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Курсы USD и EUR успешно обновлены из ЦБРФ.")
				bot.Send(msg)
			}

		default:
			msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Извините, я вас не понимаю. Попробуйте /start или /help.")
			bot.Send(msg)
		}
	}
}

// createTable создает таблицу для хранения курсов валют
func createTable(db *sql.DB) {
	query := `
	CREATE TABLE IF NOT EXISTS currency (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		code 	TEXT NOT NULL,
		nominal INTEGER NOT NULL,
		rate 	REAL NOT NULL,
		date 	TEXT NOT NULL,
		UNIQUE(code, date) -- Уникальный индекс по code и date
	);`
	_, err := db.Exec(query)
	if err != nil {
		log.Fatalf("Ошибка при создании таблицы: %s", err)
	}
}

// // seedData добавляет пример данных
// func seedData(db *sql.DB) {
// 	now := time.Now().Format("2006-01-02")
// 	_, err := db.Exec(`
// 	INSERT INTO currency (code, rate, date) VALUES
// 		('USD', 75.50, ?),
// 		('EUR', 82.30, ?);
// 	`, now, now)
// 	if err != nil {
// 		log.Fatalf("Ошибка при добавлении данных: %s", err)
// 	}
// }

// getCurrencyRate возвращает курс валюты по коду
func getCurrencyRate(db *sql.DB, code string) (float64, error) {
	var rate float64
	err := db.QueryRow("SELECT rate FROM currency WHERE code = ?", code).Scan(&rate)
	if err != nil {
		return 0, err
	}
	return rate, nil
}

// getCurrencyRate возвращает курс валюты по коду
func getCurrenciesRate(db *sql.DB) (float64, error) {
	var rate float64
	err := db.QueryRow("SELECT rate FROM currency").Scan(&rate)
	if err != nil {
		return 0, err
	}
	return rate, nil
}

// getLatestCurrencyRate возвращает последний курс валюты по коду
func getLatestCurrencyRate(db *sql.DB, code string) (float64, string, error) {
	var rate float64
	var date string
	err := db.QueryRow(`
		SELECT rate, date FROM currency
		WHERE code = ?
		ORDER BY date DESC
		LIMIT 1;
	`, code).Scan(&rate, &date)
	if err != nil {
		return 0, "", err
	}
	return rate, date, nil
}

// getLatestCurrenciesRate возвращает список всех последних значений курсов валют
func getLatestCurrenciesRate(db *sql.DB) (map[string]struct {
	Rate float64
	Date string
}, error) {
	// Карта для хранения результатов
	results := make(map[string]struct {
		Rate float64
		Date string
	})

	// SQL-запрос для получения последних курсов
	query := `
		SELECT code, rate, date 
		FROM currency
		WHERE date IN (
			SELECT MAX(date) 
			FROM currency 
			GROUP BY code
		)
		ORDER BY code;
	`

	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// Проходим по всем строкам результата
	for rows.Next() {
		var code string
		var rate float64
		var date string

		// Считываем данные строки
		err := rows.Scan(&code, &rate, &date)
		if err != nil {
			return nil, err
		}

		// Сохраняем в результат
		results[code] = struct {
			Rate float64
			Date string
		}{
			Rate: rate,
			Date: date,
		}
	}

	return results, nil
}

// saveBitcoinPrice получает цену биткойна из API Coinbase и сохраняет ее в базу данных
func saveBitcoinPrice(db *sql.DB) error {
	// URL API Coinbase
	url := "https://api.coinbase.com/v2/prices/BTC-USD/spot"

	// Делаем HTTP-запрос
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("ошибка при запросе к API: %v", err)
	}
	defer resp.Body.Close()

	// Проверяем статус ответа
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("неудачный запрос: статус %d", resp.StatusCode)
	}

	// Парсим JSON-ответ
	var result struct {
		Data struct {
			Base     string `json:"base"`
			Currency string `json:"currency"`
			Amount   string `json:"amount"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("ошибка при разборе JSON: %v", err)
	}

	// Преобразуем цену в float64
	var price float64
	if _, err := fmt.Sscanf(result.Data.Amount, "%f", &price); err != nil {
		return fmt.Errorf("ошибка при преобразовании цены: %v", err)
	}

	// Получаем текущую дату и время
	now := time.Now().Format("2006-01-02")

	// Сохраняем данные в базу
	query := `
		INSERT INTO currency (code, rate, date) 
		VALUES (?, ?, ?)
		ON CONFLICT(code, date) DO UPDATE SET
		rate = excluded.rate;
	`
	_, err = db.Exec(query, "BTC", price, now)
	if err != nil {
		return fmt.Errorf("ошибка при сохранении данных в базу: %v", err)
	}

	log.Printf("Успешно сохранена цена BTC: %.2f USD (дата: %s)", price, now)
	return nil
}
