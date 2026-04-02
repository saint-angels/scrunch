package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"image/color"
	"log"
	"os"
	"sync"
	"time"

	"image"
	"strings"

	"gioui.org/app"
	"gioui.org/font"
	"gioui.org/font/gofont"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"github.com/gorilla/websocket"
)

type Standing struct {
	User      string `json:"user"`
	StartedAt int64  `json:"startedAt"`
	EndsAt    int64  `json:"endsAt"`
}

type ServerMsg struct {
	Type      string     `json:"type"`
	User      string     `json:"user,omitempty"`
	StartedAt int64      `json:"startedAt,omitempty"`
	EndsAt    int64      `json:"endsAt,omitempty"`
	Reason    string     `json:"reason,omitempty"`
	Standings []Standing `json:"standings,omitempty"`
}

type State struct {
	mu        sync.Mutex
	connected bool
	standings []Standing
}

func (s *State) SetConnected(v bool) {
	s.mu.Lock()
	s.connected = v
	s.mu.Unlock()
}

func (s *State) IsConnected() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.connected
}

func (s *State) Apply(msg ServerMsg) {
	s.mu.Lock()
	defer s.mu.Unlock()

	switch msg.Type {
	case "STATE_SYNC":
		s.standings = msg.Standings
	case "STAND_STARTED":
		s.standings = filterUser(s.standings, msg.User)
		s.standings = append(s.standings, Standing{User: msg.User, StartedAt: msg.StartedAt, EndsAt: msg.EndsAt})
	case "STAND_ENDED":
		s.standings = filterUser(s.standings, msg.User)
	}
}

func (s *State) GetStandings() []Standing {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Standing, len(s.standings))
	copy(out, s.standings)
	return out
}

func (s *State) IsStanding(user string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, st := range s.standings {
		if st.User == user {
			return true
		}
	}
	return false
}

func filterUser(standings []Standing, user string) []Standing {
	out := standings[:0]
	for _, s := range standings {
		if s.User != user {
			out = append(out, s)
		}
	}
	return out
}

var (
	userName = flag.String("user", "", "your display name")
	serverAddr = flag.String("server", "localhost:9000", "server address")
	duration   = flag.Int("duration", 2700, "standing duration in seconds")
)

func main() {
	flag.Parse()
	if *userName == "" {
		fmt.Fprintln(os.Stderr, "usage: scrunch-client -user <name>")
		os.Exit(1)
	}

	state := &State{}
	sendCh := make(chan []byte, 16)

	go wsLoop(*serverAddr, state, sendCh)

	go func() {
		w := new(app.Window)
		w.Option(app.Title("Scrunch - " + *userName))
		w.Option(app.Size(unit.Dp(240), unit.Dp(220)))

		if err := runUI(w, state, sendCh); err != nil {
			log.Fatal(err)
		}
		os.Exit(0)
	}()

	app.Main()
}

func wsLoop(addr string, state *State, sendCh chan []byte) {
	for {
		url := fmt.Sprintf("ws://%s/", addr)
		conn, _, err := websocket.DefaultDialer.Dial(url, nil)
		if err != nil {
			state.SetConnected(false)
			time.Sleep(2 * time.Second)
			continue
		}
		state.SetConnected(true)

		// Writer goroutine
		done := make(chan struct{})
		go func() {
			defer close(done)
			for {
				select {
				case msg := <-sendCh:
					if err := conn.WriteMessage(websocket.TextMessage, msg); err != nil {
						return
					}
				case <-done:
					return
				}
			}
		}()

		// Reader loop
		for {
			_, raw, err := conn.ReadMessage()
			if err != nil {
				break
			}
			var msg ServerMsg
			if json.Unmarshal(raw, &msg) == nil {
				state.Apply(msg)
			}
		}

		conn.Close()
		state.SetConnected(false)
		time.Sleep(2 * time.Second)
	}
}

