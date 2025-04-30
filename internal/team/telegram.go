package team

import (
	"Cyber-chase/internal/models"
	"Cyber-chase/internal/service"
	"fmt"
	"log"
	"regexp"
	"strconv"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

const (
	StateStart           = "start"    // Начальное состояние
	StateEmail           = "email"    // Ввод email
	StatePassword        = "password" // Ввод пароля
	StateMenu            = "menu"     // Главное меню
	StateAnswer          = "answer"   // Ввод ответа на задачу
	StateRegisterName    = "register_name"
	StateRegisterPass    = "register_pass"
	StateRegisterConfirm = "register_confirm"
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
		b.sendMainMenu(message.Chat.ID)
		session.State = StateMenu

	case StateMenu:
		b.handleMenuCommand(message)

	case StateAnswer:
		answer := strings.TrimSpace(message.Text)

		// Исправлено: Удалены неиспользуемые переменные teamID и taskID
		// Прямо использующие методы для парсинга из строки в UUID
		correct, err := b.teamService.SubmitAnswer(models.UUIDFromString(session.TeamID), models.UUIDFromString(session.TaskID), answer)
		if err != nil {
			b.sendMessage(message.Chat.ID, "Ошибка при отправке ответа: "+err.Error())
		} else if correct {
			b.sendMessage(message.Chat.ID, "Правильный ответ! 🎉")
		} else {
			b.sendMessage(message.Chat.ID, "Неправильный ответ. Попробуйте еще раз.")
		}

		b.sendMainMenu(message.Chat.ID)
		session.State = StateMenu
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

	// Отвечаем на callback, чтобы убрать часы загрузки
	callbackConfig := tgbotapi.NewCallback(callback.ID, "")
	b.bot.Request(callbackConfig)

	switch callback.Data {
	case "join_contest":
		b.handleJoinContest(callback.Message.Chat.ID, session)
	case "get_task":
		b.handleGetTask(callback.Message.Chat.ID, session)
	case "submit_answer":
		b.sendMessage(callback.Message.Chat.ID, "Введите ваш ответ:")
		session.State = StateAnswer
	}
}

// sendStartMessage отправляет приветственное сообщение
func (b *TelegramBot) sendStartMessage(chatID int64) {
	msg := "👋 Добро пожаловать в бот для команд!\nДля авторизации введите email вашей команды:"
	b.sendMessage(chatID, msg)
}

// sendMainMenu отправляет главное меню
func (b *TelegramBot) sendMainMenu(chatID int64) {
	var keyboard = tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Присоединиться к контесту", "join_contest"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Получить задачу", "get_task"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Отправить ответ", "submit_answer"),
		),
	)

	msg := tgbotapi.NewMessage(chatID, "Главное меню:")
	msg.ReplyMarkup = keyboard
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

// handleGetTask обрабатывает запрос на получение задачи
func (b *TelegramBot) handleGetTask(chatID int64, session *UserSession) {
	teamID := models.UUIDFromString(session.TeamID)
	task, err := b.teamService.GetTask(teamID)
	if err != nil {
		b.sendMessage(chatID, "Ошибка при получении задачи: "+err.Error())
		return
	}

	session.TaskID = task.ID.String()

	msg := "Задача:\n\n" + task.Question
	if task.QuestionFile != "" {
		msg += "\n\nФайл с дополнительной информацией: " + task.QuestionFile
	}

	if task.TimeLimit > 0 {
		msg += "\n\nВремя на решение: " + strconv.Itoa(task.TimeLimit) + " минут"
	}

	b.sendMessage(chatID, msg)
}

// sendMessage отправляет сообщение пользователю
func (b *TelegramBot) sendMessage(chatID int64, text string) {
	msg := tgbotapi.NewMessage(chatID, text)
	_, err := b.bot.Send(msg)
	if err != nil {
		log.Printf("Error sending message: %v", err)
	}
}
