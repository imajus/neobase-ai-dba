package constants

type MessageType string

const (
	MessageTypeUser      MessageType = "user"
	MessageTypeAssistant MessageType = "assistant"
	MessageTypeSystem    MessageType = "system"
)
