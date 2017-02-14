package protocol

import (
	"io"
	"net"
	"net/url"
)

/*
type chunkHeader struct {
	//cbh             chunkBasicHeader
	ch                chunkBasicHeader
	cmh               chunkMessageHeader
	extendedTimestamp uint32
}

type chunkBasicHeader struct {
	fmt  uint8
	csid uint32
}

type chunkMessageHeader struct {
	timestamp       uint32
	messageLength   uint32
	messageTypeId   uint8
	messageStreamId uint32
	timestampDelta  uint32
}
*/

type rtmpChunk struct {
	//rtmpChunk basic header
	fmt  uint8
	csid uint32

	//rtmpChunk message header
	timestamp       uint32
	messageLength   uint32
	messageTypeId   uint8
	messageStreamId uint32
	timestampDelta  uint32

	extendedTimestamp uint32
	payload           []byte
}

func newChunk(csid uint32) *rtmpChunk {
	return &rtmpChunk{csid: csid}
}

//message

/*
type messageHeader struct {
	messageType   uint8
	payloadLength uint32
	timestamp     uint32
	streamId      uint32
}

*/

type message struct {
	//message header
	messageType   uint8
	payloadLength uint32
	timestamp     uint32
	streamId      uint32

	payload []byte
}

type rtmpRequest struct {
	app      string
	stream   string
	tcurl    string
	url      *url.URL
	duration float64
	vhost    string
}

type rtmpStack struct {
	in  io.ReadCloser
	out io.WriteCloser
}

func newRtmpStack(c net.Conn) *rtmpStack {
	return &rtmpStack{in: c, out: c}
}

func newRtmpRequest() *rtmpRequest {
	return &rtmpRequest{url: &url.URL{}}
}
