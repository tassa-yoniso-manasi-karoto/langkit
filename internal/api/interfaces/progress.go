package interfaces

// ProgressBroadcaster interface for broadcasting download progress
type ProgressBroadcaster interface {
	Broadcast(event string, data interface{})
}