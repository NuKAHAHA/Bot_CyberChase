package team

import (
	"Cyber-chase/internal/models"
	"Cyber-chase/internal/pkg"
	"Cyber-chase/internal/service"
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/golang-jwt/jwt/v4"
	"github.com/google/uuid"
	"io"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

const (
	StateStart            = "start"
	StateEmail            = "email"
	StatePassword         = "password"
	StateMenu             = "menu"
	StateAnswer           = "answer"
	StateRegisterName     = "register_name"
	StateRegisterPass     = "register_pass"
	StateRegisterConfirm  = "register_confirm"
	StateWaitingGeo       = "waiting_geo"
	StateWaitingApprove   = "waiting_approve"
	StateReadyToGetTask   = "ready_to_get_task"
	StateTaskReceived     = "task_received"
	StateAllTasksComplete = "all_tasks_done"
)

// Сессия пользователя
type UserSession struct {
	State        string
	Email        string
	TeamID       string
	TaskID       string
	TempTeamName string
}

// TelegramBot структура для телеграм бота
type TelegramBot struct {
	bot         *tgbotapi.BotAPI
	teamService service.TeamService
	sessions    map[int64]*UserSession
}

// NewTelegramBot создает новый экземпляр телеграм бота
func NewTelegramBot(token string, teamService service.TeamService) (*TelegramBot, error) {
	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, err
	}

	return &TelegramBot{
		bot:         bot,
		teamService: teamService,
		sessions:    make(map[int64]*UserSession),
	}, nil
}

// Start запускает телеграм бота
func (b *TelegramBot) Start() error {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := b.bot.GetUpdatesChan(u)

	for update := range updates {
		if update.Message != nil {
			b.handleMessage(update.Message)
		} else if update.CallbackQuery != nil {
			b.handleCallback(update.CallbackQuery)
		}
	}

	return nil // или возвращаем реальную ошибку, если есть
}

// getSession возвращает сессию пользователя, создает новую если не существует
func (b *TelegramBot) getSession(chatID int64) *UserSession {
	session, exists := b.sessions[chatID]
	if !exists {
		session = &UserSession{
			State: StateStart,
		}
		b.sessions[chatID] = session
	}
	return session
}

