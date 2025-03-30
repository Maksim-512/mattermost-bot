package bot

import (
	"context"
	"fmt"
	"log/slog"
	"regexp"
	"strings"

	"github.com/google/uuid"
	"github.com/mattermost/mattermost/server/public/model"

	"my-mattermost-bot/internal/storage"
)

type Bot struct {
	Client    *model.Client4
	Storage   *storage.TarantoolStorage
	ChannelID string
	myLogger  *slog.Logger
	BotName   string
}

func NewBot(myLogger *slog.Logger, client *model.Client4, storagePath, channelID, botName string) (*Bot, error) {
	conn, err := storage.NewTarantoolStorage(storagePath)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Tarantool: %w", err)
	}
	return &Bot{
		myLogger:  myLogger,
		Client:    client,
		Storage:   conn,
		ChannelID: channelID,
		BotName:   botName,
	}, nil
}

func (b *Bot) HandleCommand(msg, userID string) error {
	const op = "bot.HandleCommand"

	b.myLogger.Info(fmt.Sprintf("%s. Получена команда. Message: %s. User: %s", op, msg, userID))

	if msg == "help" {
		return b.SendHelp()
	}

	msg = strings.TrimSpace(msg)

	parts := strings.Fields(msg)
	command := parts[0]
	lenParts := len(parts)

	if lenParts < 2 {
		return b.SendHelp()
	}

	switch command {
	case "create":
		re := regexp.MustCompile(`"([^"]*)"`)
		matches := re.FindAllStringSubmatch(msg, -1)

		var question string
		var options []string

		if len(matches) > 0 {
			question = matches[0][1]
		}

		for i := 1; i < len(matches); i++ {
			options = append(options, matches[i][1])
		}
		return b.handleCreateVote(question, options, userID)
	case "vote":
		return b.handleVote(parts[1:])
	case "results":
		return b.handleResults(parts[1:])
	case "close":
		return b.handleCloseVote(parts[1:], userID)
	case "delete":
		return b.handleDeleteVote(parts[1:], userID)
	default:
		return b.SendHelp()
	}
}

func (b *Bot) handleCreateVote(question string, options []string, userID string) error {
	const op = "bot.handleCreateVote"

	b.myLogger.Info(fmt.Sprintf("%s. Начало создание опроса.", op))

	if question == "" || len(options) == 0 {
		return b.SendMessage("Для создания голосования укажите вопрос и варианты ответов.\nПример: @system-bot create \"Вопрос\" \"Вариант 1\" \"Вариант 2\" ...")
	}

	voteID := uuid.New().String()

	err := b.Storage.CreateVoting(voteID, question, userID, options)
	if err != nil {
		b.myLogger.Error("%s. Ошибка при создании голосования: %w", op, err)
		return b.SendMessage("Не удалось создать голосование. Пожалуйста, попробуйте позднее")
	}

	message := fmt.Sprintf("**Создано новое голосование**\nID: `%s`\nВопрос: %s\nВарианты:\n", voteID, question)
	for i, option := range options {
		message += fmt.Sprintf("%d. %s\n", i+1, option)
	}
	message += fmt.Sprintf("\nДля голосования: `@system-bot vote %s <номер варианта>`", voteID)

	b.myLogger.Info("Голосование успешно создано")

	fmt.Println(message)
	return b.SendMessage(message)
}

func (b *Bot) handleVote(args []string) error {
	if len(args) < 2 {
		return b.SendMessage("Ошибка: укажите ID голосования и вариант\nПример: `@system-bot vote <ID голосования> <вариант>`")
	}

	voteID := args[0]
	optionNum := strings.Join(args[1:], " ")

	exists, err := b.Storage.VotingExists(voteID)
	if err != nil {
		return b.SendMessage("Ошибка при проверке голосования")
	}
	if !exists {
		return b.SendMessage("Голосование с таким ID не найдено")
	}

	err = b.Storage.AddVote(voteID, optionNum)
	if err != nil {
		return b.SendMessage(fmt.Sprintf("Ошибка: ваш голос не засчитан. %w", err.Error()))
	}

	return b.SendMessage(fmt.Sprintf(
		"Ваш голос за вариант %s учтён!\nПросмотреть результаты: @system-bot results %s",
		optionNum, voteID))
}