type colorScheme struct {
	bg  color.NRGBA
	fg  color.NRGBA
	dim color.NRGBA
}

var (
	sittingColors = colorScheme{
		bg:  color.NRGBA{R: 0x0A, G: 0x0A, B: 0x1A, A: 255},
		fg:  color.NRGBA{R: 0x66, G: 0x88, B: 0xAA, A: 255},
		dim: color.NRGBA{R: 0x66, G: 0x88, B: 0xAA, A: 128},
	}
	standingColors = colorScheme{
		bg:  color.NRGBA{R: 0x00, G: 0x18, B: 0x30, A: 255},
		fg:  color.NRGBA{R: 0x00, G: 0xDD, B: 0xFF, A: 255},
		dim: color.NRGBA{R: 0x00, G: 0xDD, B: 0xFF, A: 128},
	}
)

func monoLabel(th *material.Theme, size unit.Sp, txt string, col color.NRGBA) material.LabelStyle {
	l := material.Label(th, size, strings.ToUpper(txt))
	l.Color = col
	l.Font.Typeface = "Go Mono"
	l.Font.Weight = font.Bold
	return l
}

func drawDivider(gtx layout.Context, col color.NRGBA) layout.Dimensions {
	height := gtx.Dp(unit.Dp(1))
	if height < 1 {
		height = 1
	}
	sz := image.Point{X: gtx.Constraints.Max.X, Y: height}
	defer clip.Rect{Max: sz}.Push(gtx.Ops).Pop()
	paint.ColorOp{Color: col}.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
	return layout.Dimensions{Size: sz}
}

func borderedButton(gtx layout.Context, th *material.Theme, btn *widget.Clickable, label string, fg, bg color.NRGBA) layout.Dimensions {
	return btn.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Top: unit.Dp(6), Bottom: unit.Dp(6), Left: unit.Dp(16), Right: unit.Dp(16)}.Layout(gtx,
			func(gtx layout.Context) layout.Dimensions {
				l := monoLabel(th, unit.Sp(10), label, fg)
				dims := l.Layout(gtx)

				// Draw border around the full button area
				borderSize := image.Point{
					X: dims.Size.X + gtx.Dp(unit.Dp(32)),
					Y: dims.Size.Y + gtx.Dp(unit.Dp(12)),
				}
				borderRect := clip.Stroke{
					Path:  clip.Rect{Max: borderSize}.Path(),
					Width: float32(gtx.Dp(unit.Dp(1))),
				}.Op().Push(gtx.Ops)
				paint.ColorOp{Color: fg}.Add(gtx.Ops)
				paint.PaintOp{}.Add(gtx.Ops)
				borderRect.Pop()

				return dims
			},
		)
	})
}

