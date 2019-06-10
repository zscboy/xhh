package proxy

import (
	"fmt"
	"gscfg"
	"time"

	"github.com/garyburd/redigo/redis"
	log "github.com/sirupsen/logrus"
)

var (
	pool *redis.Pool
)

// newPool 新建redis连接池
func newPool(addr string) *redis.Pool {
	return &redis.Pool{
		MaxIdle:     3,
		IdleTimeout: 240 * time.Second,
		Dial:        func() (redis.Conn, error) { return redis.Dial("tcp", addr) },
	}
}

func redisStartup() {
	if gscfg.ServerID == "" {
		log.Panic("Must specify the server ID in config json")
		return
	}

	pool = newPool(gscfg.RedisServer)

	serverRegister()
}

// serverRegister 往redis上登记自己
func serverRegister() {
	// 获取redis链接，并退出函数时释放
	conn := pool.Get()
	defer conn.Close()

	if serverIDSubscriberExist(conn) {
		log.Panicln("The same UUID server instance exists, failed to startup, server ID:", gscfg.ServerID)
		return
	}

	hashKey := proxyServerInstancePrefix + gscfg.ServerID
	conn.Send("MULTI")
	conn.Send("hmset", hashKey, "roomtype", int(myRoomType), "ver", versionCode, "p", gscfg.ServerPort)
	conn.Send("SADD", fmt.Sprintf("%s%d", proxyServerInstancePrefix, int(myRoomType)), gscfg.ServerID)

	// conn.Send("HSET", fmt.Sprintf("%s%d", gconst.RoomTypeKey, myRoomType), "type", 1)
	_, err := conn.Do("EXEC")
	if err != nil {
		log.Panicln("failed to register server to redis:", err)
	}
}

func serverIDSubscriberExist(conn redis.Conn) bool {
	subCounts, err := redis.Int64Map(conn.Do("PUBSUB", "NUMSUB", gscfg.ServerID))
	if err != nil {
		log.Println("warning: serverIDSubscriberExist, redis err:", err)
	}

	count, _ := subCounts[gscfg.ServerID]
	if count > 0 {
		return true
	}

	return false
}
