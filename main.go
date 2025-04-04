package main

import (
	"bufio"
	"bytes"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

func getEnvOrFail(key string) string {
	val := os.Getenv(key)
	if val == "" {
		log.Fatalf("❌ Переменная окружения %s не задана", key)
	}
	return val
}

var tlsUrl string = getEnvOrFail("TLS_URL")
var botToken string = getEnvOrFail("BOT_TOKEN")
var chatID string = getEnvOrFail("CHAT_ID")

func main() {

	// Загружаем уже сохранённые заголовки
	existing, err := loadTitlesSet("titles.txt")
	if err != nil {
		log.Fatal("Ошибка загрузки сохранённых заголовков:", err)
	}

	// Получаем HTML с сайта
	resp, err := http.Get(tlsUrl)
	if err != nil {
		log.Fatal("Ошибка запроса:", err)
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		log.Fatal("Ошибка парсинга HTML:", err)
	}

	// Ищем новые заголовки
	var newTitles []string
	doc.Find("h3").Each(func(i int, s *goquery.Selection) {
		title := strings.TrimSpace(s.Text())
		title = strings.ReplaceAll(title, "\n", "")
		title = strings.ReplaceAll(title, "\r", "")
		if title != "" {
			if _, found := existing[title]; !found {
				newTitles = append(newTitles, title)
				existing[title] = struct{}{}
			}
		}
	})

	// Выводим и сохраняем
	if len(newTitles) > 0 {
		fmt.Println("Новые заголовки:")
		for _, t := range newTitles {
			fmt.Println("→", t)
		}
		sendNewsToTelegram(newTitles)
		if err := appendTitlesToFile("titles.txt", newTitles); err != nil {
			log.Fatal("Ошибка сохранения заголовков:", err)
		}
	} else {
		fmt.Println("Новых заголовков нет.")
	}
}

// Чтение заголовков в Set
func loadTitlesSet(filename string) (map[string]struct{}, error) {
	set := make(map[string]struct{})

	file, err := os.Open(filename)
	if err != nil {
		if os.IsNotExist(err) {
			return set, nil
		}
		return nil, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		title := strings.TrimSpace(scanner.Text())
		if title != "" {
			set[title] = struct{}{}
		}
	}
	return set, scanner.Err()
}

// Добавление новых заголовков в файл
func appendTitlesToFile(filename string, titles []string) error {
	file, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	// Проход с конца
	for i := len(titles) - 1; i >= 0; i-- {
		line := fmt.Sprintf("%s\n", titles[i])
		if _, err := file.WriteString(line); err != nil {
			return err
		}
	}
	return nil
}

func sendNewsToTelegram(titles []string) error {
	if len(titles) == 0 {
		return nil
	}

	var builder strings.Builder
	builder.WriteString("🔥 *Обновления на сайте TLS ВЦ:*\n\n")

	for _, title := range titles {
		builder.WriteString(fmt.Sprintf("• %s\n\n", title))
	}

	message := builder.String()
	return sendToTelegramMessage(message)
}

func sendToTelegramMessage(message string) error {
	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", botToken)

	payload := fmt.Sprintf(`{
		"chat_id": "%s",
		"text": "%s",
		"parse_mode": "Markdown",
        "reply_markup": {
			"inline_keyboard": [
				[
					{
						"text": "🌐 Открыть сайт TLS",
						"url": "%s"
					}
				]
			]
		}
	}`, chatID, escapeTelegramMarkdown(message), tlsUrl)

	resp, err := http.Post(url, "application/json", bytes.NewBuffer([]byte(payload)))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

func escapeTelegramMarkdown(text string) string {
	replacer := strings.NewReplacer(
		"_", "\\_",
		"*", "\\*",
		"`", "\\`",
		"[", "\\[",
	)
	return replacer.Replace(text)
}
