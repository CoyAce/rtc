package ui

import (
	"image"
	"image/color"
	"rtc/assets/fonts"
	"strings"

	"gioui.org/font"
	"gioui.org/io/clipboard"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"golang.org/x/exp/shiny/materialdesign/colornames"
	"golang.org/x/exp/shiny/materialdesign/icons"
)

type accountForm struct {
	Theme                 *material.Theme
	InActiveTheme         *material.Theme
	iconCreateNewID       *widget.Icon
	iconImportFile        *widget.Icon
	pvtKeyStr             string
	title                 string
	importLabelText       string
	btnClear              IconButtonX
	btnNewID              IconButtonX
	btnSubmitImportKey    IconButtonX
	btnPasteKey           IconButtonX
	navigationIcon        *widget.Icon
	errorCreateNewID      error
	errorImportKey        error
	creatingNewID         bool
	submittingImportedKey bool
	OnSuccess             func()
	*ModalContent
	Modal
}

func NewAccountFormView(theme *material.Theme, onSuccess func()) View {
	clearIcon, _ := widget.NewIcon(icons.ContentClear)
	navIcon, _ := widget.NewIcon(icons.NavigationArrowBack)
	iconCreateNewID, _ := widget.NewIcon(icons.ActionDone)
	iconImportFile, _ := widget.NewIcon(icons.FileFileUpload)
	pasteIcon, _ := widget.NewIcon(icons.ContentContentPaste)
	errorTh := *fonts.NewTheme()
	errorTh.ContrastBg = color.NRGBA(colornames.Red500)
	inActiveTh := *fonts.NewTheme()
	inActiveTh.ContrastBg = color.NRGBA(colornames.Grey500)
	s := accountForm{
		Theme:           theme,
		InActiveTheme:   &inActiveTh,
		title:           "Account",
		navigationIcon:  navIcon,
		iconCreateNewID: iconCreateNewID,
		iconImportFile:  iconImportFile,
		importLabelText: "Import Key",
		OnSuccess:       onSuccess,
		btnSubmitImportKey: IconButtonX{
			Theme: theme,
			Icon:  iconCreateNewID,
			Text:  "Submit",
		},
		btnPasteKey: IconButtonX{
			Theme: theme,
			Icon:  pasteIcon,
			Text:  "Paste",
		},
		btnNewID: IconButtonX{
			Theme: theme,
			Icon:  iconCreateNewID,
			Text:  "Auto Create New Account",
		},
		btnClear: IconButtonX{
			Theme: theme,
			Icon:  clearIcon,
			Text:  "Clear",
		},
	}
	s.ModalContent = NewModalContent(func() {
		s.Modal.Dismiss(nil)
		s.creatingNewID = false
		s.submittingImportedKey = false
		if s.OnSuccess != nil {
			s.OnSuccess()
		}
	})
	return &s
}

func (p *accountForm) Layout(gtx layout.Context) layout.Dimensions {
	if p.Theme == nil {
		p.Theme = fonts.NewTheme()
	}

	inset := layout.UniformInset(unit.Dp(16))
	flex := layout.Flex{Axis: layout.Vertical, Alignment: layout.Start}
	d := flex.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			inset := inset
			return inset.Layout(gtx, p.drawImportKeyTextField)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			bd := material.Body1(p.Theme, "Or")
			bd.Font.Weight = font.Bold
			bd.Alignment = text.Middle
			bd.TextSize = unit.Sp(20)
			return bd.Layout(gtx)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			inset := inset
			return inset.Layout(gtx, p.drawAutoCreateField)
		}),
	)
	if p.creatingNewID || p.submittingImportedKey {
		layout.Stack{}.Layout(gtx,
			layout.Stacked(func(gtx layout.Context) layout.Dimensions {
				loader := material.Loader(p.Theme)
				gtx.Constraints.Max, gtx.Constraints.Min = d.Size, d.Size
				return layout.Flex{Alignment: layout.Middle, Spacing: layout.SpaceSides}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return loader.Layout(gtx)
					}))
			}),
		)
		return d
	}
	return d
}

