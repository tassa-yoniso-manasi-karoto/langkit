package browser

// URLOpener interface provides runtime-agnostic URL opening
type URLOpener interface {
	OpenURL(url string) error
}