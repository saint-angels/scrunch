package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"image/color"
	"log"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"

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
	"github.com/gen2brain/beeep"
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
	Users     []string   `json:"users,omitempty"`
}

type State struct {
	mu        sync.Mutex
	connected bool
	standings []Standing
	users     []string
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

func notify(title, body string) {
	beeep.Notify(title, body, "")
}

func (s *State) Apply(msg ServerMsg, self string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	switch msg.Type {
	case "STATE_SYNC":
		s.standings = msg.Standings
		s.users = msg.Users
	case "USER_JOINED":
		s.users = msg.Users
	case "USER_LEFT":
		s.users = msg.Users
	case "STAND_STARTED":
		s.standings = filterUser(s.standings, msg.User)
		s.standings = append(s.standings, Standing{User: msg.User, StartedAt: msg.StartedAt, EndsAt: msg.EndsAt})
		if msg.User != self {
			notify("Scrunch", msg.User+" stood up")
		}
	case "STAND_ENDED":
		s.standings = filterUser(s.standings, msg.User)
		if msg.User != self {
			notify("Scrunch", msg.User+" sat down")
		}
	case "TIME_UP":
		if msg.User == self {
			notify("Scrunch", "Your standing time is up!")
		} else {
			notify("Scrunch", msg.User+"'s time is up")
		}
	}
}

func (s *State) GetUsers() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]string, len(s.users))
	copy(out, s.users)
	return out
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

func defaultUser() string {
	if runtime.GOOS == "windows" {
		return "karen"
	}
	return "misha"
}

var (
	userName   = flag.String("user", defaultUser(), "your display name")
	serverAddr = flag.String("server", "admins-mac-mini:9000", "server address")
	duration   = flag.Int("duration", 2700, "standing duration in seconds")
)

func main() {
	flag.Parse()

	state := &State{}
	sendCh := make(chan []byte, 16)

	go wsLoop(*serverAddr, state, sendCh)

	go func() {
		w := new(app.Window)
		w.Option(app.Title("Scrunch - " + *userName))
		w.Option(app.Size(unit.Dp(240), unit.Dp(220)))
		w.Option(app.MaxSize(unit.Dp(240), unit.Dp(220)))
		w.Option(app.MinSize(unit.Dp(240), unit.Dp(220)))

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

		// Send JOIN
		join, _ := json.Marshal(map[string]any{"type": "JOIN", "user": *userName})
		conn.WriteMessage(websocket.TextMessage, join)

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
				state.Apply(msg, *userName)
			}
		}

		conn.Close()
		state.SetConnected(false)
		time.Sleep(2 * time.Second)
	}
}

// Win98 color palette
var (
	colBg         = color.NRGBA{R: 0xD4, G: 0xD0, B: 0xC8, A: 255}
	colText       = color.NRGBA{R: 0x00, G: 0x00, B: 0x00, A: 255}
	colDim        = color.NRGBA{R: 0x80, G: 0x80, B: 0x80, A: 255}
	colHighlight  = color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 255}
	colShadow     = color.NRGBA{R: 0x80, G: 0x80, B: 0x80, A: 255}
	colDarkShadow = color.NRGBA{R: 0x40, G: 0x40, B: 0x40, A: 255}
	colSunkenBg   = color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 255}
)

func drawEdge(ops *op.Ops, min, max image.Point, col color.NRGBA) {
	s := clip.Rect{Min: min, Max: max}.Push(ops)
	paint.ColorOp{Color: col}.Add(ops)
	paint.PaintOp{}.Add(ops)
	s.Pop()
}

func drawRaisedBorder(ops *op.Ops, sz image.Point) {
	drawEdge(ops, image.Point{}, image.Point{X: sz.X, Y: 1}, colHighlight)
	drawEdge(ops, image.Point{}, image.Point{X: 1, Y: sz.Y}, colHighlight)
	drawEdge(ops, image.Point{Y: sz.Y - 1}, sz, colDarkShadow)
	drawEdge(ops, image.Point{X: sz.X - 1}, sz, colDarkShadow)
}

func drawSunkenBorder(ops *op.Ops, sz image.Point) {
	drawEdge(ops, image.Point{}, image.Point{X: sz.X, Y: 1}, colShadow)
	drawEdge(ops, image.Point{}, image.Point{X: 1, Y: sz.Y}, colShadow)
	drawEdge(ops, image.Point{Y: sz.Y - 1}, sz, colHighlight)
	drawEdge(ops, image.Point{X: sz.X - 1}, sz, colHighlight)
}

func drawGroove(gtx layout.Context) layout.Dimensions {
	sz := image.Point{X: gtx.Constraints.Max.X, Y: 2}
	drawEdge(gtx.Ops, image.Point{}, image.Point{X: sz.X, Y: 1}, colShadow)
	drawEdge(gtx.Ops, image.Point{Y: 1}, sz, colHighlight)
	return layout.Dimensions{Size: sz}
}

