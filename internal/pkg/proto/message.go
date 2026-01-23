package proto

import (
	"encoding/binary"
	"encoding/json"
	"io"
)

/*
Message 结构体、编解码方法
+------+--------+---------+
| Type | Length |  Data   |
| 1字节 | 4字节  |  N字节   |
+------+--------+---------+
*/

// Message
type Message struct {
	Type uint8
	Data []byte
}

// 对 Message 按照已经定义好的协议进行编码
func (m *Message) WriteTo(w io.Writer) (n int64, err error) {
	// 检查消息长度
	dataLen := len(m.Data)
	if dataLen > MaxDataLen {
		return 0, ErrMsgTooLarge
	}

	// 构造消息头
	header := make([]byte, HeaderLen)
	// 第1字节 数据类型，2-5字节 数据长度（大端序）
	header[0] = m.Type
	binary.BigEndian.PutUint32(header[1:5], uint32(dataLen))

	// 写入消息头
	written, err := w.Write(header)
	n = int64(written)
	if err != nil {
		return n, err
	}

	// 写入消息体
	if dataLen <= 0 {
		return n, nil
	}
	written, err = w.Write(m.Data)
	n += int64(written)
	if err != nil {
		return n, err
	}

	return n, nil
}

// 对 Message 按照已经定义好的协议进行解码
func (m *Message) ReadFrom(r io.Reader) (n int64, err error) {
	// 读取消息头
	header := make([]byte, HeaderLen)
	readN, err := io.ReadFull(r, header)
	n = int64(readN)
	if err != nil {
		return n, err
	}

	// 解析消息头
	m.Type = header[0]
	dataLen := binary.BigEndian.Uint32(header[1:5])
	if dataLen > MaxDataLen {
		return n, ErrMsgTooLarge
	}

	// 读取消息体
	if dataLen > 0 {
		m.Data = make([]byte, dataLen)
		readN, err = io.ReadFull(r, m.Data)
		n += int64(readN)
		if err != nil {
			return n, err
		}
	} else {
		m.Data = nil
	}

	return n, nil
}

// 将消息体反序列化到制定结构体
// v 必须指针类型
func (m *Message) Unmarshal(v interface{}) error {
	if len(m.Data) == 0 {
		return nil
	}
	return json.Unmarshal(m.Data, v)
}

// 创建一条 Message
func NewMessage(msgType uint8, payload interface{}) (*Message, error) {
	msg := &Message{Type: msgType}
	if payload == nil {
		return msg, nil
	}

	// 将 payload 序列为 json
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	msg.Data = data

	return msg, nil
}
