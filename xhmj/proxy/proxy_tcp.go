package proxy

import (
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"errors"
	"io/ioutil"
	"net"
	"time"

	log "github.com/sirupsen/logrus"
)

const (
	packHeaderSize = 12
	minimalSize    = 256
	flagCompressed = 0x40
)

type packetHeader struct {
	msg      uint16 // 消息码
	flag     byte   // 包标志，目前用于判断包是否压缩
	compress byte   // 没有用，是否压缩需要根据flag判断
	size     uint32 // 包体大小
	hash     uint32 // hash
}

func (ph *pairHolder) readRequiredBytes(buf []byte, requiredSize int) error {
	conn := ph.tcpConn
	read, err := conn.Read(buf[0:requiredSize])
	if read == 0 {
		// pear shudown, get a FIN package
		log.Println("serveTCP, tcp server finished connection")
		return err
	}

	if err != nil {
		log.Printf("serveTCP read error:%v, addr:%v", err, conn.RemoteAddr())
		return err
	}

	if read != requiredSize {
		log.Printf("serveTCP read error, ret:%d != required:%d", read, requiredSize)
		return errors.New("ret not equal required")
	}

	return nil
}

func (ph *pairHolder) serveTCP() {
	conn := ph.tcpConn
	defer func() {
		conn.Close()
		ph.onTCPConnClosed(conn)
	}()

	log.Println("serveTCP for:", conn.RemoteAddr())

	buf := make([]byte, minimalSize)

	for {
		// read packet header
		err := ph.readRequiredBytes(buf, packHeaderSize)
		if err != nil {
			log.Println("serveTCP read packet header failed:", err)
			break
		}

		// read packet content
		header := ph.decodeHeader(buf)
		if len(buf) < int(header.size) {
			// renew buf
			buf = make([]byte, int(header.size))
		}

		err = ph.readRequiredBytes(buf, int(header.size))
		if err != nil {
			log.Println("serveTCP read packet body failed:", err)
			break
		}

		// send to websocket
		msg32 := int(header.msg)
		data := buf[0:header.size]
		hash := calcHash(data)
		if header.hash != hash {
			log.Printf("serveTCP hash not match, header hash:%d, calc:%d\n", header, hash)
			break
		}

		if (header.flag & flagCompressed) != 0 {
			// compressed packet, need uncompressed first
			log.Println("serveTCP got compressed packet, decompress ...")
			data, err = zlibDecompress(data)

			if err != nil {
				log.Println("serveTCP decompress failed:", err)
				break
			}
		}

		// msg32 left shift 8 bit
		err = ph.sendProxyMessage(data, msg32<<8)

		if err != nil {
			log.Println("serveTCP send ws packet failed:", err)
			break
		}
	}
}

func (ph *pairHolder) decodeHeader(buf []byte) *packetHeader {
	pheader := &packetHeader{}

	msgU16 := binary.LittleEndian.Uint16(buf)
	flag := buf[2]
	compress := buf[3]
	sizeU32 := binary.LittleEndian.Uint32(buf[4:])
	hashU32 := binary.LittleEndian.Uint32(buf[8:])
	pheader.msg = msgU16
	pheader.flag = flag
	pheader.compress = compress
	pheader.size = sizeU32
	pheader.hash = hashU32
	log.Printf("decodeHeader, header:%v", pheader)

	return pheader
}

func sendTCPMessage(gmsg *ProxyMessage, tcpConn *net.TCPConn) {
	data, err := wsMessage2TcpMessage(gmsg)
	if err != nil {
		log.Println("pair holder onWebsocketMessage wsMessage2TcpMessage failed:", err)
		return
	}

	tcpConn.SetWriteDeadline(time.Now().Add(tcpWriteDeadLine))
	log.Printf("onWebsocketMessage, write %d to tcp", len(data))
	wrote, err := tcpConn.Write(data)

	if err != nil {
		log.Println("pair holder onWebsocketMessage write tcp failed:", err)
		return
	}

	if wrote < len(data) {
		log.Printf("pair holder onWebsocketMessage write tcp, wrote:%d != expected:%d", wrote, len(data))
	}

	log.Println("sendTCPMessage, length:", wrote)
}

func zlibDecompress(data []byte) ([]byte, error) {
	b := bytes.NewReader(data)
	r, err := gzip.NewReader(b)
	if err != nil {
		return nil, err
	}

	return ioutil.ReadAll(r)
}

func wsMessage2TcpMessage(gmsg *ProxyMessage) ([]byte, error) {
	wsData := gmsg.GetData()
	wsDataLength := len(wsData)
	data := make([]byte, packHeaderSize+wsDataLength)
	if wsDataLength > 0 {
		copy(data[12:], gmsg.GetData())
	}

	log.Println("wsMessage2TcpMessage, ops:", gmsg.GetOps()>>8)
	binary.LittleEndian.PutUint16(data, uint16(gmsg.GetOps()>>8)) // msg code, right shift 8 bits
	data[2] = 0                                                   // flag none
	data[3] = 0                                                   // uncompressed

	binary.LittleEndian.PutUint32(data[4:], uint32(wsDataLength)) // size

	hash := calcHash(wsData)
	binary.LittleEndian.PutUint32(data[8:], uint32(hash)) // hash

	return data, nil
}

func calcHash(data []byte) uint32 {
	// 以下代码是copy自南京项目组的pb.cpp文件中的calchash函数
	var hash uint32
	for _, b := range data {
		hash = (hash << 4) + uint32(b)
		tmp := (hash & 0xf0000000)
		if tmp != 0 {
			hash = hash ^ (tmp >> 24)
			hash = hash ^ tmp
		}
	}

	return hash
}