func runUI(w *app.Window, state *State, sendCh chan []byte) error {
	th := material.NewTheme()
	th.Shaper = text.NewShaper(text.NoSystemFonts(), text.WithCollection(gofont.Collection()))
	var toggleBtn widget.Clickable
	var ops op.Ops
	pending := false
	lastStanding := false

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	go func() {
		for range ticker.C {
			w.Invalidate()
		}
	}()

	for {
		switch e := w.Event().(type) {
		case app.DestroyEvent:
			return e.Err
		case app.FrameEvent:
			gtx := app.NewContext(&ops, e)
			standing := state.IsStanding(*userName)

			// Clear pending when server confirms the state change
			if standing != lastStanding {
				pending = false
				lastStanding = standing
			}

			cs := sittingColors
			if standing {
				cs = standingColors
			}

			if toggleBtn.Clicked(gtx) && !pending {
				pending = true
				var msg map[string]any
				if standing {
					msg = map[string]any{"type": "SIT", "user": *userName}
				} else {
					msg = map[string]any{"type": "STAND", "user": *userName, "duration": *duration}
				}
				data, _ := json.Marshal(msg)
				sendCh <- data
			}

			// My elapsed timer
			myTime := "0:00"
			if standing {
				for _, s := range state.GetStandings() {
					if s.User == *userName {
						elapsed := time.Since(time.UnixMilli(s.StartedAt))
						if elapsed < 0 {
							elapsed = 0
						}
						myTime = fmt.Sprintf("%d:%02d", int(elapsed.Minutes()), int(elapsed.Seconds())%60)
						break
					}
				}
			}

			layout.Stack{}.Layout(gtx,
				// Background
				layout.Expanded(func(gtx layout.Context) layout.Dimensions {
					sz := gtx.Constraints.Max
					defer clip.Rect{Max: sz}.Push(gtx.Ops).Pop()
					paint.ColorOp{Color: cs.bg}.Add(gtx.Ops)
					paint.PaintOp{}.Add(gtx.Ops)
					return layout.Dimensions{Size: sz}
				}),
				// Content
				layout.Stacked(func(gtx layout.Context) layout.Dimensions {
					return layout.UniformInset(unit.Dp(12)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
							// Username
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								l := monoLabel(th, unit.Sp(11), *userName, cs.fg)
								return l.Layout(gtx)
							}),
							// State label
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								stateStr := "SITTING"
								if standing {
									stateStr = "STANDING"
								}
								l := monoLabel(th, unit.Sp(9), stateStr, cs.dim)
								return layout.Inset{Top: unit.Dp(1)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
									return l.Layout(gtx)
								})
							}),
							// Timer
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								l := monoLabel(th, unit.Sp(36), myTime, cs.fg)
								return layout.Inset{Top: unit.Dp(8), Bottom: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
									return l.Layout(gtx)
								})
							}),
							// Button
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								label := "STAND UP"
								if standing {
									label = "SIT DOWN"
								}
								btnFg := cs.fg
								if pending {
									label = "..."
									btnFg = cs.dim
								}
								return layout.Inset{Bottom: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
									return borderedButton(gtx, th, &toggleBtn, label, btnFg, cs.bg)
								})
							}),
							// Divider
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								divColor := cs.fg
								divColor.A = 50
								return layout.Inset{Top: unit.Dp(4), Bottom: unit.Dp(4)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
									return drawDivider(gtx, divColor)
								})
							}),
							// Others standing
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								standings := state.GetStandings()
								if len(standings) == 0 {
									return layout.Dimensions{}
								}
								list := layout.List{Axis: layout.Vertical}
								return list.Layout(gtx, len(standings), func(gtx layout.Context, i int) layout.Dimensions {
									s := standings[i]
									elapsed := time.Since(time.UnixMilli(s.StartedAt))
									if elapsed < 0 {
										elapsed = 0
									}
									timeStr := fmt.Sprintf("%d:%02d", int(elapsed.Minutes()), int(elapsed.Seconds())%60)

									return layout.Inset{Top: unit.Dp(2), Bottom: unit.Dp(2)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
										return layout.Flex{Spacing: layout.SpaceBetween}.Layout(gtx,
											layout.Rigid(func(gtx layout.Context) layout.Dimensions {
												return monoLabel(th, unit.Sp(9), s.User, cs.fg).Layout(gtx)
											}),
											layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
												l := monoLabel(th, unit.Sp(9), timeStr, cs.dim)
												l.Alignment = text.End
												return l.Layout(gtx)
											}),
										)
									})
								})
							}),
							// Spacer
							layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
								return layout.Dimensions{Size: image.Point{Y: gtx.Constraints.Min.Y}}
							}),
							// Connection status
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								statusStr := "DISCONNECTED"
								statusColor := cs.dim
								if state.IsConnected() {
									statusStr = "CONNECTED"
								}
								l := monoLabel(th, unit.Sp(8), statusStr, statusColor)
								l.Alignment = text.End
								return l.Layout(gtx)
							}),
						)
					})
				}),
			)
			e.Frame(gtx.Ops)
		}
	}
}
