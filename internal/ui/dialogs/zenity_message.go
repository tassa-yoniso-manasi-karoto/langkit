package dialogs

import (
	"github.com/ncruces/zenity"
)

// Compile-time check that ZenityMessageDialog implements MessageDialog
var _ MessageDialog = (*ZenityMessageDialog)(nil)

// ZenityMessageDialog implements MessageDialog using zenity
type ZenityMessageDialog struct{}

// NewZenityMessageDialog creates a new Zenity message dialog instance
func NewZenityMessageDialog() *ZenityMessageDialog {
	return &ZenityMessageDialog{}
}

// ShowMessage displays a message dialog using zenity
func (z *ZenityMessageDialog) ShowMessage(title, message string, msgType MessageType) (bool, error) {
	opts := []zenity.Option{
		zenity.Title(title),
	}

	var err error
	switch msgType {
	case MessageInfo:
		err = zenity.Info(message, opts...)
	case MessageWarning:
		err = zenity.Warning(message, opts...)
	case MessageError:
		err = zenity.Error(message, opts...)
	case MessageQuestion:
		err = zenity.Question(message, opts...)
		if err == nil {
			return true, nil // User clicked OK/Yes
		}
		if err == zenity.ErrCanceled {
			return false, nil // User clicked Cancel/No
		}
		return false, err
	default:
		err = zenity.Info(message, opts...)
	}

	if err == zenity.ErrCanceled {
		return false, nil
	}
	return true, err
}
