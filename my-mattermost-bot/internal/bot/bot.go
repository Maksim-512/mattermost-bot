package bot

import (
	"context"
	"fmt"
	"log/slog"
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

	msg = strings.TrimSpace(msg)
	if !strings.HasPrefix(msg, "vote") {
		return nil
	}

	parts := strings.Fields(msg)
	lenParts := len(parts)

	if lenParts < 2 {
		return b.SendHelp()
	}

	switch parts[1] {
	case "create":
		return b.handleCreateVote(parts[2:], userID)
	case "vote":
		return b.handleVote(parts[2:], userID)
	case "results":
		return b.handleResults(parts[2:])
	case "close":
		return b.handleCloseVote(parts[2:], userID)
	case "delete":
		return b.handleDeleteVote(parts[2:], userID)
	default:
		return b.handleVoteForOption(parts[1:], userID)
	}
}

func (b *Bot) handleCreateVote(parts []string, userID string) error {
	const op = "bot.handleCreateVote"

	b.myLogger.Info(fmt.Sprintf("%s. Начало создание опроса.", op))

	if len(parts) < 2 {
		return b.SendMessage(`Для создания голосования укажите вопрос и варианты ответов.\nПример: /vote create \"Ваш вопрос\" \"Вариант 1\" \"Вариант 2\"`)
	}

	question := parts[0]
	options := parts[1:]
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
	message += fmt.Sprintf("\nДля голосования: `/vote %s <номер варианта>`", voteID)

	b.myLogger.Info("Голосование успешно создано")

	fmt.Println(message)
	return b.SendMessage(message)
}

func (b *Bot) handleVoteForOption(parts []string, userID string) error {
	const op = "bot.handleVoteForOption"

	if len(parts) < 2 {
		return b.SendHelp()
	}

	voteID := parts[0]
	option := strings.Join(parts[1:], " ")

	err := b.Storage.AddVote(voteID, userID, option)
	if err != nil {
		b.myLogger.Error(fmt.Sprintf("%s. Failed to add vote", op), slog.String("error", err.Error()))
		return b.SendMessage(fmt.Sprintf("Не удалось зарегистрировать ваш голос: %v", err))
	}

	return b.SendMessage(fmt.Sprintf("Ваш голос за вариант \"%s\" зарегистрирован!", option))
}

func (b *Bot) handleVote(args []string, userID string) error {
	if len(args) < 2 {
		return b.SendMessage("Ошибка: укажите ID голосования и номер варианта\nПример: `vote a8f5acae-be6a-40c0-bab8-c70aaab96383 1`")
	}

	voteID := args[0]
	optionNum := args[1]

	// Проверка существования голосования (запрос к Tarantool)
	exists, err := b.Storage.VotingExists(voteID)
	if err != nil {
		return b.SendMessage("Ошибка при проверке голосования")
	}
	if !exists {
		return b.SendMessage("Голосование с таким ID не найдено")
	}

	// Запись голоса
	err = b.Storage.AddVote(voteID, userID, optionNum)
	if err != nil {
		return b.SendMessage("Ошибка: ваш голос не засчитан")
	}

	return b.SendMessage(fmt.Sprintf(
		"✅ Ваш голос за вариант %s учтён!",
		optionNum))
}

func (b *Bot) handleResults(parts []string) error {
	const op = "bot.handleResults"

	if len(parts) < 1 {
		return b.SendHelp()
	}

	voteID := parts[0]
	vote, err := b.Storage.GetVoting(voteID)
	if err != nil {
		b.myLogger.Error(fmt.Sprintf("%s. Failed to get voting results", op), slog.String("error", err.Error()))
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
		message += fmt.Sprintf("\n\nДля голосования: `/vote %s <номер варианта>`", vote.ID)
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
		b.myLogger.Error(fmt.Sprintf("%s. Failed to close voting", op), slog.String("error", err.Error()))
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
		b.myLogger.Error(fmt.Sprintf("%s. Failed to delete voting", op), slog.String("error", err.Error()))
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

	b.myLogger.Debug("Message sent successfully")
	return nil
}

func (b *Bot) SendHelp() error {
	helpMessage := `**Команды бота для голосования:

	1. Создать голосование:
	   /vote create "Вопрос" "Вариант 1" "Вариант 2" ...
	
	2. Проголосовать:
	   /vote <ID голосования> <номер варианта>
	   или
	   /vote vote <ID голосования> <номер варианта>
	
	3. Просмотреть результаты:
	   /vote results <ID голосования>
	
	4. Завершить голосование (только создатель):
	   /vote close <ID голосования>
	
	5. Удалить голосование (только создатель):
	   /vote delete <ID голосования>`

	return b.SendMessage(helpMessage)
}
