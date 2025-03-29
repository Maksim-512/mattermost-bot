package models

import (
	"context"
	"log/slog"

	"github.com/mattermost/mattermost/server/public/model"

	"my-mattermost-bot/internal/config"
)

type MatterInfo struct {
	Ctx                       context.Context
	Config                    *config.Config
	MyLogger                  *slog.Logger
	MattermostClient          *model.Client4
	MattermostWebSocketClient *model.WebSocketClient
	MattermostUser            *model.User
	MattermostChannel         *model.Channel
	MattermostTeam            *model.Team
}