// handleMessage обрабатывает сообщения от пользователя
func (b *TelegramBot) handleMessage(message *tgbotapi.Message) {
	session := b.getSession(message.Chat.ID)

	switch session.State {
	case StateStart:
		if message.Text == "/start" {
			msg := "👋 Добро пожаловать!\nВыберите действие:\n1. Войти - введите email\n2. Регистрация - введите /register"
			b.sendMessage(message.Chat.ID, msg)
		} else if message.Text == "/register" {
			b.sendMessage(message.Chat.ID, "Введите название вашей команды:")
			session.State = StateRegisterName
		} else {
			session.Email = strings.TrimSpace(message.Text)
			b.checkEmailAndProceed(message.Chat.ID, session)
		}
	case StateRegisterConfirm:
		response := strings.ToLower(strings.TrimSpace(message.Text))
		switch response {
		case "да", "yes":
			b.sendMessage(message.Chat.ID, "Введите название вашей команды:")
			session.State = StateRegisterName
		case "нет", "no":
			b.sendMessage(message.Chat.ID, "Хорошо, попробуйте ввести email снова:")
			session.State = StateStart
		default:
			b.sendMessage(message.Chat.ID, "Пожалуйста, ответьте 'да' или 'нет'")
		}
	case StateRegisterName:
		session.TempTeamName = message.Text
		b.sendMessage(message.Chat.ID, "Введите email для регистрации:")
		session.State = StateRegisterPass

	case StateRegisterPass:
		email := strings.TrimSpace(message.Text)
		if !isValidEmail(email) {
			b.sendMessage(message.Chat.ID, "❌ Неверный формат email. Попробуйте снова:")
			return
		}

		// Регистрируем команду
		err := b.teamService.RegisterTeam(email, session.TempTeamName)
		if err != nil {
			b.sendMessage(message.Chat.ID, "❌ Ошибка регистрации: "+err.Error())
			session.State = StateStart
			return
		}

		msg := fmt.Sprintf("✅ Регистрация успешна!\nВременный пароль отправлен на %s", email)
		b.sendMessage(message.Chat.ID, msg)
		session.State = StateStart

	case StateEmail:
		email := strings.TrimSpace(message.Text)
		session.Email = email
		b.sendMessage(message.Chat.ID, "Введите пароль:")
		session.State = StatePassword

	case StatePassword:
		password := strings.TrimSpace(message.Text)

		// Аутентификация команды
		team, err := b.teamService.AuthenticateTeam(session.Email, password)
		if err != nil {
			b.sendMessage(message.Chat.ID, "Неверный email или пароль. Попробуйте снова.\nВведите email:")
			session.State = StateEmail
			return
		}

		// Связываем Telegram ID с командой
		err = b.teamService.LinkTelegramToTeam(session.Email, message.Chat.ID)
		if err != nil {
			b.sendMessage(message.Chat.ID, "Ошибка при привязке Telegram к команде: "+err.Error())
			session.State = StateStart
			return
		}

		session.TeamID = team.ID.String()
		session.State = StateMenu
		b.sendMainMenu(message.Chat.ID)

	case StateMenu:
		b.handleMenuCommand(message)

	case StateAnswer:
		answer := strings.TrimSpace(message.Text)

		correct, err := b.teamService.SubmitAnswer(
			models.UUIDFromString(session.TeamID),
			models.UUIDFromString(session.TaskID),
			answer,
		)

		if err != nil {
			b.sendMessage(message.Chat.ID, "Ошибка при отправке ответа: "+err.Error())
		} else if correct {
			b.sendMessage(message.Chat.ID, "✅ Правильный ответ!")
		} else {
			b.sendMessage(message.Chat.ID, "❌ Неправильный ответ.")
		}

		// Проверка: закончена ли задача
		sessionData, err := b.teamService.GetTaskSession(
			models.UUIDFromString(session.TeamID),
			models.UUIDFromString(session.TaskID),
		)

		if err == nil && sessionData.Finished {
			// Пытаемся выдать следующее задание
			task, err := b.teamService.GetTask(models.UUIDFromString(session.TeamID))
			if err != nil {
				b.sendMessage(message.Chat.ID, "Больше нет доступных задач или ваша сессия завершена.")
			} else {
				session.TaskID = task.ID.String()
				b.sendMessage(message.Chat.ID, "Следующая задача:\\n\\n\""+task.Question)
				return
			}
		}

		session.State = StateMenu
		b.sendMainMenu(message.Chat.ID)
	}
}

func (b *TelegramBot) checkEmailAndProceed(chatID int64, session *UserSession) {
	// Проверяем существование email
	_, err := b.teamService.GetTeamByEmail(session.Email)

	if err != nil {
		// Если email не найден, предлагаем регистрацию
		msg := fmt.Sprintf("⚠️ Команда с email %s не найдена.\nХотите зарегистрироваться? (да/нет)", session.Email)
		b.sendMessage(chatID, msg)
		session.State = StateRegisterConfirm
	} else {
		// Если email существует, запрашиваем пароль
		b.sendMessage(chatID, "Введите пароль:")
		session.State = StatePassword
	}
}

func isValidEmail(email string) bool {
	return regexp.MustCompile(`^[a-z0-9._%+\-]+@[a-z0-9.\-]+\.[a-z]{2,4}$`).MatchString(email)
}

