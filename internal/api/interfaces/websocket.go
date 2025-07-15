package interfaces

// WebsocketService interface for WebSocket event broadcasting
type WebsocketService interface {
	Emit(event string, data interface{})
}