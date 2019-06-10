package proxy

import (
	"gscfg"
	"net/http"

	log "github.com/sirupsen/logrus"

	"fmt"
	"path"
	"time"

	"container/list"

	"github.com/gorilla/websocket"
	"github.com/julienschmidt/httprouter"
	"github.com/rs/cors"
)

const (
	versionCode = 1

	wsReadLimit       = 1024 // 每个websocket的接收数据包长度限制
	wsReadBufferSize  = 2048 // 每个websocket的接收缓冲限制
	wsWriteBufferSize = 4096 // 每个websocket的发送缓冲限制

	myRoomType                    = 1
	gameServerOnlineUserNumPrefix = "wsproxy:"
	proxyServerInstancePrefix     = "proxyserver:"
)

var (
	upgrader = websocket.Upgrader{ReadBufferSize: wsReadBufferSize,
		WriteBufferSize: wsWriteBufferSize, CheckOrigin: func(r *http.Request) bool {
			return true
		}}
	// 根router，只有http server看到
	rootRouter     = httprouter.New()
	pairHolderList = list.New()
)

// 在线玩家数量加1
func incrOnlinePlayerNum() {
	conn := pool.Get()
	defer conn.Close()

	var key = fmt.Sprintf("%s%d", gameServerOnlineUserNumPrefix, myRoomType)
	conn.Do("HINCRBY", key, gscfg.ServerID, 1)
}

// 在线玩家数量减1
func decrOnlinePlayerNum() {
	conn := pool.Get()
	defer conn.Close()

	var key = fmt.Sprintf("%s%d", gameServerOnlineUserNumPrefix, myRoomType)
	conn.Do("HINCRBY", key, gscfg.ServerID, -1)
}

// GetVersion 版本号
func GetVersion() int {
	return versionCode
}

func echoVersion(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	w.Write([]byte(fmt.Sprintf("version:%d", versionCode)))
}

func waitWebsocketMessage(holder *pairHolder, r *http.Request) {
	ws := holder.ws

	ws.SetPongHandler(func(msg string) error {
		//log.Printf("websocket recv ping msg:%s, size:%d\n", msg, len(msg))
		holder.lastReceivedTime = time.Now()
		return nil
	})

	ws.SetPingHandler(func(msg string) error {
		//log.Printf("websocket recv ping msg size:%d\n", len(msg))
		holder.lastReceivedTime = time.Now()
		holder.sendPong(msg)
		return nil
	})

	// 确保无论出任何情况都会调用onWebsocketClosed，以便房间可以做对玩家做离线处理
	defer func() {
		holder.onWebsocketClosed(ws)
	}()

	log.Printf("wait ws msg, peer: %s", r.RemoteAddr)
	for {
		mt, message, err := ws.ReadMessage()
		if err != nil {
			log.Println(" websocket receive error:", err)
			ws.Close()
			break
		}

		holder.lastReceivedTime = time.Now()

		// 只处理BinaryMessage，其他的忽略
		if message != nil && len(message) > 0 && mt == websocket.BinaryMessage {
			holder.onWebsocketMessage(ws, message)
		}

		// log.Printf("receive from user %s message:%v", user.userID(), message)
	}
	log.Printf("ws closed,  peer:%s", r.RemoteAddr)
}

// tryAcceptGameUser 游戏玩家接入
func tryAcceptGameUser(ws *websocket.Conn, r *http.Request) {
	query := r.URL.Query()
	isFromWeb := query.Get("web") == "1"
	target := query.Get("target")
	log.Println("tryAcceptGameUser, target:", target)

	holder := newPairHolder(ws, isFromWeb, target)

	e := pairHolderList.PushBack(holder)

	defer func() {
		pairHolderList.Remove(e)
		decrOnlinePlayerNum()
	}()

	incrOnlinePlayerNum()
	holder.lastReceivedTime = time.Now()

	go holder.proxyStart()
	waitWebsocketMessage(holder, r)
}

// acceptWebsocket 把http请求转换为websocket
func acceptWebsocket(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	var requestPath = r.URL.Path
	requestPath = path.Base(requestPath)

	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}

	// 接收限制
	ws.SetReadLimit(wsReadLimit)

	// 确保 websocket 关闭
	defer ws.Close()

	log.Println("accept websocket:", r.URL)
	switch requestPath {
	case "play":
		tryAcceptGameUser(ws, r)
		break
	}
}

func registerForwardHandlers() {
	rootRouter.Handle("POST", "/t9user/Login", forwardHTTPHandle)
}

// CreateHTTPServer 启动服务器
func CreateHTTPServer() {
	log.Printf("CreateHTTPServer")

	// 所有模块看到的mainRouter
	// 外部访问需要形如/game/uuid/play
	rootRouter.Handle("GET", "/game/:uuid/ws/:wtype", acceptWebsocket)
	rootRouter.Handle("GET", "/game/:uuid/version", echoVersion)

	// POST和GET都要订阅
	rootRouter.Handle("GET", "/game/:uuid/support/*sp", monkeyHTTPHandle)
	rootRouter.Handle("POST", "/game/:uuid/support/*sp", monkeyHTTPHandle)

	upgrader.CheckOrigin = func(r *http.Request) bool { return true }

	registerForwardHandlers()
	redisStartup()

	go acceptHTTPRequest()
	go startAliveKeeper()
}

// acceptHTTPRequest 监听和接受HTTP
func acceptHTTPRequest() {
	c := cors.New(cors.Options{
		AllowOriginFunc: func(origin string) bool {
			return true
		},
	})

	portStr := fmt.Sprintf(":%d", gscfg.ServerPort)
	s := &http.Server{
		Addr:    portStr,
		Handler: c.Handler(rootRouter),
		// ReadTimeout:    10 * time.Second,
		//WriteTimeout:   120 * time.Second,
		MaxHeaderBytes: 1 << 8,
	}

	log.Printf("Http server listen at:%d\n", gscfg.ServerPort)

	err := s.ListenAndServe()
	if err != nil {
		log.Fatalf("Http server ListenAndServe %d failed:%s\n", gscfg.ServerPort, err)
	}
}
