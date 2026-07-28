//go:build !headless

package ui

import (
	"fmt"
	"image"
	"image/color"
	"time"

	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget/material"
)

// Render draws the entire immediate-mode user interface on the given frame context
func (w *Window) Render(gtx layout.Context) layout.Dimensions {
	// Fill the entire background with Libadwaita dark charcoal color
	paint.Fill(gtx.Ops, w.theme.Palette.Bg)

	// Outer layout wrapper (centered vertically and horizontally)
	return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{
			Top:    unit.Dp(20),
			Bottom: unit.Dp(20),
			Left:   unit.Dp(40),
			Right:  unit.Dp(40),
		}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {

			// Main login box layout list (Slick Greeter vertical stack)
			return layout.Flex{
				Axis:      layout.Vertical,
				Alignment: layout.Middle,
			}.Layout(gtx,

				// 0. Slick Greeter Clock Widget at Top
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{Bottom: unit.Dp(20)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						timeStr := time.Now().Format("3:04 PM — Monday, January 2")
						clockLbl := material.Body1(w.theme, timeStr)
						clockLbl.Color = color.NRGBA{R: 0xBB, G: 0xBB, B: 0xBB, A: 0xFF}
						clockLbl.Alignment = text.Middle
						return clockLbl.Layout(gtx)
					})
				}),

				// 1. Decorative Header / Slick Branding
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{Bottom: unit.Dp(25)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(gtx,
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								h1 := material.H5(w.theme, "LIGHTDM")
								h1.Color = w.theme.Palette.Fg
								h1.Alignment = text.Middle
								return h1.Layout(gtx)
							}),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								sub := material.Caption(w.theme, "Greeter Ganapati — The Remover of Obstacles")
								sub.Color = color.NRGBA{R: 0x99, G: 0x99, B: 0x99, A: 0xFF}
								sub.Alignment = text.Middle
								return sub.Layout(gtx)
							}),
						)
					})
				}),

				// 2. Main Login Box Container (Slick Greeter Frosted Card)
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return w.drawLoginPanel(gtx, func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{
							Axis:      layout.Vertical,
							Alignment: layout.Middle,
						}.Layout(gtx,

							// A. User Avatar Circle Placeholder (Slick Greeter Profile Icon)
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return layout.Inset{Bottom: unit.Dp(20)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
									size := gtx.Dp(unit.Dp(64))
									cornerRadius := gtx.Dp(unit.Dp(32))

									defer clip.RRect{
										Rect: image.Rectangle{Max: image.Point{X: size, Y: size}},
										NE:   cornerRadius, NW: cornerRadius, SE: cornerRadius, SW: cornerRadius,
									}.Push(gtx.Ops).Pop()
									paint.Fill(gtx.Ops, color.NRGBA{R: 0x35, G: 0x84, B: 0xE4, A: 0xFF}) // Blue avatar bg

									return layout.Dimensions{Size: image.Point{X: size, Y: size}}
								})
							}),

							// B. User Credential Input Fields
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

							// C. Desktop Session Selector Dropdown (Slick Greeter style)
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return layout.Inset{Bottom: unit.Dp(20)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
									return w.drawSessionSelector(gtx)
								})
							}),

							// D. Control Actions (Login, Cancel buttons)
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
										btn.Background = color.NRGBA{R: 0x3E, G: 0x3E, B: 0x3E, A: 0xFF}
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
							lbl.Color = color.NRGBA{R: 0x35, G: 0x84, B: 0xE4, A: 0xFF} // Libadwaita Blue success
						}
						lbl.Alignment = text.Middle
						return lbl.Layout(gtx)
					})
				}),
			)
		})
	})
}

// drawLoginPanel renders a padded rounded panel background in Slick Greeter style
func (w *Window) drawLoginPanel(gtx layout.Context, widget layout.Widget) layout.Dimensions {
	bgColor := color.NRGBA{R: 0x30, G: 0x30, B: 0x30, A: 0xFF} // Libadwaita Panel Gray
	cornerRadius := gtx.Dp(unit.Dp(12))

	defer clip.RRect{
		Rect: image.Rectangle{Max: gtx.Constraints.Max},
		NE:   cornerRadius, NW: cornerRadius, SE: cornerRadius, SW: cornerRadius,
	}.Push(gtx.Ops).Pop()
	paint.Fill(gtx.Ops, bgColor)

	// Internal panel padding
	return layout.UniformInset(unit.Dp(30)).Layout(gtx, widget)
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
			btn.Background = color.NRGBA{R: 0x3E, G: 0x3E, B: 0x3E, A: 0xFF}
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
	bgColor := color.NRGBA{R: 0x3E, G: 0x3E, B: 0x3E, A: 0xFF}
	cornerRadius := gtx.Dp(unit.Dp(6))

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
