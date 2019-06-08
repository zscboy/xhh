package proxy

import (
	"fmt"
	"log"
	"net/http"

	"github.com/garyburd/redigo/redis"
	"github.com/julienschmidt/httprouter"
)

type monkeySupportHandler func(w http.ResponseWriter, r *http.Request)

var (
	monkeySupportHandlers = make(map[string]monkeySupportHandler)
)

// monkeyAccountVerify 检查monkey用户接入合法
func monkeyAccountVerify(w http.ResponseWriter, r *http.Request) bool {
	var account = r.URL.Query().Get("account")
	var password = r.URL.Query().Get("password")
	// log.Printf("monkey access, account:%s, password:%s\n", account, password)
	conn := pool.Get()
	defer conn.Close()

	tableName := fmt.Sprintf("%s%d", "xhproxy", 1)
	pass, e := redis.String(conn.Do("HGET", tableName, account))
	if e != nil || password != pass {
		return false
	}

	return true
}

func monkeyHTTPHandle(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	var spName = ps.ByName("sp")

	log.Println("monkey support handler call:", spName)
	if monkeyAccountVerify(w, r) {
		h, ok := monkeySupportHandlers[spName]
		if ok {
			h(w, r)
		} else {
			log.Println("no monkey support handler found:", spName)
		}
	} else {
		var msg = "no authorization for call monkey handler:" + spName
		log.Println(msg)
		w.WriteHeader(404)
		w.Write([]byte(msg))
	}
}