// handleCallback обрабатывает callback запросы от inline кнопок
func (b *TelegramBot) handleCallback(callback *tgbotapi.CallbackQuery) {
	session := b.getSession(callback.Message.Chat.ID)
	b.bot.Request(tgbotapi.NewCallback(callback.ID, ""))

	switch callback.Data {
	case "join_contest":
		contest, err := b.teamService.JoinContest(uuid.MustParse(session.TeamID))
		if err != nil {
			b.sendMessage(callback.Message.Chat.ID, "❌ Ошибка: "+err.Error())
			return
		}
		b.sendMessage(callback.Message.Chat.ID, fmt.Sprintf("✅ Вы присоединились к контесту: %s", contest.Name))
		session.State = StateWaitingGeo
		b.sendMainMenu(callback.Message.Chat.ID)

	case "send_geo":
		teamID := uuid.MustParse(session.TeamID)

		// 1. Получаем companyID для команды
		companyID, err := b.teamService.GetCompanyIDByTeam(teamID)
		if err != nil {
			b.sendMessage(callback.Message.Chat.ID, "❌ Команда не привязана к компании: "+err.Error())
			return
		}

		// 2. Получаем токен компании
		token, err := b.getCompanyTokenDirect(companyID)
		if err != nil {
			b.sendMessage(callback.Message.Chat.ID, "❌ Ошибка авторизации: "+err.Error())
			return
		}

		// 3. Запрашиваем геолокацию
		locReq, err := http.NewRequest("GET", "http://localhost:8080/api/v1/company/location", nil)
		if err != nil {
			b.sendMessage(callback.Message.Chat.ID, "❌ Ошибка создания запроса геолокации: "+err.Error())
			return
		}
		locReq.Header.Set("Authorization", "Bearer "+token)

		client := &http.Client{}
		locResp, err := client.Do(locReq)
		if err != nil {
			b.sendMessage(callback.Message.Chat.ID, "❌ Ошибка запроса геолокации: "+err.Error())
			return
		}
		defer locResp.Body.Close()

		// 4. Обрабатываем ответ
		locBody, _ := io.ReadAll(locResp.Body)
		if locResp.StatusCode != http.StatusOK {
			b.sendMessage(callback.Message.Chat.ID, fmt.Sprintf("❌ Не удалось получить геолокацию: статус %d, ответ: %s",
				locResp.StatusCode, string(locBody)))
			return
		}

		// 5. Парсим данные геолокации
		var locationResponse struct {
			MapLink string `json:"map_link"`
		}
		if err := json.Unmarshal(locBody, &locationResponse); err != nil {
			b.sendMessage(callback.Message.Chat.ID, "❌ Ошибка обработки данных геолокации: "+err.Error())
			return
		}

		// 6. Отправляем результат пользователю
		b.sendMessage(callback.Message.Chat.ID, "📍 Геолокация компании:\n"+locationResponse.MapLink)
		b.sendMessage(callback.Message.Chat.ID, "Ожидайте одобрения...")
		session.State = StateWaitingApprove
		go b.awaitApproval(callback.Message.Chat.ID, session)

	case "get_task":
		b.handleGetTask(callback.Message.Chat.ID, session)

	case "submit_answer":
		b.sendMessage(callback.Message.Chat.ID, "✍️ Введите ваш ответ:")
		session.State = StateAnswer

	case "logout":
		delete(b.sessions, callback.Message.Chat.ID)
		b.sendMessage(callback.Message.Chat.ID, "🚪 Вы вышли. Введите /start чтобы начать заново.")
	}
}

func (b *TelegramBot) getCompanyToken(teamID uuid.UUID) (string, error) {
	// 1. Получить CompanyID через сервис команд
	companyID, err := b.teamService.GetCompanyIDByTeam(teamID)
	if err != nil {
		return "", fmt.Errorf("team company not found: %v", err)
	}

	// 2. Получить данные компании через сервис
	companyData, err := b.teamService.GetCompanyCredentials(companyID)
	if err != nil {
		return "", fmt.Errorf("company data error: %v", err)
	}

	// 3. Авторизоваться как компания
	client := &http.Client{}
	data := map[string]string{
		"email":    companyData.Email,
		"password": companyData.TempPassword,
	}
	jsonData, _ := json.Marshal(data)

	resp, err := client.Post(
		"http://localhost:8080/api/v1/company/login",
		"application/json",
		bytes.NewBuffer(jsonData),
	)
	if err != nil || resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("ошибка аутентификации: %v", err)
	}
	defer resp.Body.Close()

	// 3. Парсим токен из ответа
	var result struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("ошибка парсинга токена: %v", err)
	}

	return result.Token, nil
}

// sendStartMessage отправляет приветственное сообщение
func (b *TelegramBot) sendStartMessage(chatID int64) {
	msg := "👋 Добро пожаловать в бот для команд!\nДля авторизации введите email вашей команды:"
	b.sendMessage(chatID, msg)
}

