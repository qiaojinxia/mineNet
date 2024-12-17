package network

import (
	"bytes"
	"compress/gzip"
	"crypto/aes"
	"crypto/cipher"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"sync/atomic"
	"time"
)

// 添加版本号常量
const (
	CurrentVersion = uint16(1) // 当前协议版本号
	MaxPacketSize  = 10 * 1024 * 1024
)

var (
	ErrInvalidPacketSize  = errors.New("invalid packet size")
	ErrUnsupportedVersion = errors.New("unsupported protocol version")
)

type CmdId uint32

const (
	CmdTypeReserved uint32 = iota // 保留
	CmdPing                       // 心跳
	CmdPong                       // 心跳响应
	CmdRequest                    // 请求包：客户端->服务端，需要响应
	CmdReply                      // 响应包：服务端->客户端，对应请求
	CmdPush                       // 推送包：服务端->客户端，无需响应
)

type Packet struct {
	version   uint16 // 协议版本号
	seqId     uint64 // 自增消息号
	cmdId     uint32
	data      []byte
	Timestamp int64
}

func NewPacket(cmdId uint32, data []byte) (*Packet, error) {
	return &Packet{
		version:   CurrentVersion,
		seqId:     0,
		cmdId:     cmdId,
		data:      data,
		Timestamp: time.Now().Unix(),
	}, nil
}

func NewHeartPacket(hp *HeartPacket) (*Packet, error) {
	data, err := json.Marshal(hp)
	if err != nil {
		return nil, err
	}
	return &Packet{
		version:   CurrentVersion,
		seqId:     0,
		cmdId:     CmdPing,
		data:      data,
		Timestamp: time.Now().Unix(),
	}, nil
}

// Version 添加版本号相关方法
func (p *Packet) Version() uint16 {
	return p.version
}

func (p *Packet) SetVersion(v uint16) {
	p.version = v
}

// CmdId 保留原有方法
func (p *Packet) CmdId() uint32 {
	return p.cmdId
}

func (p *Packet) SeqId() uint64 {
	return p.seqId
}

func (p *Packet) Data() []byte {
	return p.data
}

func (p *Packet) SetData(i []byte) {
	p.data = i
}

func (p *Packet) SetSeqId(i uint64) {
	p.seqId = i
}

type PacketCodec struct {
	compress bool
	encrypt  bool
	key      []byte
	iv       []byte
	seqId    uint64
}

func NewPacketCodec(compress bool, encrypt bool, key []byte, iv []byte) *PacketCodec {
	return &PacketCodec{compress: compress, encrypt: encrypt, key: key, iv: iv}
}

func (c *PacketCodec) Encode(msg *Packet) ([]byte, error) {
	// 设置自增消息号
	msg.SetSeqId(atomic.AddUint64(&c.seqId, 1))

	data := msg.Data()
	if c.compress {
		var err error
		data, err = c.compressData(data)
		if err != nil {
			return nil, err
		}
	}

	if c.encrypt {
		var err error
		data, err = c.encryptData(data)
		if err != nil {
			return nil, err
		}
	}

	// 包格式：| total length(4) | version(2) | seq_id(8) | cmd_id(4) | timestamp(8) | data |
	totalLen := 26 + len(data) // 增加2字节版本号
	buf := make([]byte, totalLen)

	binary.BigEndian.PutUint32(buf[0:4], uint32(totalLen))
	binary.BigEndian.PutUint16(buf[4:6], msg.Version()) // 写入版本号
	binary.BigEndian.PutUint64(buf[6:14], msg.SeqId())
	binary.BigEndian.PutUint32(buf[14:18], msg.CmdId())
	binary.BigEndian.PutUint64(buf[18:26], uint64(msg.Timestamp))
	copy(buf[26:], data)

	return buf, nil
}

func (c *PacketCodec) Decode(r io.Reader) (*Packet, error) {
	// 读取包头长度(4字节)
	lenBuf := make([]byte, 4)
	if _, err := io.ReadFull(r, lenBuf); err != nil {
		return nil, err
	}
	totalLen := binary.BigEndian.Uint32(lenBuf)

	// 检查包长度是否合法
	if totalLen < 26 || totalLen > MaxPacketSize { // 最小长度改为26字节
		return nil, ErrInvalidPacketSize
	}

	// 读取完整的包数据
	buf := make([]byte, totalLen-4)
	if _, err := io.ReadFull(r, buf); err != nil {
		return nil, err
	}

	// 解析包头
	version := binary.BigEndian.Uint16(buf[0:2])
	// 检查版本号
	if version > CurrentVersion {
		return nil, ErrUnsupportedVersion
	}

	seqId := binary.BigEndian.Uint64(buf[2:10])
	cmdId := binary.BigEndian.Uint32(buf[10:14])
	timestamp := binary.BigEndian.Uint64(buf[14:22])

	// 获取数据部分
	data := buf[22:]

	if c.encrypt {
		var err error
		data, err = c.decryptData(data)
		if err != nil {
			return nil, err
		}
	}

	if c.compress {
		var err error
		data, err = c.decompressData(data)
		if err != nil {
			return nil, err
		}
	}

	packet := &Packet{
		version:   version,
		seqId:     seqId,
		cmdId:     cmdId,
		Timestamp: int64(timestamp),
		data:      data,
	}

	return packet, nil
}

func (c *PacketCodec) compressData(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	w, err := gzip.NewWriterLevel(&buf, gzip.BestSpeed)
	if err != nil {
		return nil, err
	}

	if _, err := w.Write(data); err != nil {
		return nil, err
	}

	if err := w.Close(); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func (c *PacketCodec) encryptData(data []byte) ([]byte, error) {
	block, err := aes.NewCipher(c.key)
	if err != nil {
		return nil, err
	}

	stream := cipher.NewCFBEncrypter(block, c.iv)
	encrypted := make([]byte, len(data))
	stream.XORKeyStream(encrypted, data)

	return encrypted, nil
}

// 解密数据
func (c *PacketCodec) decryptData(data []byte) ([]byte, error) {
	block, err := aes.NewCipher(c.key)
	if err != nil {
		return nil, err
	}

	stream := cipher.NewCFBDecrypter(block, c.iv)
	decrypted := make([]byte, len(data))
	stream.XORKeyStream(decrypted, data)

	return decrypted, nil
}

// 解压数据
func (c *PacketCodec) decompressData(data []byte) ([]byte, error) {
	reader := bytes.NewReader(data)
	gzipReader, err := gzip.NewReader(reader)
	if err != nil {
		return nil, err
	}
	defer func(gzipReader *gzip.Reader) {
		err := gzipReader.Close()
		if err != nil {
			panic(err)
		}
	}(gzipReader)

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, gzipReader); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}
