/*
Copyright 2013 The Camlistore Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

     http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package search

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"camlistore.org/third_party/github.com/gorilla/websocket"
)

const (
	// Time allowed to write a message to the peer.
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer.
	pongWait = 60 * time.Second

	// Send pings to peer with this period. Must be less than pongWait.
	pingPeriod = (pongWait * 9) / 10

	// Maximum message size allowed from peer.
	maxMessageSize = 10 << 10
)

type wsHub struct {
	sh             *Handler
	register       chan *wsConn
	unregister     chan *wsConn
	conns          map[*wsConn]bool
	watchReq       chan watchReq
	newBlobRecv    chan string // new blob received. string is camliType.
	updatedResults chan *watchedQuery
}

func newWebsocketHub(sh *Handler) *wsHub {
	return &wsHub{
		sh:             sh,
		register:       make(chan *wsConn, buffered),
		unregister:     make(chan *wsConn, buffered),
		conns:          make(map[*wsConn]bool),
		watchReq:       make(chan watchReq, buffered),
		newBlobRecv:    make(chan string, buffered),
		updatedResults: make(chan *watchedQuery, buffered),
	}
}

func (h *wsHub) run() {
	for {
		select {
		case c := <-h.register:
			h.conns[c] = true
		case c := <-h.unregister:
			delete(h.conns, c)
			close(c.send)
		case camliType := <-h.newBlobRecv:
			if camliType == "" {
				// TODO: something smarter. some
				// queries might care about all blobs.
				// But for now only re-kick off
				// queries if schema blobs arrive.  We
				// should track per-WatchdQuery which
				// blob types the search cares about.
				continue
			}
			// New blob was received. Kick off standing search queries to see if any changed.
			for conn := range h.conns {
				for _, wq := range conn.queries {
					go h.doSearch(wq)
				}
			}
		case wr := <-h.watchReq:
			// Unsubscribe
			if wr.q == nil {
				delete(wr.conn.queries, wr.tag)
				log.Printf("Removed subscription for %v, %q", wr.conn, wr.tag)
				continue
			}
			// Very similar type, but semantically
			// different, so separate for now:
			wq := &watchedQuery{
				conn: wr.conn,
				tag:  wr.tag,
				q:    wr.q,
			}
			wr.conn.queries[wr.tag] = wq
			log.Printf("Added/updated search subscription for tag %q", wr.tag)
			go h.doSearch(wq)

		case wq := <-h.updatedResults:
			if !h.conns[wq.conn] || wq.conn.queries[wq.tag] == nil {
				// Client has since disconnected or unsubscribed.
				continue
			}
			wq.mu.Lock()
			lastres := wq.lastres
			wq.mu.Unlock()
			resb, err := json.Marshal(wsUpdateMessage{
				Tag:    wq.tag,
				Result: lastres,
			})
			if err != nil {
				panic(err)
			}
			wq.conn.send <- resb
		}
	}
}

func (h *wsHub) doSearch(wq *watchedQuery) {
	// Make our own copy, in case
	q := new(SearchQuery)
	*q = *wq.q // shallow copy, since Query will mutate its internal state fields
	if q.Describe != nil {
		q.Describe = new(DescribeRequest)
		*q.Describe = *wq.q.Describe
	}

	res, err := h.sh.Query(q)
	if err != nil {
		log.Printf("Query error: %v", err)
		return
	}
	resj, _ := json.Marshal(res)

	wq.mu.Lock()
	eq := bytes.Equal(wq.lastresj, resj)
	wq.lastres = res
	wq.lastresj = resj
	wq.mu.Unlock()
	if eq {
		// No change in search. Ignore.
		return
	}
	h.updatedResults <- wq
}

type wsConn struct {
	ws   *websocket.Conn
	send chan []byte // Buffered channel of outbound messages.
	sh   *Handler

	queries map[string]*watchedQuery // tag -> subscription
}

type watchedQuery struct {
	conn *wsConn
	tag  string
	q    *SearchQuery

	mu       sync.Mutex // guards lastRes
	lastres  *SearchResult
	lastresj []byte // as JSON
}

// watchReq is a (un)subscribe request.
type watchReq struct {
	conn *wsConn
	tag  string       // required
	q    *SearchQuery // if nil, subscribe
}

// Client->Server subscription message.
type wsClientMessage struct {
	// Tag is required.
	Tag string `json:"tag"`
	// Query is required to subscribe. If absent, it means unsubscribe.
	Query *SearchQuery `json:"query,omitempty"`
}

type wsUpdateMessage struct {
	Tag    string        `json:"tag"`
	Result *SearchResult `json:"result,omitempty"`
}

// readPump pumps messages from the websocket connection to the hub.
func (c *wsConn) readPump() {
	defer func() {
		c.sh.wsHub.unregister <- c
		c.ws.Close()
	}()
	c.ws.SetReadLimit(maxMessageSize)
	c.ws.SetReadDeadline(time.Now().Add(pongWait))
	c.ws.SetPongHandler(func(string) error { c.ws.SetReadDeadline(time.Now().Add(pongWait)); return nil })
	for {
		_, message, err := c.ws.ReadMessage()
		if err != nil {
			break
		}
		log.Printf("Got websocket message %q", message)
		cm := new(wsClientMessage)
		if err := json.Unmarshal(message, cm); err != nil {
			log.Printf("Ignoring bogus websocket message. Err: %v", err)
			continue
		}
		c.sh.wsHub.watchReq <- watchReq{
			conn: c,
			tag:  cm.Tag,
			q:    cm.Query,
		}
	}
}

// write writes a message with the given message type and payload.
func (c *wsConn) write(mt int, payload []byte) error {
	c.ws.SetWriteDeadline(time.Now().Add(writeWait))
	return c.ws.WriteMessage(mt, payload)
}

// writePump pumps messages from the hub to the websocket connection.
func (c *wsConn) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.ws.Close()
	}()
	for {
		select {
		case message, ok := <-c.send:
			if !ok {
				c.write(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.write(websocket.TextMessage, message); err != nil {
				return
			}
		case <-ticker.C:
			if err := c.write(websocket.PingMessage, []byte{}); err != nil {
				return
			}
		}
	}
}

func (sh *Handler) serveWebSocket(rw http.ResponseWriter, req *http.Request) {
	ws, err := websocket.Upgrade(rw, req, nil, 1024, 1024)
	if _, ok := err.(websocket.HandshakeError); ok {
		http.Error(rw, "Not a websocket handshake", 400)
		return
	} else if err != nil {
		log.Println(err)
		return
	}
	c := &wsConn{
		ws:      ws,
		send:    make(chan []byte, 256),
		sh:      sh,
		queries: make(map[string]*watchedQuery),
	}
	sh.wsHub.register <- c
	go c.writePump()
	c.readPump()
}
