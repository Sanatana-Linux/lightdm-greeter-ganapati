//go:build !headless

package ui

import (
	"fmt"
	"image"
	"image/color"

	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget/material"
)

// Render draws the entire immediate-mode user interface on the given frame context
func (w *Window) Render(gtx layout.Context) layout.Dimensions {
	// Fill the entire background with charcoal dark color
	paint.Fill(gtx.Ops, w.theme.Palette.Bg)

	// Outer layout wrapper (centered vertically and horizontally)
	return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{
			Top:    unit.Dp(20),
			Bottom: unit.Dp(20),
			Left:   unit.Dp(40),
			Right:  unit.Dp(40),
		}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {

			// Main login box layout list
			return layout.Flex{
				Axis:      layout.Vertical,
				Alignment: layout.Middle,
			}.Layout(gtx,

				// 1. Decorative Header Logo/Brand
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{Bottom: unit.Dp(30)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						h1 := material.H4(w.theme, "ELEPHANT GREETER")
						h1.Color = w.theme.Palette.Fg
						h1.Alignment = text.Middle
						return h1.Layout(gtx)
					})
				}),

				// 2. Main Login Box Container (Rounded panel with dark accent background)
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return w.drawLoginPanel(gtx, func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{
							Axis:      layout.Vertical,
							Alignment: layout.Middle,
						}.Layout(gtx,

							// A. User Credential Input Fields
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								if !w.needPassword {
									// Draw Username field
									return layout.Inset{Bottom: unit.Dp(15)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
										ed := material.Editor(w.theme, &w.usernameEditor, "Username")
										ed.Color = w.theme.Palette.Fg
										return ed.Layout(gtx)
									})
								} else {
									// Draw Password prompt and field
									return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
										layout.Rigid(func(gtx layout.Context) layout.Dimensions {
											return layout.Inset{Bottom: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
												lbl := material.Body2(w.theme, w.promptText)
												lbl.Color = w.theme.Palette.Fg
												return lbl.Layout(gtx)
											})
										}),
										layout.Rigid(func(gtx layout.Context) layout.Dimensions {
											return layout.Inset{Bottom: unit.Dp(15)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
												ed := material.Editor(w.theme, &w.passwordEditor, "Password")
												ed.Color = w.theme.Palette.Fg
												return ed.Layout(gtx)
											})
										}),
									)
								}
							}),

							// B. Desktop Session Selector Dropdown
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return layout.Inset{Bottom: unit.Dp(20)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
									return w.drawSessionSelector(gtx)
								})
							}),

							// C. Control Actions (Login, Cancel buttons)
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return layout.Flex{Axis: layout.Horizontal, Spacing: layout.SpaceBetween}.Layout(gtx,
									layout.Rigid(func(gtx layout.Context) layout.Dimensions {
										btnText := "Sign In"
										if w.isAuthenticating && !w.needPassword {
											btnText = "Verifying..."
										}
										btn := material.Button(w.theme, &w.loginClick, btnText)
										btn.Background = w.theme.Palette.ContrastBg
										btn.Color = w.theme.Palette.ContrastFg
										return btn.Layout(gtx)
									}),
									layout.Rigid(layout.Spacer{Width: unit.Dp(15)}.Layout),
									layout.Rigid(func(gtx layout.Context) layout.Dimensions {
										btn := material.Button(w.theme, &w.cancelClick, "Reset")
										btn.Background = color.NRGBA{R: 0x33, G: 0x33, B: 0x33, A: 0xFF}
										btn.Color = w.theme.Palette.Fg
										return btn.Layout(gtx)
									}),
								)
							}),
						)
					})
				}),

				// 3. Informational/Error Status Text Bar
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					if w.statusMsg == "" {
						return layout.Dimensions{}
					}
					return layout.Inset{Top: unit.Dp(25)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						lbl := material.Body2(w.theme, w.statusMsg)
						if w.isError {
							lbl.Color = color.NRGBA{R: 0xFF, G: 0x44, B: 0x44, A: 0xFF} // Bright Alert Red
						} else {
							lbl.Color = color.NRGBA{R: 0x44, G: 0xC2, B: 0x44, A: 0xFF} // Green success
						}
						lbl.Alignment = text.Middle
						return lbl.Layout(gtx)
					})
				}),
			)
		})
	})
}

// drawLoginPanel renders a padded rounded panel background with a subtle border outline
func (w *Window) drawLoginPanel(gtx layout.Context, widget layout.Widget) layout.Dimensions {
	bgColor := color.NRGBA{R: 0x22, G: 0x22, B: 0x22, A: 0xFF} // Accent Panel Dark Gray
	cornerRadius := gtx.Dp(unit.Dp(8))

	defer clip.RRect{
		Rect: image.Rectangle{Max: gtx.Constraints.Max},
		NE:   cornerRadius, NW: cornerRadius, SE: cornerRadius, SW: cornerRadius,
	}.Push(gtx.Ops).Pop()
	paint.Fill(gtx.Ops, bgColor)

	// Internal panel padding
	return layout.UniformInset(unit.Dp(25)).Layout(gtx, widget)
}

// drawSessionSelector draws the current active session name and exposes a dropdown layout if toggled open
func (w *Window) drawSessionSelector(gtx layout.Context) layout.Dimensions {
	activeSessName := "No Sessions Found"
	if w.currentSess < len(w.sessions) {
		activeSessName = fmt.Sprintf("Session: %s (%s)", w.sessions[w.currentSess].Name, w.sessions[w.currentSess].Type)
	}

	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			btn := material.Button(w.theme, &w.sessMenuClick, activeSessName)
			btn.Background = color.NRGBA{R: 0x2D, G: 0x2D, B: 0x2D, A: 0xFF}
			btn.Color = w.theme.Palette.Fg
			return btn.Layout(gtx)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if !w.showSessions {
				return layout.Dimensions{}
			}
			// Draw dropdown options list below
			return layout.Inset{Top: unit.Dp(5)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return w.drawDropdownMenu(gtx)
			})
		}),
	)
}

// drawDropdownMenu renders session choices inside an absolute stacked box overlay
func (w *Window) drawDropdownMenu(gtx layout.Context) layout.Dimensions {
	bgColor := color.NRGBA{R: 0x2D, G: 0x2D, B: 0x2D, A: 0xFF}
	cornerRadius := gtx.Dp(unit.Dp(4))

	defer clip.RRect{
		Rect: image.Rectangle{Max: gtx.Constraints.Max},
		NE:   cornerRadius, NW: cornerRadius, SE: cornerRadius, SW: cornerRadius,
	}.Push(gtx.Ops).Pop()
	paint.Fill(gtx.Ops, bgColor)

	// Lay options vertically
	return layout.UniformInset(unit.Dp(5)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		flexItems := make([]layout.FlexChild, len(w.sessions))
		for i := range w.sessions {
			idx := i
			flexItems[i] = layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				btn := material.Button(w.theme, &w.sessionClicks[idx], w.sessions[idx].Name)
				if idx == w.currentSess {
					btn.Background = w.theme.Palette.ContrastBg
					btn.Color = w.theme.Palette.ContrastFg
				} else {
					btn.Background = color.NRGBA{}
					btn.Color = w.theme.Palette.Fg
				}
				return btn.Layout(gtx)
			})
		}
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, flexItems...)
	})
}
