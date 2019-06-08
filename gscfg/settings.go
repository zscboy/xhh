package gscfg

import (
	"encoding/json"
	"os"

	log "github.com/sirupsen/logrus"

	"github.com/DisposaBoy/JsonConfigReader"
)

// make a copy of this file, rename to settings.go
// then set the correct value for these follow variables
var (
	monitorEstablished = false
	ServerPort         = 3001
	// LogFile              = ""
	Daemon               = "yes"
	RedisServer          = ":6379"
	ServerID             = ""
	RequiredAppModuleVer = 0
	EtcdServer           = ""
	RoomServerID         = ""

	DbIP       = "localhost"
	DbPort     = 1433
	DbUser     = "abc"
	DbPassword = "ab"
	DbName     = "gamedb"

	RoomTypeName string
)

var (
	loadedCfgFilePath = ""
)

// ReLoadConfigFile 重新加载配置
func ReLoadConfigFile() bool {
	log.Println("ReLoadConfigFile-------------------")
	if loadedCfgFilePath == "" {
		log.Println("ReLoadConfigFile-------cfg file path is empty, try load from etcd----")
		// if EtcdServer != "" {
		// 	log.Println("ReLoadConfigFile-----------From ETCD--------:", EtcdServer)
		// 	if !LoadConfigFromEtcd() {
		// 		log.Println("ReLoadConfigFile-------------------FAILED")
		// 		return false
		// 	}

		// 	log.Println("ReLoadConfigFile-------------------OK")
		// 	return true
		// }

		log.Println("ReLoadConfigFile----FAILED:---neigther cfg file path or etcd is valid")
		return false
	}

	log.Println("ReLoadConfigFile-----------From File--------:", loadedCfgFilePath)
	if !ParseConfigFile(loadedCfgFilePath) {
		log.Println("ReLoadConfigFile-------------------FAILED")
		return false
	}

	log.Println("ReLoadConfigFile-------------------OK")
	return true
}

// ParseConfigFile 解析配置
func ParseConfigFile(filepath string) bool {
	type Params struct {
		ServerPort int `json:"port"`
		// LogFile           string `json:"log_file"`
		Daemon      string `json:"daemon"`
		RedisServer string `json:"redis_server"`
		ServreID    string `json:"guid"`
		// URL         string `json:"url"`

		EtcdServer string `json:"etcd"`

		RequiredAppModuleVer int `json:"requiredAppModuleVer"`

		RoomServerID string `json:"roomServerID"`

		DbIP       string `json:"dbIP"`
		DbPort     int    `json:"dbPort"`
		DbPassword string `json:"dbPassword"`
		DbUser     string `json:"dbUser"`
		DbName     string `json:"dbName"`

		RoomTypeName string `json:"roomTypeName"`
	}

	loadedCfgFilePath = filepath

	var params = &Params{}

	f, err := os.Open(filepath)
	if err != nil {
		log.Println("failed to open config file:", filepath)
		return false
	}

	// wrap our reader before passing it to the json decoder
	r := JsonConfigReader.New(f)
	err = json.NewDecoder(r).Decode(params)

	if err != nil {
		log.Println("json un-marshal error:", err)
		return false
	}

	log.Println("-------------------Configure params are:-------------------")
	log.Printf("%+v\n", params)

	// if params.LogFile != "" {
	// 	LogFile = params.LogFile
	// }

	if params.Daemon != "" {
		Daemon = params.Daemon
	}

	if params.ServerPort != 0 {
		ServerPort = params.ServerPort
	}

	if params.RedisServer != "" {
		RedisServer = params.RedisServer
	}

	if params.ServreID != "" {
		ServerID = params.ServreID
	}

	// if params.URL != "" {
	// 	URL = params.URL
	// }

	RoomTypeName = params.RoomTypeName

	if params.RequiredAppModuleVer > 0 {
		RequiredAppModuleVer = params.RequiredAppModuleVer
	}

	if params.RoomServerID != "" {
		RoomServerID = params.RoomServerID
	}

	if params.EtcdServer != "" {
		EtcdServer = params.EtcdServer
	}

	if params.DbIP != "" {
		DbIP = params.DbIP
	}

	if params.DbUser != "" {
		DbUser = params.DbUser
	}

	if params.DbPassword != "" {
		DbPassword = params.DbPassword
	}

	if params.DbName != "" {
		DbName = params.DbName
	}

	if params.DbPort != 0 {
		DbPort = params.DbPort
	}

	if ServerID == "" {
		log.Println("Server id 'guid' must not be empty!")
		return false
	}

	// if EtcdServer != "" {
	// 	if !LoadConfigFromEtcd() {
	// 		return false
	// 	}
	// }

	if RoomServerID == "" {
		log.Println("room server id  must not be empty!")
		return false
	}

	if RedisServer == "" {
		log.Println("redis server id  must not be empty!")
		return false
	}

	return true
}
