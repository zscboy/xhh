package proxy

import (
	"errors"
	"gscfg"
	"net"
	"sync"
	"time"

	proto "github.com/golang/protobuf/proto"
	"github.com/gorilla/websocket"
	log "github.com/sirupsen/logrus"
)

const (
	websocketWriteDeadLine = 5 * time.Second
	tcpWriteDeadLine       = 5 * time.Second
)

// pairHolder hold websocket and tcp pair
type pairHolder struct {
	ws      *websocket.Conn
	tcpConn *net.TCPConn

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
			log.Println("pair holder  ws write err:", err)
			ws.Close()
		}
	}
}

func (ph *pairHolder) send(bytes []byte) error {
	ws := ph.ws
	if ws != nil {
		ph.wsLock.Lock()
		defer ph.wsLock.Unlock()

		ws.SetWriteDeadline(time.Now().Add(websocketWriteDeadLine))
		err := ws.WriteMessage(websocket.BinaryMessage, bytes)
		if err != nil {
			ws.Close()
			log.Println("pair holder ws write err:", err)
		}

		return err
	}

	return errors.New("websocket is nil")
}

func (ph *pairHolder) sendProxyMessage(data []byte, ops int) error {
	d := formatProxyMsgByData(data, int32(ops))
	return ph.send(d)
}

func (ph *pairHolder) closeWebsocket() {
	if ph.ws != nil {
		ph.ws.Close()
	}
}

func (ph *pairHolder) onWebsocketClosed(ws *websocket.Conn) {
	if ws == ph.ws {
		// my websocket has closed
		ph.ws = nil

		tcpConn := ph.tcpConn
		if tcpConn != nil {
			tcpConn.Close()
		}
	}
}

func (ph *pairHolder) onTCPConnClosed(tcpConn *net.TCPConn) {
	if tcpConn == ph.tcpConn {
		// my tcp conn has closed
		ph.tcpConn = nil

		ws := ph.ws
		if ws != nil {
			ws.Close()
		}
	}
}

func (ph *pairHolder) onWebsocketMessage(ws *websocket.Conn, message []byte) {
	tcpConn := ph.tcpConn
	if tcpConn != nil {
		data, err := wsMessage2TcpMessage(message)
		if err != nil {
			log.Println("pair holder onWebsocketMessage wsMessage2TcpMessage failed:", err)
			return
		}

		tcpConn.SetWriteDeadline(time.Now().Add(tcpWriteDeadLine))
		wrote, err := tcpConn.Write(data)

		if err != nil {
			log.Println("pair holder onWebsocketMessage write tcp failed:", err)
			return
		}

		if wrote < len(message) {
			log.Printf("pair holder onWebsocketMessage write tcp, wrote:%d != expected:%d", wrote, len(message))
		}
	}
}

func (ph *pairHolder) proxyStart() error {
	tcpAddr, err := net.ResolveTCPAddr("tcp", gscfg.TCPServer)
	if err != nil {
		log.Println("pair holder ResolveTCPAddr failed:", err)

		return err
	}

	conn, err := net.DialTCP("tcp", nil, tcpAddr)
	if err != nil {
		// handle error
		log.Println("pair holder dial to tcp server failed:", err)

		return err
	}

	ph.tcpConn = conn

	ph.tcpConn.SetNoDelay(true)

	go ph.serveTCP()

	return nil
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
