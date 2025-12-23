package subs

import (
	"github.com/jinzhu/copier"
)

// DeepCopy creates a deep copy of the Subtitles structure.
// Uses jinzhu/copier for reliable deep copying that automatically handles
// new fields added to astisub types.
func DeepCopy(in *Subtitles) *Subtitles {
	if in == nil || in.Subtitles == nil {
		return nil
	}
	dst := &Subtitles{}
	if err := copier.CopyWithOption(dst, in, copier.Option{DeepCopy: true}); err != nil {
		// This should rarely happen - only for unexportable fields
		// Fall back to shallow copy in case of error
		return &Subtitles{Subtitles: in.Subtitles}
	}
	return dst
}
