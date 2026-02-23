package net_ws

import (
	"encoding/json"
	"errors"
	"kasper/src/abstract"
	"kasper/src/shell/layer1/adapters"
	module_model "kasper/src/shell/layer1/model"

	"kasper/src/shell/layer1/tools/security"
	"kasper/src/shell/layer1/tools/signaler"
	net_http "kasper/src/shell/layer3/tools/network/http"
	"log"

	"strings"
	"sync"
	"time"

	"github.com/gofiber/contrib/websocket"
)

type WebsocketAnswer struct {
	Status    int
	RequestId string
	Data      any
}

type WsServer struct {
	Tokens map[string]string
}

func (q *Queue) AnswerSocket(conn *websocket.Conn, t string, requestId string, answer any) {
	q.Lock.Lock()
	defer q.Lock.Unlock()
	answerBytes, err0 := json.Marshal(answer)
	if err0 != nil {
		log.Println(err0)
		return
	}
	if err := conn.WriteMessage(websocket.TextMessage, []byte(t+" "+requestId+" "+string(answerBytes))); err != nil {
		log.Println(err)
		return
	}
}

func ParseInput[T abstract.IInput](i string) (abstract.IInput, error) {
	body := new(T)
	err := json.Unmarshal([]byte(i), body)
	if err != nil {
		return nil, errors.New("invalid input format")
	}
	return *body, nil
}

func (ws *WsServer) PrepareAnswer(answer any) []byte {
	answerBytes, err0 := json.Marshal(answer)
	if err0 != nil {
		log.Println(err0)
		return nil
	}
	return answerBytes
}

type Queue struct {
	Lock sync.Mutex
	Ack  bool
	Data []any
}

var queue = map[string]*Queue{}

var cfg = websocket.Config{
	RecoverHandler: func(conn *websocket.Conn) {
		if err := recover(); err != nil {
			log.Println(err)
		}
	},
}

func connectListener(ws *WsServer, signaler *signaler.Signaler, token string, uid string, conn *websocket.Conn) (*module_model.Listener, *Queue) {
	signaler.Lock.Lock()
	defer signaler.Lock.Unlock()
	var lis *module_model.Listener
	ws.Tokens[uid] = token
	lisTemp, found := signaler.Listeners.Get(uid)
	if found {
		lis = lisTemp
	}
	q := queue[uid]
	if lis == nil {
		q = &Queue{Data: []any{}, Ack: true}
		queue[uid] = q
	}
	lis = &module_model.Listener{
		Id:      uid,
		Paused:  false,
		DisTime: 0,
		Signal: func(b any) {
			q.Lock.Lock()
			defer q.Lock.Unlock()
			if b != nil {
				q.Data = append(q.Data, b)
			}
			if len(q.Data) > 0 {
				if q.Ack {
					q.Ack = false
					err := conn.WriteMessage(websocket.TextMessage, q.Data[0].([]byte))
					if err != nil {
						q.Ack = true
						log.Println(err)
					}
				}
			}
		},
	}
	q.Ack = true
	return lis, q
}

func (ws *WsServer) Load(core abstract.ICore, httpServer *net_http.HttpServer, security *security.Security, signaler *signaler.Signaler, storage adapters.IStorage) {
	httpServer.Server.Get("/ws", websocket.New(func(conn *websocket.Conn) {
		defer func() {
			if err := recover(); err != nil {
				log.Println(err)
			}
		}()
		var uid string = ""
		var lis *module_model.Listener
		var q *Queue
		processPacket := func() int {
			conn.SetReadDeadline(time.Now().Add(30 * time.Second))
			_, p, err := conn.ReadMessage()
			if err != nil {
				return 1
			}
			var dataStr = string(p[:])
			if dataStr == "packet_received" {
				send := func() {
					q.Lock.Lock()
					defer q.Lock.Unlock()
					q.Ack = true
					if len(q.Data) > 0 {
						q.Data = q.Data[1:]
						if len(q.Data) > 0 {
							q.Ack = false
							err := conn.WriteMessage(websocket.TextMessage, q.Data[0].([]byte))
							if err != nil {
								q.Ack = true
								log.Println(err)
							}
						}
					}
				}
				send()
				return 0
			} else if dataStr == "KeepAlive" {
				send := func() {
					q.Lock.Lock()
					defer q.Lock.Unlock()
					err := conn.WriteMessage(websocket.TextMessage, []byte("KeepAlive"))
					if err != nil {
						log.Println(err)
					}
				}
				send()
				return 0
			}
			var splittedMsg = strings.Split(dataStr, " ")
			var uri = splittedMsg[0]
			if len(splittedMsg) > 1 {
				if uri == "authenticate" {
					if len(splittedMsg) >= 3 {
						var token = splittedMsg[1]
						var requestId = splittedMsg[2]
						userId, _, _ := security.AuthWithToken(token)
						if userId != "" {
							uid = userId
							lis, q = connectListener(ws, signaler, token, uid, conn)
							q.AnswerSocket(conn, "response", requestId, module_model.ResponseSimpleMessage{Message: "authenticated"})
							oldQueueEndPack, _ := json.Marshal(module_model.ResponseSimpleMessage{Message: "old_queue_end"})
							signaler.ListenToSingle(lis)
							lis.Signal(oldQueueEndPack)
						} else {
							q.AnswerSocket(conn, "error", requestId, module_model.ResponseSimpleMessage{Message: "authentication failed"})
						}
					}
				}
			}
			return 0
		}
		for {
			if processPacket() != 0 {
				if lis != nil {
					lis.Paused = true
					lis.DisTime = time.Now().UnixMilli()
					time.Sleep(time.Duration(62) * time.Second)
					dispose := func() {
						signaler.Lock.Lock()
						defer signaler.Lock.Unlock()
						l, found := signaler.Listeners.Get(uid)
						if !found {
							delete(queue, uid)
							signaler.Listeners.Remove(uid)
							delete(ws.Tokens, uid)
							log.Println("client queue disposed")
						} else if l.Paused {
							if (time.Now().UnixMilli() - l.DisTime) > (60 * 1000) {
								delete(queue, uid)
								signaler.Listeners.Remove(uid)
								delete(ws.Tokens, uid)
								log.Println("client queue disposed")
							}
						}
					}
					dispose()
					log.Println("socket broken")
				}
				break
			}
		}
	}, cfg))
}

func New(core abstract.ICore, httpServer *net_http.HttpServer, security *security.Security, signaler *signaler.Signaler, storage adapters.IStorage) *WsServer {
	ws := &WsServer{Tokens: make(map[string]string)}
	ws.Load(core, httpServer, security, signaler, storage)
	return ws
}
