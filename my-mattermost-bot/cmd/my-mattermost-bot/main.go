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
		log.Fatal("Error loading bot.env file")
	}

	app := &models.MatterInfo{
		Ctx:      context.Background(),
		MyLogger: setupLogger(),
		Config:   config.LoadConfig(),
	}

	app.MyLogger.Info("starting application")

	//tarantool, err := storage.NewTarantoolStorage(app.Config.TarantoolAddress)
	//if err != nil {
	//	log.Fatal("Не удалось подключиться к Tarantool:", err)
	//}

	app.MattermostClient = model.NewAPIv4Client(app.Config.MattermostServer.String())
	app.MattermostClient.SetToken(app.Config.MattermostToken)

	user, _, err := app.MattermostClient.GetMe(app.Ctx, "")
	if err != nil {
		app.MyLogger.Warn("Error getting user info", slog.String("error", err.Error()))
		return
	}

	app.MyLogger.Info("Logged in to Mattermost", slog.String("username", user.Username))
	app.MattermostUser = user

	team, _, err := app.MattermostClient.GetTeamByName(app.Ctx, app.Config.MattermostTeamName, "")
	if err != nil {
		app.MyLogger.Warn("Could not find team", slog.String("error", err.Error()))
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
		app.MyLogger.Warn("Could not find channel", slog.String("error", err.Error()))
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
		app.MyLogger.Error("Failed to create bot", slog.String("error", err.Error()))
		return
	}

	websClient, err := model.NewWebSocketClient4(
		//wsURL,
		"ws://localhost:8065",
		//"ws://localhost:8065/api/v4/websocket",
		app.MattermostClient.AuthToken,
	)
	if err != nil {
		app.MyLogger.Error("Failed to connect to WebSocket", slog.String("error", err.Error()))

		return
	}

	websClient.Listen()
	app.MyLogger.Info("WebSocket listener started")

	for event := range websClient.EventChannel {
		if event.EventType() == model.WebsocketEventPosted {
			go processMessage(event, app, mmBot)
			//		app.MyLogger.Debug("Received post event")
			//
			//		postDataInterface, ok := event.GetData()["post"]
			//		if !ok || postDataInterface == nil {
			//			app.MyLogger.Warn("No 'post' field in event data")
			//			continue
			//		}
			//
			//		postDataStr, ok := postDataInterface.(string)
			//		if !ok {
			//			app.MyLogger.Warn("Failed to assert 'post' data to string")
			//			continue
			//		}
			//
			//		// Преобразуем в структуру Post
			//		post := &model.Post{}
			//		err := json.Unmarshal([]byte(postDataStr), &post)
			//		if err != nil {
			//			app.MyLogger.Warn("Failed to unmarshal post data", slog.String("error", err.Error()))
			//			continue
			//		}
			//
			//		app.MyLogger.Debug("Processing message",
			//			slog.String("channel", post.ChannelId),
			//			slog.String("user", post.UserId),
			//			slog.String("message", post.Message),
			//			slog.Bool("is_bot", post.UserId == app.MattermostUser.Id))
			//
			//		if post.ChannelId != app.MattermostChannel.Id ||
			//			post.UserId == app.MattermostUser.Id {
			//
			//			app.MyLogger.Info(fmt.Sprintf("post.ChannelId: %s, app.MattermostChannel.Id: %s, post.UserId: %s, app.MattermostUser.Id: %s",
			//				post.ChannelId, app.MattermostChannel.Id, post.UserId, app.MattermostUser.Id))
			//			continue
			//		}
			//
			//		app.MyLogger.Info("Handling vote command", slog.String("command", post.Message))
			//
			//		err = mmBot.HandleCommand(post.Message, post.UserId)
			//		if err != nil {
			//			app.MyLogger.Error(
			//				"Failed to handle command",
			//				slog.String("error", err.Error()),
			//				slog.String("command", post.Message),
			//			)
			//		}
		}
	}
}

func processMessage(event *model.WebSocketEvent, app *models.MatterInfo, mmBot *bot.Bot) {
	postData, ok := event.GetData()["post"].(string)
	if !ok {
		app.MyLogger.Warn("Invalid post data")
		return
	}

	var post model.Post
	if err := json.Unmarshal([]byte(postData), &post); err != nil {
		app.MyLogger.Warn("Post decode error", slog.String("error", err.Error()))
		return
	}

	if post.ChannelId != app.MattermostChannel.Id ||
		post.UserId == app.MattermostUser.Id {
		return
	}

	msg := strings.TrimSpace(post.Message)
	app.MyLogger.Debug("New message", slog.String("content", msg))

	// Обрабатываем два формата:
	// 1. @бот vote create...
	// 2. /vote create...
	if strings.HasPrefix(msg, "@"+mmBot.BotName) || strings.HasPrefix(msg, "/vote") {
		// Нормализация сообщения
		msg = strings.TrimPrefix(msg, "@"+mmBot.BotName)
		msg = strings.TrimPrefix(msg, "/vote")
		msg = strings.TrimSpace(msg)

		if err := mmBot.HandleCommand(msg, post.UserId); err != nil {
			app.MyLogger.Error("Command failed",
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
