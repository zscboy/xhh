package proxy

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"io/ioutil"

	log "github.com/sirupsen/logrus"
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
	read, err := conn.Read(buf)
	if read == 0 {
		// pear shudown, get a FIN package
		log.Println("serveTCP, tcp server finished connection")
		return err
	}

	if err != nil {
		log.Printf("serveTCP read error:%v, addr:%v", err, conn.RemoteAddr())
		return err
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

	buf := make([]byte, 256)

	for {
		// read packet header
		err := ph.readRequiredBytes(buf, 12)
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

		if (header.flag & 1) != 0x40 {
			// compressed packet, need uncompressed first
			log.Println("serveTCP got compressed packet, decompress ...")
			data, err = zlibDecompress(data)

			if err != nil {
				log.Println("serveTCP decompress failed:", err)
				break
			}
		}

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

func zlibDecompress(data []byte) ([]byte, error) {
	b := bytes.NewReader(data)
	r, err := zlib.NewReader(b)
	if err != nil {
		return nil, err
	}

	return ioutil.ReadAll(r)
}