// sendMainMenu отправляет главное меню
func (b *TelegramBot) sendMainMenu(chatID int64) {
	session := b.getSession(chatID)
	var buttons [][]tgbotapi.InlineKeyboardButton

	switch session.State {
	case StateMenu:
		buttons = append(buttons, []tgbotapi.InlineKeyboardButton{
			tgbotapi.NewInlineKeyboardButtonData("Добавиться к контесту", "join_contest"),
		})
	case StateWaitingGeo:
		buttons = append(buttons, []tgbotapi.InlineKeyboardButton{
			tgbotapi.NewInlineKeyboardButtonData("Получить гео", "send_geo"),
		})
	case StateWaitingApprove:
		buttons = append(buttons, []tgbotapi.InlineKeyboardButton{
			tgbotapi.NewInlineKeyboardButtonData("Ожидание одобрения...", "waiting_approve"),
		})
	case StateReadyToGetTask:
		buttons = append(buttons, []tgbotapi.InlineKeyboardButton{
			tgbotapi.NewInlineKeyboardButtonData("Получить задание", "get_task"),
		})
	case StateTaskReceived:
		buttons = append(buttons, []tgbotapi.InlineKeyboardButton{
			tgbotapi.NewInlineKeyboardButtonData("Дать ответ", "submit_answer"),
		})
	case StateAllTasksComplete:
		buttons = append(buttons, []tgbotapi.InlineKeyboardButton{
			tgbotapi.NewInlineKeyboardButtonData("Выйти", "logout"),
		})
	}

	msg := tgbotapi.NewMessage(chatID, "Главное меню:")
	if len(buttons) > 0 {
		msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(buttons...)
	} else {
		msg.Text = "Добро пожаловать! Выберите действие."
	}
	b.bot.Send(msg)
}

// handleMenuCommand обрабатывает команды из главного меню
func (b *TelegramBot) handleMenuCommand(message *tgbotapi.Message) {
	switch message.Text {
	case "/start":
		b.sendStartMessage(message.Chat.ID)
		b.sessions[message.Chat.ID].State = StateEmail
	case "/menu":
		b.sendMainMenu(message.Chat.ID)
	default:
		b.sendMessage(message.Chat.ID, "Неизвестная команда. Используйте кнопки меню или отправьте /menu")
	}
}

// handleJoinContest обрабатывает запрос на присоединение к контесту
func (b *TelegramBot) handleJoinContest(chatID int64, session *UserSession) {
	teamID := models.UUIDFromString(session.TeamID)
	contest, err := b.teamService.JoinContest(teamID)
	if err != nil {
		b.sendMessage(chatID, "Ошибка при присоединении к контесту: "+err.Error())
		return
	}

	msg := "Вы успешно присоединились к контесту: " + contest.Name + "\n" +
		"Дата начала: " + contest.StartDate.Format("02.01.2006 15:04") + "\n" +
		"Дата окончания: " + contest.EndDate.Format("02.01.2006 15:04")
	b.sendMessage(chatID, msg)
}

func (b *TelegramBot) awaitApproval(chatID int64, session *UserSession) {
	_, err := b.teamService.GetTeamByEmail(session.Email)
	if err != nil {
		b.sendMessage(chatID, "❌ Ошибка: команда не найдена")
		return
	}

	for i := 0; i < 30; i++ { // до 30 попыток (например, 30 секунд)
		time.Sleep(2 * time.Second)
		updated, err := b.teamService.GetTeamByEmail(session.Email)
		if err == nil && updated.CompanyID != nil {
			b.sendMessage(chatID, "🎉 Ваша команда была одобрена компанией!")
			session.State = StateReadyToGetTask
			b.sendMainMenu(chatID)
			return
		}
	}

	b.sendMessage(chatID, "⌛ Ожидание одобрения превысило лимит времени. Попробуйте позже.")
	session.State = StateMenu
	b.sendMainMenu(chatID)
}

// handleGetTask обрабатывает запрос на получение задачи
func (b *TelegramBot) handleGetTask(chatID int64, session *UserSession) {
	task, err := b.teamService.GetTask(uuid.MustParse(session.TeamID))
	if err != nil {
		b.sendMessage(chatID, "❌ Ошибка при получении задачи: "+err.Error())
		return
	}

	session.TaskID = task.ID.String()
	session.State = StateTaskReceived

	if task.QuestionFile != "" {
		filePath := pkg.GetFilePath(task.ID, task.QuestionFile)
		doc := tgbotapi.NewDocument(chatID, tgbotapi.FilePath(filePath))
		doc.Caption = fmt.Sprintf("Задача:\n\n%s\n\nВремя: %d минут", task.Question, task.TimeLimit)
		b.bot.Send(doc)
	} else {
		b.sendMessage(chatID, fmt.Sprintf("Задача:\n\n%s\n\nВремя: %d минут", task.Question, task.TimeLimit))
	}

	b.sendMainMenu(chatID)
}

// sendMessage отправляет сообщение пользователю
func (b *TelegramBot) sendMessage(chatID int64, text string) {
	msg := tgbotapi.NewMessage(chatID, text)
	_, err := b.bot.Send(msg)
	if err != nil {
		log.Printf("Error sending message: %v", err)
	}
}

