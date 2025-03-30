package storage

import (
	"fmt"
	"github.com/tarantool/go-tarantool"
	"my-mattermost-bot/internal/models"
)

type TarantoolStorage struct {
	DB *tarantool.Connection
}

func NewTarantoolStorage(addr string) (*TarantoolStorage, error) {
	opts := tarantool.Opts{
		User: "guest",
		Pass: "",
	}
	conn, err := tarantool.Connect(addr, opts)
	if err != nil {
		return nil, err
	}

	_, err = conn.Ping()
	if err != nil {
		return nil, fmt.Errorf("не удалось пингануть Tarantool: %w", err)
	}

	return &TarantoolStorage{
		DB: conn,
	}, nil
}

func (t *TarantoolStorage) CreateVoting(id, question, createdBy string, options []string) error {
	optionsMap := make(map[string]int)
	for _, option := range options {
		optionsMap[option] = 0
	}

	vote := models.Vote{
		ID:        id,
		Question:  question,
		Options:   optionsMap,
		CreatedBy: createdBy,
		IsClosed:  false,
	}

	_, err := t.DB.Insert("votings", []interface{}{vote.ID, vote.Question, vote.Options, vote.CreatedBy, vote.IsClosed})
	if err != nil {
		return fmt.Errorf("failed to insert voting: %w", err)
	}

	return nil
}

func (t *TarantoolStorage) GetVoting(id string) (*models.Vote, error) {
	resp, err := t.DB.Select("votings", "primary", 0, 1, tarantool.IterEq, []interface{}{id})
	if err != nil {
		return nil, fmt.Errorf("не удалось выбрать голосование: %w", err)
	}

	if len(resp.Data) == 0 {
		return nil, fmt.Errorf("голосование не найдено")
	}

	data := resp.Data[0].([]interface{})

	rawOptions := data[2].(map[interface{}]interface{})
	options := make(map[string]int)
	for k, v := range rawOptions {
		key := fmt.Sprintf("%v", k)

		switch val := v.(type) {
		case uint64:
			options[key] = int(val)
		default:
			return nil, fmt.Errorf("неверный тип подсчета голосов: %T", v)
		}
	}

	vote := &models.Vote{
		ID:        data[0].(string),
		Question:  data[1].(string),
		Options:   options,
		CreatedBy: data[3].(string),
		IsClosed:  data[4].(bool),
	}

	return vote, nil
}

func (t *TarantoolStorage) AddVote(voteID, option string) error {
	vote, err := t.GetVoting(voteID)
	if err != nil {
		return fmt.Errorf("не удалось добиться голосования: %w", err)
	}

	if vote.IsClosed {
		return fmt.Errorf("голосование уже завершено")
	}

	if _, exists := vote.Options[option]; !exists {
		return fmt.Errorf("вариант \"%s\" не существует в этом голосовании", option)
	}

	vote.Options[option]++

	_, err = t.DB.Update("votings", "primary", []interface{}{voteID}, []interface{}{
		[]interface{}{"=", 2, vote.Options},
	})
	if err != nil {
		return fmt.Errorf("не удалось обновить голосования: %w", err)
	}

	return nil
}

func (t *TarantoolStorage) CloseVoting(voteID, userID string) error {
	vote, err := t.GetVoting(voteID)
	if err != nil {
		return fmt.Errorf("не удалось добиться голосования: %w", err)
	}

	if vote.CreatedBy != userID {
		return fmt.Errorf("только создатель голосования может его завершить")
	}

	if vote.IsClosed {
		return fmt.Errorf("голосование уже завершено")
	}

	_, err = t.DB.Update("votings", "primary", []interface{}{voteID}, []interface{}{
		[]interface{}{"=", 4, true},
	})
	if err != nil {
		return fmt.Errorf("не удалось завершить голосование: %w", err)
	}

	return nil
}

func (t *TarantoolStorage) DeleteVoting(voteID, userID string) error {
	vote, err := t.GetVoting(voteID)
	if err != nil {
		return fmt.Errorf("не удалось получить голосование: %w", err)
	}

	if vote.CreatedBy != userID {
		return fmt.Errorf("только создатель голосования может его удалить")
	}

	_, err = t.DB.Delete("votings", "primary", []interface{}{voteID})
	if err != nil {
		return fmt.Errorf("ошибка удаления голосования: %w", err)
	}

	return nil
}

func (t *TarantoolStorage) VotingExists(id string) (bool, error) {
	resp, err := t.DB.Select("votings", "primary", 0, 1, tarantool.IterEq, []interface{}{id})
	if err != nil {
		return false, fmt.Errorf("не удалось проверить наличие голосования: %w", err)
	}
	return len(resp.Data) > 0, nil
}
