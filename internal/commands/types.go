package commands

import "time"

type CommandType string

const (
	CommandText        CommandType = "text"
	CommandURL         CommandType = "url"
	CommandFile        CommandType = "file"
	CommandBoardList   CommandType = "board.list"
	CommandBoardCreate CommandType = "board.create"
)

type CommandRequest struct {
	Type     CommandType `json:"type"`
	Board    string      `json:"board,omitempty"`
	Author   string      `json:"author,omitempty"`
	Content  string      `json:"content"`
	MimeType string      `json:"mime_type,omitempty"`
	Filename string      `json:"filename,omitempty"`
}

type CommandResponse struct {
	ID        string    `json:"id,omitempty"`
	Status    string    `json:"status"`
	Message   string    `json:"message,omitempty"`
	Board     string    `json:"board,omitempty"`
	Boards    []string  `json:"boards,omitempty"`
	Timestamp time.Time `json:"timestamp,omitempty"`
}