func sunkenPanel(gtx layout.Context, w layout.Widget) layout.Dimensions {
	m := op.Record(gtx.Ops)
	dims := layout.Inset{Top: unit.Dp(2), Bottom: unit.Dp(2), Left: unit.Dp(4), Right: unit.Dp(4)}.Layout(gtx, w)
	call := m.Stop()

	sz := dims.Size
	s := clip.Rect{Max: sz}.Push(gtx.Ops)
	paint.ColorOp{Color: colSunkenBg}.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
	s.Pop()
	drawSunkenBorder(gtx.Ops, sz)
	call.Add(gtx.Ops)
	return dims
}

func win98Button(gtx layout.Context, th *material.Theme, btn *widget.Clickable, label string, disabled bool) layout.Dimensions {
	return btn.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		m := op.Record(gtx.Ops)
		dims := layout.Inset{Top: unit.Dp(4), Bottom: unit.Dp(4), Left: unit.Dp(16), Right: unit.Dp(16)}.Layout(gtx,
			func(gtx layout.Context) layout.Dimensions {
				col := colText
				if disabled {
					col = colDim
				}
				l := material.Label(th, unit.Sp(11), strings.ToUpper(label))
				l.Color = col
				l.Font.Weight = font.Bold
				return l.Layout(gtx)
			})
		call := m.Stop()

		sz := dims.Size
		s := clip.Rect{Max: sz}.Push(gtx.Ops)
		paint.ColorOp{Color: colBg}.Add(gtx.Ops)
		paint.PaintOp{}.Add(gtx.Ops)
		s.Pop()
		drawRaisedBorder(gtx.Ops, sz)
		call.Add(gtx.Ops)
		return dims
	})
}

func w98Label(th *material.Theme, size unit.Sp, txt string, col color.NRGBA) material.LabelStyle {
	l := material.Label(th, size, strings.ToUpper(txt))
	l.Color = col
	l.Font.Weight = font.Bold
	return l
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
					paint.ColorOp{Color: colBg}.Add(gtx.Ops)
					paint.PaintOp{}.Add(gtx.Ops)
					return layout.Dimensions{Size: sz}
				}),
				// Content
				layout.Stacked(func(gtx layout.Context) layout.Dimensions {
					return layout.UniformInset(unit.Dp(12)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
							// Username
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								l := w98Label(th, unit.Sp(11), *userName, colText)
								return l.Layout(gtx)
							}),
							// State label
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								stateStr := "SITTING"
								if standing {
									stateStr = "STANDING"
								}
								l := w98Label(th, unit.Sp(9), stateStr, colDim)
								return layout.Inset{Top: unit.Dp(1)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
									return l.Layout(gtx)
								})
							}),
							// Timer
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return layout.Inset{Top: unit.Dp(8), Bottom: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
									return sunkenPanel(gtx, func(gtx layout.Context) layout.Dimensions {
										l := w98Label(th, unit.Sp(36), myTime, colText)
										l.Font.Typeface = "Go Mono"
										return l.Layout(gtx)
									})
								})
							}),
							// Button
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								label := "STAND UP"
								if standing {
									label = "SIT DOWN"
								}
								if pending {
									label = "..."
								}
								return layout.Inset{Bottom: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
									return win98Button(gtx, th, &toggleBtn, label, pending)
								})
							}),
							// Divider
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return layout.Inset{Top: unit.Dp(4), Bottom: unit.Dp(4)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
									return drawGroove(gtx)
								})
							}),
							// Other users
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								users := state.GetUsers()
								standings := state.GetStandings()
								standingMap := make(map[string]Standing)
								for _, s := range standings {
									standingMap[s.User] = s
								}
								var others []string
								for _, u := range users {
									if u != *userName {
										others = append(others, u)
									}
								}
								if len(others) == 0 {
									return layout.Dimensions{}
								}
								list := layout.List{Axis: layout.Vertical}
								return list.Layout(gtx, len(others), func(gtx layout.Context, i int) layout.Dimensions {
									u := others[i]
									statusStr := "SITTING"
									statusCol := colDim
									if s, ok := standingMap[u]; ok {
										elapsed := time.Since(time.UnixMilli(s.StartedAt))
										if elapsed < 0 {
											elapsed = 0
										}
										statusStr = fmt.Sprintf("%d:%02d", int(elapsed.Minutes()), int(elapsed.Seconds())%60)
										statusCol = colText
									}
									return layout.Inset{Top: unit.Dp(2), Bottom: unit.Dp(2)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
										return layout.Flex{Spacing: layout.SpaceBetween}.Layout(gtx,
											layout.Rigid(func(gtx layout.Context) layout.Dimensions {
												return w98Label(th, unit.Sp(9), u, colText).Layout(gtx)
											}),
											layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
												l := w98Label(th, unit.Sp(9), statusStr, statusCol)
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
								if state.IsConnected() {
									statusStr = "CONNECTED"
								}
								l := w98Label(th, unit.Sp(8), statusStr, colDim)
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
