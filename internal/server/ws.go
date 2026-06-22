package server

import (
	"context"
	"net/http"
	"time"

	"github.com/wuyuetianjian/kratos-template-for-all/internal/biz"
	"github.com/wuyuetianjian/kratos-template-for-all/internal/conf"

	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/websocket"
)

var wsUpgrader = websocket.Upgrader{
	HandshakeTimeout: 5 * time.Second,
	CheckOrigin:      func(_ *http.Request) bool { return true },
}

func newWSHandler(data *conf.Data, uc *biz.UseCase) http.HandlerFunc {
	apiConf := data.GetApi()
	jwtKey := []byte(apiConf.GetJwtKey())
	signingMethodName := apiConf.GetSigningMethod()

	return func(w http.ResponseWriter, r *http.Request) {
		rawToken := r.URL.Query().Get("token")
		if rawToken == "" {
			http.Error(w, "missing token", http.StatusUnauthorized)
			return
		}

		method, ok := jwtSigningMethod(signingMethodName)
		if !ok {
			http.Error(w, "server misconfigured", http.StatusInternalServerError)
			return
		}

		parsed, err := jwt.Parse(rawToken, func(*jwt.Token) (any, error) {
			return jwtKey, nil
		})
		if err != nil || !parsed.Valid || parsed.Method != method {
			http.Error(w, "invalid token", http.StatusUnauthorized)
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		session, err := uc.FindSessionByToken(ctx, rawToken)
		cancel()
		if err != nil || session.Status != biz.SessionStatusActive {
			http.Error(w, "session not active", http.StatusUnauthorized)
			return
		}

		conn, err := wsUpgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()

		kickCh := uc.Hub().Register(session.ID)
		defer uc.Hub().Unregister(session.ID)

		// Detect client disconnect via read loop.
		conn.SetReadDeadline(time.Now().Add(90 * time.Second))
		conn.SetPongHandler(func(string) error {
			conn.SetReadDeadline(time.Now().Add(90 * time.Second))
			return nil
		})

		readDone := make(chan struct{})
		go func() {
			defer close(readDone)
			for {
				if _, _, err := conn.ReadMessage(); err != nil {
					return
				}
			}
		}()

		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case event := <-kickCh:
				_ = conn.WriteJSON(map[string]string{"type": event})
				return
			case <-ticker.C:
				if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
					return
				}
			case <-readDone:
				return
			case <-r.Context().Done():
				return
			}
		}
	}
}