func (p *accountForm) drawImportKeyTextField(gtx layout.Context) layout.Dimensions {
	if p.btnPasteKey.Button.Clicked(gtx) {
		gtx.Execute(clipboard.ReadCmd{Tag: &p.btnPasteKey})
	}

	if p.btnClear.Button.Clicked(gtx) {
		p.pvtKeyStr = ""
		p.errorImportKey = nil
	}

	flex := layout.Flex{Axis: layout.Vertical}
	return flex.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			var txt string
			txt = strings.TrimSpace(p.pvtKeyStr)
			txtColor := p.Theme.Fg
			if txt == "" {
				txt = "Paste key file contents here"
				txtColor = color.NRGBA(colornames.Grey500)
			}
			if p.errorImportKey != nil {
				txt = p.errorImportKey.Error()
				txtColor = color.NRGBA(colornames.Red500)
			}
			inset := layout.UniformInset(unit.Dp(16))
			mac := op.Record(gtx.Ops)
			d := inset.Layout(gtx,
				func(gtx layout.Context) layout.Dimensions {
					lbl := material.Label(p.Theme, p.Theme.TextSize, txt)
					lbl.MaxLines = 10
					lbl.Color = txtColor
					return lbl.Layout(gtx)
				})
			stop := mac.Stop()
			bounds := image.Rect(0, 0, d.Size.X, d.Size.Y)
			rect := clip.UniformRRect(bounds, gtx.Dp(4))
			paint.FillShape(gtx.Ops,
				p.Theme.Fg,
				clip.Stroke{Path: rect.Path(gtx.Ops), Width: float32(gtx.Dp(1))}.Op(),
			)
			stop.Add(gtx.Ops)
			return d
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			mobileWidth := gtx.Dp(350)
			flex := layout.Flex{Spacing: layout.SpaceBetween}
			spacerLayout := layout.Spacer{Width: unit.Dp(16)}
			submitLayout := layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return p.btnSubmitImportKey.Layout(gtx)
			})
			pasteLayout := layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return p.btnPasteKey.Layout(gtx)
			})
			clearLayout := layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return p.btnClear.Layout(gtx)
			})
			if gtx.Constraints.Max.X <= mobileWidth {
				flex.Axis = layout.Vertical
				spacerLayout.Width = 0
				spacerLayout.Height = 8
				submitLayout = layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return p.btnSubmitImportKey.Layout(gtx)
				})
				pasteLayout = layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return p.btnPasteKey.Layout(gtx)
				})
				clearLayout = layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return p.btnClear.Layout(gtx)
				})
			}
			inset := layout.Inset{Top: unit.Dp(16)}
			return inset.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return flex.Layout(gtx,
					submitLayout,
					layout.Rigid(spacerLayout.Layout),
					pasteLayout,
					layout.Rigid(spacerLayout.Layout),
					clearLayout,
				)
			})
		}),
	)

}

func (p *accountForm) drawAutoCreateField(gtx layout.Context) layout.Dimensions {
	var button *IconButtonX
	if p.errorCreateNewID != nil {
		button = &IconButtonX{
			Theme: p.InActiveTheme,
			Icon:  p.iconCreateNewID,
			Text:  "Auto Create New Account",
		}
	} else {
		button = &p.btnNewID
	}
	if p.btnNewID.Button.Clicked(gtx) && !p.creatingNewID {
		p.creatingNewID = true
	}
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	flex := layout.Flex{Axis: layout.Vertical}
	return flex.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			flex := layout.Flex{Spacing: layout.SpaceEnd}
			inset := layout.Inset{Top: unit.Dp(16)}
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			return inset.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return flex.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						gtx.Constraints.Min.X = gtx.Constraints.Max.X
						return button.Layout(gtx)
					}),
				)
			})
		}),
	)
}

type IconButtonX struct {
	*material.Theme
	Button widget.Clickable
	Icon   *widget.Icon
	Text   string
	layout.Inset
}

func (b *IconButtonX) Layout(gtx layout.Context) layout.Dimensions {
	btnLayoutStyle := material.ButtonLayout(b.Theme, &b.Button)
	btnLayoutStyle.CornerRadius = unit.Dp(8)
	return btnLayoutStyle.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		inset := b.Inset
		if b.Inset == (layout.Inset{}) {
			inset = layout.UniformInset(unit.Dp(12))
		}
		return inset.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			iconAndLabel := layout.Flex{Alignment: layout.Middle, Spacing: layout.SpaceSides}
			textIconSpacer := unit.Dp(5)

			layIcon := layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Right: textIconSpacer}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					var d layout.Dimensions
					if b.Icon != nil {
						size := gtx.Dp(56.0 / 2.5)
						d = layout.Dimensions{Size: image.Pt(size, size)}
						gtx.Constraints = layout.Exact(d.Size)
						d = b.Icon.Layout(gtx, b.Theme.ContrastFg)
					}
					return d
				})
			})

			layLabel := layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Left: textIconSpacer}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					l := material.Label(b.Theme, b.Theme.TextSize, b.Text)
					l.Alignment = text.Middle
					l.Color = b.Theme.Palette.ContrastFg
					return l.Layout(gtx)
				})
			})

			return iconAndLabel.Layout(gtx, layIcon, layLabel)
		})
	})
}
