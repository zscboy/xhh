package proxy

import (
	"net"
	"sync"
	"time"

	proto "github.com/golang/protobuf/proto"
	"github.com/gorilla/websocket"
	log "github.com/sirupsen/logrus"
)

const (
	websocketWriteDeadLine = 5 * time.Second
)

// pairHolder hold websocket and tcp pair
type pairHolder struct {
	ws      *websocket.Conn
	tcpSock *net.TCPConn

	lastReceivedTime time.Time
	lastPingTime     time.Time

	wsLock *sync.Mutex // websocket并发写锁

	// 如果是浏览器，其websocket没有原生的ping/pong
	// 需要自定义ping pong实现
	isFromWeb bool
}

func newPairHolder(ws *websocket.Conn, isFromWeb bool) *pairHolder {
	hodler := &pairHolder{}
	hodler.ws = ws
	hodler.isFromWeb = isFromWeb

	return hodler
}

func (ph *pairHolder) sendPong(msg string) {
	ws := ph.ws
	if ws != nil {
		ph.wsLock.Lock()
		defer ph.wsLock.Unlock()

		if len(msg) == 0 {
			msg = "kr"
		}

		ws.SetWriteDeadline(time.Now().Add(websocketWriteDeadLine))
		err := ws.WriteMessage(websocket.PongMessage, []byte(msg))
		if err != nil {
			log.Println("pair holder ws write err:", err)
			ws.Close()
		}
	}
}

func (ph *pairHolder) sendPing() {
	ws := ph.ws
	if ws != nil {
		ph.wsLock.Lock()
		defer ph.wsLock.Unlock()

		ws.SetWriteDeadline(time.Now().Add(websocketWriteDeadLine))

		var err error
		if ph.isFromWeb {
			buf := formatProxyMsgByData([]byte("ka"), int32(MessageCode_OPPing))
			ws.WriteMessage(websocket.BinaryMessage, buf)
		} else {
			err = ws.WriteMessage(websocket.PingMessage, []byte("ka"))
		}

		if err != nil {
			log.Printf("user %s ws write err:", err)
			ws.Close()
		}
	}
}

func (ph *pairHolder) onWebsocketClosed(ws *websocket.Conn) {

}

func (ph *pairHolder) onWebsocketMessage(ws *websocket.Conn, message []byte) {

}

func formatProxyMsgByData(data []byte, ops int32) []byte {
	gmsg := &ProxyMessage{}
	gmsg.Ops = &ops

	gmsg.Data = data

	bytes, err := proto.Marshal(gmsg)
	if err != nil {
		log.Println("marshal game msg failed:", err)
		return nil
	}

	return bytes
}
