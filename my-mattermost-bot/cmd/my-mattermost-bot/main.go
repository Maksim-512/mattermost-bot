package main

import (
	"context"
	"encoding/json"
	"github.com/joho/godotenv"
	"github.com/mattermost/mattermost/server/public/model"
	"log"
	"log/slog"
	"my-mattermost-bot/internal/bot"
	"my-mattermost-bot/internal/config"
	"my-mattermost-bot/internal/models"
	"os"
	"strings"
)

func main() {
	err := godotenv.Load("bot.env")
	if err != nil {
		log.Fatal("Ошибка загрузки bot.env файла")
	}

	app := &models.MatterInfo{
		Ctx:      context.Background(),
		MyLogger: setupLogger(),
		Config:   config.LoadConfig(),
	}

	app.MyLogger.Info("Старт приложения")

	app.MattermostClient = model.NewAPIv4Client(app.Config.MattermostServer.String())
	app.MattermostClient.SetToken(app.Config.MattermostToken)

	user, _, err := app.MattermostClient.GetMe(app.Ctx, "")
	if err != nil {
		app.MyLogger.Warn("Ошибка получения информации", slog.String("error", err.Error()))
		return
	}

	app.MyLogger.Info("Вход в Mattermost", slog.String("username", user.Username))
	app.MattermostUser = user

	team, _, err := app.MattermostClient.GetTeamByName(app.Ctx, app.Config.MattermostTeamName, "")
	if err != nil {
		app.MyLogger.Warn("Не удалось найти команду", slog.String("error", err.Error()))
		return
	}

	app.MattermostTeam = team
	app.MyLogger.Info("Mattermost Team", slog.String("team", app.MattermostTeam.DisplayName))

	channel, _, err := app.MattermostClient.GetChannelByName(
		app.Ctx,
		app.Config.MattermostChannel,
		app.MattermostTeam.Id,
		"")
	if err != nil {
		app.MyLogger.Warn("Не удалось найти канал", slog.String("error", err.Error()))
		return
	}

	app.MattermostChannel = channel
	app.MyLogger.Info("Mattermost Channel", slog.String("channel", channel.Name))

	mmBot, err := bot.NewBot(
		app.MyLogger,
		app.MattermostClient,
		app.Config.TarantoolAddress,
		app.MattermostChannel.Id,
		user.Username,
	)
	if err != nil {
		app.MyLogger.Error("Ошибка создания бота", slog.String("error", err.Error()))
		return
	}

	websClient, err := model.NewWebSocketClient4(
		"ws://mattermost:8065",
		app.MattermostClient.AuthToken,
	)
	if err != nil {
		app.MyLogger.Error("Ошибка подключения к WebSocket", slog.String("error", err.Error()))

		return
	}

	websClient.Listen()
	app.MyLogger.Info("WebSocket прослушивание стартануло")

	for event := range websClient.EventChannel {
		if event.EventType() == model.WebsocketEventPosted {
			go processMessage(event, app, mmBot)
		}
	}
}

func processMessage(event *model.WebSocketEvent, app *models.MatterInfo, mmBot *bot.Bot) {
	postData, ok := event.GetData()["post"].(string)
	if !ok {
		app.MyLogger.Warn("Неверные данные для публикации")
		return
	}

	var post model.Post
	if err := json.Unmarshal([]byte(postData), &post); err != nil {
		app.MyLogger.Warn("Ошибка после декодирования", slog.String("error", err.Error()))
		return
	}

	if post.ChannelId != app.MattermostChannel.Id ||
		post.UserId == app.MattermostUser.Id {
		return
	}

	msg := strings.TrimSpace(post.Message)
	app.MyLogger.Debug("Новое сообщение", slog.String("content", msg))

	if strings.HasPrefix(msg, "@"+mmBot.BotName) {
		msg = strings.TrimPrefix(msg, "@"+mmBot.BotName)
		msg = strings.TrimSpace(msg)

		if err := mmBot.HandleCommand(msg, post.UserId); err != nil {
			app.MyLogger.Error("Неверная команда",
				slog.String("error", err.Error()),
				slog.String("command", msg))
		}
	}
}

func setupLogger() *slog.Logger {
	var logger *slog.Logger

	logger = slog.New(
		slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
			Level: slog.LevelDebug,
		}),
	)

	return logger
}
