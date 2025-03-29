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
		return nil, fmt.Errorf("failed to ping Tarantool: %w", err)
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
		return nil, fmt.Errorf("failed to select voting: %w", err)
	}

	if len(resp.Data) == 0 {
		return nil, fmt.Errorf("voting not found")
	}

	data := resp.Data[0].([]interface{})

	rawOptions := data[2].(map[interface{}]interface{})
	options := make(map[string]int)
	for k, v := range rawOptions {
		key := fmt.Sprintf("%v", k) // Безопасное преобразование ключа

		// Обработка разных числовых типов
		switch val := v.(type) {
		case int:
			options[key] = val
		case int64:
			options[key] = int(val)
		case float64:
			options[key] = int(val)
		case uint:
			options[key] = int(val)
		case uint64:
			options[key] = int(val)
		default:
			return nil, fmt.Errorf("invalid vote count type: %T", v)
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

func (t *TarantoolStorage) AddVote(voteID, userID, option string) error {
	vote, err := t.GetVoting(voteID)
	if err != nil {
		return fmt.Errorf("failed to get voting: %w", err)
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
		return fmt.Errorf("failed to update voting: %w", err)
	}

	return nil
}

func (t *TarantoolStorage) CloseVoting(voteID, userID string) error {
	vote, err := t.GetVoting(voteID)
	if err != nil {
		return fmt.Errorf("failed to get voting: %w", err)
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
		return fmt.Errorf("failed to close voting: %w", err)
	}

	return nil
}

func (t *TarantoolStorage) DeleteVoting(voteID, userID string) error {
	vote, err := t.GetVoting(voteID)
	if err != nil {
		return fmt.Errorf("failed to get voting: %w", err)
	}

	if vote.CreatedBy != userID {
		return fmt.Errorf("только создатель голосования может его удалить")
	}

	_, err = t.DB.Delete("votings", "primary", []interface{}{voteID})
	if err != nil {
		return fmt.Errorf("failed to delete voting: %w", err)
	}

	return nil
}

func (t *TarantoolStorage) VotingExists(id string) (bool, error) {
	resp, err := t.DB.Select("votings", "primary", 0, 1, tarantool.IterEq, []interface{}{id})
	if err != nil {
		return false, fmt.Errorf("failed to check voting existence: %w", err)
	}
	return len(resp.Data) > 0, nil
}
