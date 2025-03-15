package subs

import (
	"github.com/gookit/color"
	"github.com/k0kubun/pp"
	
	astisub "github.com/asticode/go-astisub"
)

func DeepCopy(in *Subtitles) *Subtitles {
	src := in.Subtitles
	if src == nil {
		return nil
	}

	// Create a new Subtitles instance
	dst := astisub.NewSubtitles()

	// Copy metadata
	if src.Metadata != nil {
		dst.Metadata = &astisub.Metadata{
			Comments:                                            make([]string, len(src.Metadata.Comments)),
			Framerate:                                           src.Metadata.Framerate,
			Language:                                            src.Metadata.Language,
			SSACollisions:                                       src.Metadata.SSACollisions,
			SSAOriginalEditing:                                  src.Metadata.SSAOriginalEditing,
			SSAOriginalScript:                                   src.Metadata.SSAOriginalScript,
			SSAOriginalTiming:                                   src.Metadata.SSAOriginalTiming,
			SSAOriginalTranslation:                              src.Metadata.SSAOriginalTranslation,
			SSAScriptType:                                       src.Metadata.SSAScriptType,
			SSAScriptUpdatedBy:                                  src.Metadata.SSAScriptUpdatedBy,
			SSASynchPoint:                                       src.Metadata.SSASynchPoint,
			SSAUpdateDetails:                                    src.Metadata.SSAUpdateDetails,
			SSAWrapStyle:                                        src.Metadata.SSAWrapStyle,
			STLCountryOfOrigin:                                  src.Metadata.STLCountryOfOrigin,
			STLDisplayStandardCode:                              src.Metadata.STLDisplayStandardCode,
			STLEditorContactDetails:                             src.Metadata.STLEditorContactDetails,
			STLEditorName:                                       src.Metadata.STLEditorName,
			STLOriginalEpisodeTitle:                             src.Metadata.STLOriginalEpisodeTitle,
			STLPublisher:                                        src.Metadata.STLPublisher,
			STLRevisionNumber:                                   src.Metadata.STLRevisionNumber,
			STLSubtitleListReferenceCode:                        src.Metadata.STLSubtitleListReferenceCode,
			STLTimecodeStartOfProgramme:                         src.Metadata.STLTimecodeStartOfProgramme,
			STLTranslatedEpisodeTitle:                           src.Metadata.STLTranslatedEpisodeTitle,
			STLTranslatedProgramTitle:                           src.Metadata.STLTranslatedProgramTitle,
			STLTranslatorContactDetails:                         src.Metadata.STLTranslatorContactDetails,
			STLTranslatorName:                                   src.Metadata.STLTranslatorName,
			Title:                                               src.Metadata.Title,
			TTMLCopyright:                                       src.Metadata.TTMLCopyright,
		}

		// Copy string slice 
		copy(dst.Metadata.Comments, src.Metadata.Comments)

		// Copy pointer values
		if src.Metadata.SSAPlayDepth != nil {
			val := *src.Metadata.SSAPlayDepth
			dst.Metadata.SSAPlayDepth = &val
		}
		if src.Metadata.SSAPlayResX != nil {
			val := *src.Metadata.SSAPlayResX
			dst.Metadata.SSAPlayResX = &val
		}
		if src.Metadata.SSAPlayResY != nil {
			val := *src.Metadata.SSAPlayResY
			dst.Metadata.SSAPlayResY = &val
		}
		if src.Metadata.SSATimer != nil {
			val := *src.Metadata.SSATimer
			dst.Metadata.SSATimer = &val
		}
		if src.Metadata.STLCreationDate != nil {
			val := *src.Metadata.STLCreationDate
			dst.Metadata.STLCreationDate = &val
		}
		if src.Metadata.STLMaximumNumberOfDisplayableCharactersInAnyTextRow != nil {
			val := *src.Metadata.STLMaximumNumberOfDisplayableCharactersInAnyTextRow
			dst.Metadata.STLMaximumNumberOfDisplayableCharactersInAnyTextRow = &val
		}
		if src.Metadata.STLMaximumNumberOfDisplayableRows != nil {
			val := *src.Metadata.STLMaximumNumberOfDisplayableRows
			dst.Metadata.STLMaximumNumberOfDisplayableRows = &val
		}
		if src.Metadata.STLRevisionDate != nil {
			val := *src.Metadata.STLRevisionDate
			dst.Metadata.STLRevisionDate = &val
		}
		if src.Metadata.WebVTTTimestampMap != nil {
			dst.Metadata.WebVTTTimestampMap = &astisub.WebVTTTimestampMap{
				Local:  src.Metadata.WebVTTTimestampMap.Local,
				MpegTS: src.Metadata.WebVTTTimestampMap.MpegTS,
			}
		}
	}

	// Copy regions
	if len(src.Regions) > 0 {
		dst.Regions = make(map[string]*astisub.Region, len(src.Regions))
		for k, v := range src.Regions {
			if v == nil {
				continue
			}
			dst.Regions[k] = &astisub.Region{
				ID: v.ID,
			}
			if v.InlineStyle != nil {
				dst.Regions[k].InlineStyle = copyStyleAttributes(v.InlineStyle)
			}
			if v.Style != nil {
				dst.Regions[k].Style = copyStyle(v.Style)
			}
		}
	}

	// Copy styles
	if len(src.Styles) > 0 {
		dst.Styles = make(map[string]*astisub.Style, len(src.Styles))
		for k, v := range src.Styles {
			if v == nil {
				continue
			}
			dst.Styles[k] = copyStyle(v)
		}
	}

	// Copy items
	dst.Items = make([]*astisub.Item, len(src.Items))
	for i, item := range src.Items {
		if item == nil {
			continue
		}
		
		dstItem := &astisub.Item{
			EndAt:   item.EndAt,
			Index:   item.Index,
			StartAt: item.StartAt,
			Comments: make([]string, len(item.Comments)),
		}
		
		// Copy comments
		copy(dstItem.Comments, item.Comments)
		
		// Copy inline style
		if item.InlineStyle != nil {
			dstItem.InlineStyle = copyStyleAttributes(item.InlineStyle)
		}
		
		// Copy region reference
		if item.Region != nil {
			if regionID := item.Region.ID; regionID != "" && dst.Regions[regionID] != nil {
				dstItem.Region = dst.Regions[regionID]
			} else {
				dstItem.Region = &astisub.Region{
					ID: item.Region.ID,
				}
				if item.Region.InlineStyle != nil {
					dstItem.Region.InlineStyle = copyStyleAttributes(item.Region.InlineStyle)
				}
				if item.Region.Style != nil {
					dstItem.Region.Style = copyStyle(item.Region.Style)
				}
			}
		}
		
		// Copy style reference
		if item.Style != nil {
			if styleID := item.Style.ID; styleID != "" && dst.Styles[styleID] != nil {
				dstItem.Style = dst.Styles[styleID]
			} else {
				dstItem.Style = copyStyle(item.Style)
			}
		}
		
		// Copy lines
		dstItem.Lines = make([]astisub.Line, len(item.Lines))
		for j, line := range item.Lines {
			dstLine := astisub.Line{
				VoiceName: line.VoiceName,
				Items:     make([]astisub.LineItem, len(line.Items)),
			}
			
			// Copy line items
			for k, lineItem := range line.Items {
				dstLineItem := astisub.LineItem{
					StartAt: lineItem.StartAt,
					Text:    lineItem.Text,
				}
				
				if lineItem.InlineStyle != nil {
					dstLineItem.InlineStyle = copyStyleAttributes(lineItem.InlineStyle)
				}
				
				if lineItem.Style != nil {
					if styleID := lineItem.Style.ID; styleID != "" && dst.Styles[styleID] != nil {
						dstLineItem.Style = dst.Styles[styleID]
					} else {
						dstLineItem.Style = copyStyle(lineItem.Style)
					}
				}
				
				dstLine.Items[k] = dstLineItem
			}
			
			dstItem.Lines[j] = dstLine
		}
		
		dst.Items[i] = dstItem
	}

	return &Subtitles{dst}
}