func (b *Bot) handleResults(parts []string) error {
	const op = "bot.handleResults"

	if len(parts) != 1 {
		return b.SendHelp()
	}

	voteID := parts[0]
	vote, err := b.Storage.GetVoting(voteID)
	if err != nil {
		b.myLogger.Error(fmt.Sprintf("%s. Не удалось получить результаты голосования", op), slog.String("error", err.Error()))
		return b.SendMessage("Не удалось получить результаты голосования. Проверьте правильность ID.")
	}

	message := fmt.Sprintf("**Результаты голосования**\nВопрос: %s\n", vote.Question)
	totalVotes := 0
	for option, count := range vote.Options {
		message += fmt.Sprintf("- %s: %d голосов\n", option, count)
		totalVotes += count
	}
	message += fmt.Sprintf("\nВсего голосов: %d", totalVotes)

	if vote.IsClosed {
		message += "\n\nГолосование завершено."
	} else {
		message += fmt.Sprintf("\n\nДля голосования: `@system-bot vote %s <вариант>`", vote.ID)
	}

	return b.SendMessage(message)
}

func (b *Bot) handleCloseVote(parts []string, userID string) error {
	const op = "bot.handleCloseVote"

	if len(parts) < 1 {
		return b.SendHelp()
	}

	voteID := parts[0]
	err := b.Storage.CloseVoting(voteID, userID)
	if err != nil {
		b.myLogger.Error(fmt.Sprintf("%s. Не удалось завершить голосование", op), slog.String("error", err.Error()))
		return b.SendMessage(fmt.Sprintf("Не удалось завершить голосование: %v", err))
	}

	return b.SendMessage(fmt.Sprintf("Голосование `%s` успешно завершено!", voteID))
}

func (b *Bot) handleDeleteVote(parts []string, userID string) error {
	const op = "bot.handleDeleteVote"

	if len(parts) < 1 {
		return b.SendHelp()
	}

	voteID := parts[0]
	err := b.Storage.DeleteVoting(voteID, userID)
	if err != nil {
		b.myLogger.Error(fmt.Sprintf("%s. Не удалось удалить голосование", op), slog.String("error", err.Error()))
		return b.SendMessage(fmt.Sprintf("Не удалось удалить голосование: %v", err))
	}

	return b.SendMessage(fmt.Sprintf("Голосование `%s` успешно удалено!", voteID))
}

func (b *Bot) SendMessage(msg string) error {
	post := &model.Post{
		ChannelId: b.ChannelID,
		Message:   msg,
	}

	_, _, err := b.Client.CreatePost(context.Background(), post)
	if err != nil {
		b.myLogger.Error(fmt.Sprintf("Ошибка при отправке сообщения: %v", err))
		return fmt.Errorf("не удалось отправить сообщение: %w", err)
	}

	b.myLogger.Debug("Сообщение успешно отправлено")
	return nil
}

func (b *Bot) SendHelp() error {
	helpMessage := `**Команды бота для голосования:

	1. Создать голосование:
	   @system-bot create "Вопрос" "Вариант 1" "Вариант 2" ...
	
	2. Проголосовать:
	   @system-bot <ID голосования> <вариант>
	
	3. Просмотреть результаты:
	   @system-bot results <ID голосования>
	
	4. Завершить голосование (только создатель):
	   @system-bot close <ID голосования>
	
	5. Удалить голосование (только создатель):
	   @system-bot delete <ID голосования>`

	return b.SendMessage(helpMessage)
}