// Helper function to authenticate as a company and get a token
func (b *TelegramBot) authenticateCompany(email, password string) (string, error) {
	client := &http.Client{}

	// Check the actual request structure expected by the server
	// This structure should match what the server expects to unmarshal
	data := map[string]interface{}{
		"email":    email,
		"password": password,
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		return "", fmt.Errorf("ошибка сериализации JSON: %v", err)
	}

	// For debugging - log the request body
	log.Printf("Authentication request body: %s", string(jsonData))

	resp, err := client.Post(
		"http://localhost:8080/api/v1/company/login",
		"application/json",
		bytes.NewBuffer(jsonData),
	)
	if err != nil {
		return "", fmt.Errorf("ошибка запроса: %v", err)
	}
	defer resp.Body.Close()

	// Read the response body for debugging
	respBody, _ := io.ReadAll(resp.Body)
	log.Printf("Authentication response (status %d): %s", resp.StatusCode, string(respBody))

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("ошибка авторизации, статус: %d, ответ: %s", resp.StatusCode, string(respBody))
	}

	// Need to create a new reader since we consumed the response body above
	var result struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("ошибка парсинга токена: %v", err)
	}

	return result.Token, nil
}

// Alternative implementation that checks company credentials first
func (b *TelegramBot) authenticateCompany2(companyID uuid.UUID) (string, error) {
	// 1. Get company data to check what credentials we have
	company, err := b.teamService.GetCompanyCredentials(companyID)
	if err != nil {
		return "", fmt.Errorf("ошибка получения данных компании: %v", err)
	}

	// Log credentials for debugging (remove in production)
	log.Printf("Company credentials - Email: %s, TempPass: %s", company.Email, company.TempPassword)

	client := &http.Client{}

	// Try different request structures
	// Version 1: Standard format
	reqBody1 := map[string]string{
		"email":    company.Email,
		"password": company.TempPassword,
	}

	// Version 2: Using different field names (in case server expects different names)
	reqBody2 := map[string]string{
		"email":        company.Email,
		"tempPassword": company.TempPassword,
	}

	// Try first format
	jsonData, _ := json.Marshal(reqBody1)
	resp, err := client.Post(
		"http://localhost:8080/api/v1/company/login",
		"application/json",
		bytes.NewBuffer(jsonData),
	)

	if err != nil || resp.StatusCode != http.StatusOK {
		// If first format fails, try second format
		jsonData, _ = json.Marshal(reqBody2)
		resp, err = client.Post(
			"http://localhost:8080/api/v1/company/login",
			"application/json",
			bytes.NewBuffer(jsonData),
		)

		if err != nil {
			return "", fmt.Errorf("ошибка запроса: %v", err)
		}
	}

	defer resp.Body.Close()

	// Read response body
	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("ошибка авторизации, статус: %d, ответ: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("ошибка парсинга токена: %v", err)
	}

	return result.Token, nil
}

func (b *TelegramBot) getCompanyLocation(token string) (string, error) {
	client := &http.Client{}
	req, err := http.NewRequest("GET", "http://localhost:8080/api/v1/company/location", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("ошибка получения геолокации, статус: %d", resp.StatusCode)
	}

	var locationResponse struct {
		MapLink string `json:"map_link"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&locationResponse); err != nil {
		return "", fmt.Errorf("ошибка обработки данных: %v", err)
	}

	return locationResponse.MapLink, nil
}

func (b *TelegramBot) getCompanyTokenDirect(companyID uuid.UUID) (string, error) {
	// Do a direct database query to get the company info
	// This is instead of going through the API login flow
	company, err := b.teamService.GetCompanyByID(companyID)
	if err != nil {
		return "", fmt.Errorf("failed to get company: %v", err)
	}

	// Generate a JWT token directly using the same logic as your API
	// You'll need to share the JWT secret with this code
	jwtSecret := os.Getenv("JWT_SECRET") // Replace with your actual JWT secret

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":            company.ID.String(),
		"role":           "company",
		"exp":            time.Now().Add(time.Hour * 24).Unix(),
		"reset_required": company.ResetRequired,
	})

	tokenString, err := token.SignedString([]byte(jwtSecret))
	if err != nil {
		return "", fmt.Errorf("failed to generate token: %v", err)
	}

	return tokenString, nil
}