// Helper function to deep copy a Style
func copyStyle(src *astisub.Style) *astisub.Style {
	if src == nil {
		return nil
	}
	
	dst := &astisub.Style{
		ID: src.ID,
	}
	
	if src.InlineStyle != nil {
		dst.InlineStyle = copyStyleAttributes(src.InlineStyle)
	}
	
	if src.Style != nil {
		dst.Style = copyStyle(src.Style)
	}
	
	return dst
}

// Helper function to deep copy StyleAttributes
func copyStyleAttributes(src *astisub.StyleAttributes) *astisub.StyleAttributes {
	if src == nil {
		return nil
	}
	
	dst := &astisub.StyleAttributes{
		SRTBold:       src.SRTBold,
		SRTItalics:    src.SRTItalics,
		SRTPosition:   src.SRTPosition,
		SRTUnderline:  src.SRTUnderline,
		SSAEffect:     src.SSAEffect,
		SSAFontName:   src.SSAFontName,
		WebVTTAlign:   src.WebVTTAlign,
		WebVTTBold:    src.WebVTTBold,
		WebVTTItalics: src.WebVTTItalics,
		WebVTTLine:    src.WebVTTLine,
		WebVTTLines:   src.WebVTTLines,
		WebVTTPosition: src.WebVTTPosition,
		WebVTTRegionAnchor: src.WebVTTRegionAnchor,
		WebVTTScroll:       src.WebVTTScroll,
		WebVTTSize:         src.WebVTTSize,
		WebVTTUnderline:    src.WebVTTUnderline,
		WebVTTVertical:     src.WebVTTVertical,
		WebVTTViewportAnchor: src.WebVTTViewportAnchor,
		WebVTTWidth:          src.WebVTTWidth,
	}
	
	// Copy pointer string fields
	if src.SRTColor != nil {
		val := *src.SRTColor
		dst.SRTColor = &val
	}
	
	// Copy WebVTT styles slice
	if len(src.WebVTTStyles) > 0 {
		dst.WebVTTStyles = make([]string, len(src.WebVTTStyles))
		copy(dst.WebVTTStyles, src.WebVTTStyles)
	}
	
	// Copy WebVTT tags
	if len(src.WebVTTTags) > 0 {
		dst.WebVTTTags = make([]astisub.WebVTTTag, len(src.WebVTTTags))
		for i, tag := range src.WebVTTTags {
			dst.WebVTTTags[i] = astisub.WebVTTTag{
				Name:       tag.Name,
				Annotation: tag.Annotation,
			}
			if len(tag.Classes) > 0 {
				dst.WebVTTTags[i].Classes = make([]string, len(tag.Classes))
				copy(dst.WebVTTTags[i].Classes, tag.Classes)
			}
		}
	}
	
	// Copy Color pointers
	if src.SSABackColour != nil {
		dst.SSABackColour = copyColor(src.SSABackColour)
	}
	if src.SSAOutlineColour != nil {
		dst.SSAOutlineColour = copyColor(src.SSAOutlineColour)
	}
	if src.SSAPrimaryColour != nil {
		dst.SSAPrimaryColour = copyColor(src.SSAPrimaryColour)
	}
	if src.SSASecondaryColour != nil {
		dst.SSASecondaryColour = copyColor(src.SSASecondaryColour)
	}
	if src.TeletextColor != nil {
		dst.TeletextColor = copyColor(src.TeletextColor)
	}
	
	// Copy numeric pointers
	if src.SSAAlignment != nil {
		val := *src.SSAAlignment
		dst.SSAAlignment = &val
	}
	if src.SSAAlphaLevel != nil {
		val := *src.SSAAlphaLevel
		dst.SSAAlphaLevel = &val
	}
	if src.SSAAngle != nil {
		val := *src.SSAAngle
		dst.SSAAngle = &val
	}
	if src.SSABold != nil {
		val := *src.SSABold
		dst.SSABold = &val
	}
	if src.SSABorderStyle != nil {
		val := *src.SSABorderStyle
		dst.SSABorderStyle = &val
	}
	if src.SSAEncoding != nil {
		val := *src.SSAEncoding
		dst.SSAEncoding = &val
	}
	if src.SSAFontSize != nil {
		val := *src.SSAFontSize
		dst.SSAFontSize = &val
	}
	if src.SSAItalic != nil {
		val := *src.SSAItalic
		dst.SSAItalic = &val
	}
	if src.SSALayer != nil {
		val := *src.SSALayer
		dst.SSALayer = &val
	}
	if src.SSAMarginLeft != nil {
		val := *src.SSAMarginLeft
		dst.SSAMarginLeft = &val
	}
	if src.SSAMarginRight != nil {
		val := *src.SSAMarginRight
		dst.SSAMarginRight = &val
	}
	if src.SSAMarginVertical != nil {
		val := *src.SSAMarginVertical
		dst.SSAMarginVertical = &val
	}
	if src.SSAMarked != nil {
		val := *src.SSAMarked
		dst.SSAMarked = &val
	}
	if src.SSAOutline != nil {
		val := *src.SSAOutline
		dst.SSAOutline = &val
	}
	if src.SSAScaleX != nil {
		val := *src.SSAScaleX
		dst.SSAScaleX = &val
	}
	if src.SSAScaleY != nil {
		val := *src.SSAScaleY
		dst.SSAScaleY = &val
	}
	if src.SSAShadow != nil {
		val := *src.SSAShadow
		dst.SSAShadow = &val
	}
	if src.SSASpacing != nil {
		val := *src.SSASpacing
		dst.SSASpacing = &val
	}
	if src.SSAStrikeout != nil {
		val := *src.SSAStrikeout
		dst.SSAStrikeout = &val
	}
	if src.SSAUnderline != nil {
		val := *src.SSAUnderline
		dst.SSAUnderline = &val
	}
	
	// Copy STL specific fields
	if src.STLBoxing != nil {
		val := *src.STLBoxing
		dst.STLBoxing = &val
	}
	if src.STLItalics != nil {
		val := *src.STLItalics
		dst.STLItalics = &val
	}
	if src.STLJustification != nil {
		val := *src.STLJustification
		dst.STLJustification = &val
	}
	if src.STLPosition != nil {
		dst.STLPosition = &astisub.STLPosition{
			VerticalPosition: src.STLPosition.VerticalPosition,
			MaxRows:          src.STLPosition.MaxRows,
			Rows:             src.STLPosition.Rows,
		}
	}
	if src.STLUnderline != nil {
		val := *src.STLUnderline
		dst.STLUnderline = &val
	}
	
	// Copy Teletext specific fields
	if src.TeletextDoubleHeight != nil {
		val := *src.TeletextDoubleHeight
		dst.TeletextDoubleHeight = &val
	}
	if src.TeletextDoubleSize != nil {
		val := *src.TeletextDoubleSize
		dst.TeletextDoubleSize = &val
	}
	if src.TeletextDoubleWidth != nil {
		val := *src.TeletextDoubleWidth
		dst.TeletextDoubleWidth = &val
	}
	if src.TeletextSpacesAfter != nil {
		val := *src.TeletextSpacesAfter
		dst.TeletextSpacesAfter = &val
	}
	if src.TeletextSpacesBefore != nil {
		val := *src.TeletextSpacesBefore
		dst.TeletextSpacesBefore = &val
	}
	
	// Copy TTML style attributes
	if src.TTMLBackgroundColor != nil {
		val := *src.TTMLBackgroundColor
		dst.TTMLBackgroundColor = &val
	}
	if src.TTMLColor != nil {
		val := *src.TTMLColor
		dst.TTMLColor = &val
	}
	if src.TTMLDirection != nil {
		val := *src.TTMLDirection
		dst.TTMLDirection = &val
	}
	if src.TTMLDisplay != nil {
		val := *src.TTMLDisplay
		dst.TTMLDisplay = &val
	}
	if src.TTMLDisplayAlign != nil {
		val := *src.TTMLDisplayAlign
		dst.TTMLDisplayAlign = &val
	}
	if src.TTMLExtent != nil {
		val := *src.TTMLExtent
		dst.TTMLExtent = &val
	}
	if src.TTMLFontFamily != nil {
		val := *src.TTMLFontFamily
		dst.TTMLFontFamily = &val
	}
	if src.TTMLFontSize != nil {
		val := *src.TTMLFontSize
		dst.TTMLFontSize = &val
	}
	if src.TTMLFontStyle != nil {
		val := *src.TTMLFontStyle
		dst.TTMLFontStyle = &val
	}
	if src.TTMLFontWeight != nil {
		val := *src.TTMLFontWeight
		dst.TTMLFontWeight = &val
	}
	if src.TTMLLineHeight != nil {
		val := *src.TTMLLineHeight
		dst.TTMLLineHeight = &val
	}
	if src.TTMLOpacity != nil {
		val := *src.TTMLOpacity
		dst.TTMLOpacity = &val
	}
	if src.TTMLOrigin != nil {
		val := *src.TTMLOrigin
		dst.TTMLOrigin = &val
	}
	if src.TTMLOverflow != nil {
		val := *src.TTMLOverflow
		dst.TTMLOverflow = &val
	}
	if src.TTMLPadding != nil {
		val := *src.TTMLPadding
		dst.TTMLPadding = &val
	}
	if src.TTMLShowBackground != nil {
		val := *src.TTMLShowBackground
		dst.TTMLShowBackground = &val
	}
	if src.TTMLTextAlign != nil {
		val := *src.TTMLTextAlign
		dst.TTMLTextAlign = &val
	}
	if src.TTMLTextDecoration != nil {
		val := *src.TTMLTextDecoration
		dst.TTMLTextDecoration = &val
	}
	if src.TTMLTextOutline != nil {
		val := *src.TTMLTextOutline
		dst.TTMLTextOutline = &val
	}
	if src.TTMLUnicodeBidi != nil {
		val := *src.TTMLUnicodeBidi
		dst.TTMLUnicodeBidi = &val
	}
	if src.TTMLVisibility != nil {
		val := *src.TTMLVisibility
		dst.TTMLVisibility = &val
	}
	if src.TTMLWrapOption != nil {
		val := *src.TTMLWrapOption
		dst.TTMLWrapOption = &val
	}
	if src.TTMLWritingMode != nil {
		val := *src.TTMLWritingMode
		dst.TTMLWritingMode = &val
	}
	if src.TTMLZIndex != nil {
		val := *src.TTMLZIndex
		dst.TTMLZIndex = &val
	}
	
	return dst
}

// Helper function to deep copy a Color
func copyColor(src *astisub.Color) *astisub.Color {
	if src == nil {
		return nil
	}
	return &astisub.Color{
		Alpha: src.Alpha,
		Blue:  src.Blue,
		Green: src.Green,
		Red:   src.Red,
	}
}


func placeholder2() {
	color.Redln(" 𝒻*** 𝓎ℴ𝓊 𝒸ℴ𝓂𝓅𝒾𝓁ℯ𝓇")
	pp.Println("𝓯*** 𝔂𝓸𝓾 𝓬𝓸𝓶𝓹𝓲𝓵𝓮𝓻")
}

