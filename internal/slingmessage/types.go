package slingmessage

import "time"

type SlingMessage struct {
	ID        string    `json:"id"`        // Unique message identifier
	Sender    string    `json:"sender"`    // Identifier of the sender (e.g., username or ID)
	Timestamp time.Time `json:"timestamp"` // Time the message was created
	MimeType  string    `json:"mime_type"` // MIME type of the content (e.g., text/plain, image/png, application/pdf)
	Content   []byte    `json:"content"`   // Arbitrary content (text, images, PDFs, etc.)
}
