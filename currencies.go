package main

import (
	"database/sql"
	"encoding/xml"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"golang.org/x/net/html/charset"
)

// Valute представляет структуру одной валюты из XML
type Valute struct {
	CharCode string `xml:"CharCode"`
	Nominal  int    `xml:"Nominal"`
	Value    string `xml:"Value"`
}

// ValCurs представляет корневую структуру XML
type ValCurs struct {
	Date   string   `xml:"Date,attr"`
	Valute []Valute `xml:"Valute"`
}

// updateCurrencyRatesFromCBR обновляет курсы USD и EUR в базе данных
func updateCurrencyRatesFromCBR(db *sql.DB) error {
	// URL API ЦБРФ
	url := "https://www.cbr.ru/scripts/XML_daily.asp"

	// Создаем новый HTTP-запрос
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Fatalf("Ошибка создания запроса: %v", err)
	}

	// Добавляем заголовок User-Agent
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")

	// Выполняем запрос
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("ошибка запроса к ЦБРФ: %v", err)
	}
	defer resp.Body.Close()

	// Проверяем статус ответа
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("неудачный запрос к ЦБРФ: статус %d", resp.StatusCode)
	}

	// Парсим XML
	var valCurs ValCurs
	dec := xml.NewDecoder(resp.Body)
	dec.CharsetReader = charset.NewReaderLabel
	if err := dec.Decode(&valCurs); err != nil {
		return fmt.Errorf("ошибка разбора XML: %v", err)
	}

	// Получаем дату из XML
	date, err := time.Parse("02.01.2006", valCurs.Date)
	if err != nil {
		return fmt.Errorf("ошибка разбора даты из XML: %v", err)
	}
	formattedDate := date.Format("2006-01-02")

	// Сохраняем курсы USD и EUR в базу данных
	currenciesToUpdate := []string{"USD", "EUR"}
	for _, valute := range valCurs.Valute {
		if contains(currenciesToUpdate, valute.CharCode) {
			// Преобразуем курс в float64
			cleanedValue := strings.Replace(valute.Value, ",", ".", 1)
			rate, err := strconv.ParseFloat(cleanedValue, 64)
			if err != nil {
				return fmt.Errorf("ошибка преобразования курса валюты %s: %v", valute.CharCode, err)
			}

			// Учитываем номинал
			rate /= float64(valute.Nominal)

			// Сохраняем в базу
			query := `
				INSERT INTO currency (code, rate, date)
				VALUES (?, ?, ?);
			`
			_, err = db.Exec(query, valute.CharCode, rate, formattedDate)
			if err != nil {
				return fmt.Errorf("ошибка сохранения курса %s в базу: %v", valute.CharCode, err)
			}

			log.Printf("Успешно сохранён курс %s: %.2f (дата: %s)", valute.CharCode, rate, formattedDate)
		}
	}

	return nil
}

// contains проверяет, содержится ли строка в списке
func contains(list []string, str string) bool {
	for _, v := range list {
		if v == str {
			return true
		}
	}
	return false
}
