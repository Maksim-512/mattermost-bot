package models

// Vote - Структура для хранения информации о голосовании
type Vote struct {
	ID        string         `json:"id"`
	Question  string         `json:"question"`
	Options   map[string]int `json:"options"`    // Ключ - вариант ответа, значение - количество голосов
	CreatedBy string         `json:"created_by"` // Кто создал голосование
	IsClosed  bool           `json:"is_closed"`  // Статус завершено или нет
}
