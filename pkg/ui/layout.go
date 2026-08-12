//go:build !headless

package ui

import (
	"fmt"
	"image"
	"image/color"
	"time"

	"gioui.org/f32"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget/material"
)

// drawWallpaper paints the wallpaper image to fill the entire window using a
// cover fit (scale up/down proportionally and crop the overflow), so the full
// image is always displayed with no distortion.
func (w *Window) drawWallpaper(gtx layout.Context) {
	img := *w.wallpaper
	size := gtx.Constraints.Max

	iw := float32(img.Bounds().Dx())
	ih := float32(img.Bounds().Dy())
	sw := float32(size.X)
	sh := float32(size.Y)

	// Cover-fit scale: the larger of the two axis ratios so the image always
	// covers the window, cropping the longer dimension.
	scale := sw / iw
	if sh/ih > scale {
		scale = sh / ih
	}

	// Center the scaled image by offsetting by half the remaining space.
	dw := iw * scale
	dh := ih * scale
	off := f32.Point{
		X: (sw - dw) / 2,
		Y: (sh - dh) / 2,
	}

	imgOp := paint.NewImageOp(img)
	imgOp.Filter = paint.FilterLinear

	// Clip to the window, then scale+offset the image so the cover-fit
	// transformation is applied and paint. The clip stack must stay pushed
	// until after painting (pushing and immediately popping would leave the
	// paint unclipped).
	clipStack := clip.Rect{Max: size}.Push(gtx.Ops)
	// Push the transform so it only affects the wallpaper ops. Using Add here
	// would leak the transform into every subsequent op of the frame — the
	// input router folds TypeTransform ops into the hit-area transform, so a
	// leaked scale/offset would break pointer and keyboard input for the
	// entire login UI.
	trans := op.Affine(f32.Affine2D{}.Scale(f32.Point{}, f32.Point{X: scale, Y: scale}).Offset(off)).Push(gtx.Ops)
	imgOp.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
	trans.Pop()
	clipStack.Pop()
}

// Render draws the entire immediate-mode user interface on the given frame context
func (w *Window) Render(gtx layout.Context) layout.Dimensions {
	// Paint the wallpaper (cover-fit) when one is available, otherwise fall
	// back to the solid theme background color.
	if w.wallpaper != nil {
		w.drawWallpaper(gtx)
	} else {
		paint.Fill(gtx.Ops, w.theme.Palette.Bg)
	}

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
						clockLbl.Color = w.mutedColor
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
								sub.Color = w.mutedColor
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
									paint.Fill(gtx.Ops, w.avatarColor) // Avatar fill (theme accent)

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
										btn.Background = w.secondaryColor
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
							lbl.Color = w.errorColor
						} else {
							lbl.Color = w.successColor
						}
						lbl.Alignment = text.Middle
						return lbl.Layout(gtx)
					})
				}),
			)
		})
	})
}

// drawLoginPanel renders a padded rounded panel background in Slick Greeter
// style. The background is sized to the widget content (not the window), so
// the card wraps its fields instead of painting over the whole display.
func (w *Window) drawLoginPanel(gtx layout.Context, widget layout.Widget) layout.Dimensions {
	bgColor := w.panelColor
	// Slightly translucent surface lets the wallpaper glow through for a
	// frosted-glass effect; keep it mostly opaque so text stays readable.
	bgColor.A = 0xE6
	cornerRadius := gtx.Dp(unit.Dp(16))

	background := func(gtx layout.Context) layout.Dimensions {
		size := gtx.Constraints.Min
		defer clip.RRect{
			Rect: image.Rectangle{Max: size},
			NE:   cornerRadius, NW: cornerRadius, SE: cornerRadius, SW: cornerRadius,
		}.Push(gtx.Ops).Pop()
		paint.Fill(gtx.Ops, bgColor)
		return layout.Dimensions{Size: size}
	}

	content := func(gtx layout.Context) layout.Dimensions {
		// Keep the card a comfortable width even on very wide displays.
		if maxW := gtx.Dp(unit.Dp(420)); gtx.Constraints.Max.X > maxW {
			gtx.Constraints.Max.X = maxW
		}
		return layout.UniformInset(unit.Dp(30)).Layout(gtx, widget)
	}

	return layout.Background{}.Layout(gtx, background, content)
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
			btn.Background = w.secondaryColor
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
	bgColor := w.secondaryColor
	bgColor.A = 0xF2
	cornerRadius := gtx.Dp(unit.Dp(6))

	background := func(gtx layout.Context) layout.Dimensions {
		size := gtx.Constraints.Min
		defer clip.RRect{
			Rect: image.Rectangle{Max: size},
			NE:   cornerRadius, NW: cornerRadius, SE: cornerRadius, SW: cornerRadius,
		}.Push(gtx.Ops).Pop()
		paint.Fill(gtx.Ops, bgColor)
		return layout.Dimensions{Size: size}
	}

	// Lay options vertically
	content := func(gtx layout.Context) layout.Dimensions {
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
	}

	return layout.Background{}.Layout(gtx, background, func(gtx layout.Context) layout.Dimensions {
		return layout.UniformInset(unit.Dp(5)).Layout(gtx, content)
	})
}
