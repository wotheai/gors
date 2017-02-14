package protocol

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"neo/gors/core"
	"net"
	"sync"
)

type rtmpConnection struct {
	stack     *rtmpStack
	req       *rtmpRequest
	in        chan *message
	out       []message
	wg        *sync.WaitGroup
	handshake *hsBytes
}

func NewRtmpconnection(c net.Conn) *rtmpConnection {
	v := &rtmpConnection{
		stack:     newRtmpStack(c),
		req:       newRtmpRequest(),
		in:        make(chan *message, 1024),
		out:       make([]message, 1024),
		wg:        &sync.WaitGroup{},
		handshake: newHsBytes(),
	}

	wait := make(chan bool)

	go core.Recover(func() error {

		wait <- true

		if err := v.receiver(); err != nil {
			return err
		}
		return nil

	})

	return v
}

func (v *rtmpConnection) SetRtmpConnectionWaitGroup() {
	v.wg.Add(1)
}

func (v *rtmpConnection) WaitRtmpConnection() {
	v.wg.Wait()
}

func HandlerRtmpConnection(v *rtmpConnection) {
	defer v.wg.Done()
}

func (v *rtmpConnection) receiver() error {
	err := v.handshake.readC0C1(v.stack)
	if err != nil {
		return err
	}

	if err := v.handshake.readC2(v.stack); err != nil {
		return err
	}

	for {
		var m *message

		m, err := v.stack.readMessage()
		if err != nil {
			return err
		}

		select {
		case v.in <- m:
		default:

		}

	}

}

func (v *hsBytes) readC0C1(s *rtmpStack) error {
	fmt.Println("begin cache handshake c0c1 bytes.")
	var buff bytes.Buffer

	r, ok := s.in.(io.Reader)
	if !ok {
		//fmt.Println("rtmpStack in chan error.")
		return errors.New("rtmpStack c0c1 chan error.")
	}

	if _, err := io.CopyN(&buff, r, 1537); err != nil {
		fmt.Println("cache c0c1 error.")
		return err
	}

	copy(v.c0c1c2[:1537], buff.Bytes())
	return nil
}

func (v *hsBytes) readC2(s *rtmpStack) error {
	fmt.Println("begin read handshake c2 bytes.")

	var buff bytes.Buffer
	r, ok := s.in.(io.Reader)
	if !ok {
		return errors.New("rtmpStack c2 chan error.")
	}

	if _, err := io.CopyN(&buff, r, 1536); err != nil {
		return err
	}

	copy(v.c0c1c2[1537:], buff.Bytes())
	return nil
}

func (v *rtmpStack) readMessage() (*message, error) {
	var m *message

	chunks := make(map[uint32]*rtmpChunk)

	for m == nil {
		fmt, csid, err := readChunkBasicHeader(v.in)
		if err != nil {
			return nil, err
		}

		var chunk *rtmpChunk

		if c, ok := chunks[csid]; !ok {
			chunk = newChunk(csid)
			chunks[csid] = chunk
		} else {
			chunks[csid] = c
		}

		// rtmpChunk stream message header
		if err = readMessageHeader(v.in, fmt, chunk); err != nil {
			return
		}

	}
}

func readChunkBasicHeader(c io.ReadCloser) (fmt uint8, csid uint32, err error) {
	var vb byte

	in := bufio.NewReader(c)

	if vb, err = in.ReadByte(); err != nil {
		return
	}

	fmt = uint8(vb)
	csid = uint32(fmt & 0x3f)
	fmt = (fmt >> 6) & 0x03

	// 2-63, 1B rtmpChunk header
	if csid >= 2 {
		return
	}

	// 2 or 3B
	if csid >= 0 {
		// 64-319, 2B rtmpChunk header
		if vb, err = in.ReadByte(); err != nil {
			return
		}

		temp := uint32(vb) + 64

		// 64-65599, 3B rtmpChunk header
		if csid >= 1 {
			if vb, err = in.ReadByte(); err != nil {
				return
			}

			temp += uint32(vb) * 256
		}

		return fmt, temp, nil
	}

	return fmt, csid, RtmpChunkError
}

func rtmpReadMessageHeader(in bufferedReader, fmt uint8, chunk *rtmpChunk) (err error) {
	if core.Conf.Debug.RtmpDumpRecv {
		core.Trace.Println(ctx, "start read message header.")
	}

	// we should not assert anything about fmt, for the first packet.
	// (when first packet, the rtmpChunk->msg is NULL).
	// the fmt maybe 0/1/2/3, the FMLE will send a 0xC4 for some audio packet.
	// the previous packet is:
	//     04                // fmt=0, cid=4
	//     00 00 1a          // timestamp=26
	//     00 00 9d          // payload_length=157
	//     08                // message_type=8(audio)
	//     01 00 00 00       // stream_id=1
	// the current packet maybe:
	//     c4             // fmt=3, cid=4
	// it's ok, for the packet is audio, and timestamp delta is 26.
	// the current packet must be parsed as:
	//     fmt=0, cid=4
	//     timestamp=26+26=52
	//     payload_length=157
	//     message_type=8(audio)
	//     stream_id=1
	// so we must update the timestamp even fmt=3 for first packet.
	//
	// fresh packet used to update the timestamp even fmt=3 for first packet.
	// fresh packet always means the rtmpChunk is the first one of message.
	isFirstMsgOfChunk := bool(chunk.partialMessage == nil)

	// but, we can ensure that when a rtmpChunk stream is fresh,
	// the fmt must be 0, a new stream.
	if chunk.isFresh && fmt != RtmpFmtType0 {
		// for librtmp, if ping, it will send a fresh stream with fmt=1,
		// 0x42             where: fmt=1, cid=2, protocol contorl user-control message
		// 0x00 0x00 0x00   where: timestamp=0
		// 0x00 0x00 0x06   where: payload_length=6
		// 0x04             where: message_type=4(protocol control user-control message)
		// 0x00 0x06            where: event Ping(0x06)
		// 0x00 0x00 0x0d 0x0f  where: event data 4bytes ping timestamp.
		// @see: https://github.com/ossrs/srs/issues/98
		if chunk.cid == RtmpCidProtocolControl && fmt == RtmpFmtType1 {
			core.Warn.Println(ctx, "accept cid=2,fmt=1 to make librtmp happy.")
		} else {
			// must be a RTMP protocol level error.
			core.Error.Println(ctx, "fresh rtmpChunk fmt must be", RtmpFmtType0, "actual is", fmt)
			return RtmpChunkError
		}
	}

	// when exists cache msg, means got an partial message,
	// the fmt must not be type0 which means new message.
	if !isFirstMsgOfChunk && fmt == RtmpFmtType0 {
		core.Error.Println(ctx, "rtmpChunk partial msg, fmt must be", RtmpFmtType0, "actual is", fmt)
		return RtmpChunkError
	}

	// create msg when new rtmpChunk stream start
	if chunk.partialMessage == nil {
		chunk.partialMessage = NewRtmpMessage()
	}

	// read message header from socket to buffer.
	nbhs := [4]int{11, 7, 3, 0}
	nbh := nbhs[fmt]

	var bh []byte
	if nbh > 0 {
		if bh, err = in.Peek(nbh); err != nil {
			return
		}
		if _, err = io.CopyN(ioutil.Discard, in, int64(nbh)); err != nil {
			return
		}
	}

	// parse the message header.
	//   3bytes: timestamp delta,    fmt=0,1,2
	//   3bytes: payload length,     fmt=0,1
	//   1bytes: message type,       fmt=0,1
	//   4bytes: stream id,          fmt=0
	// where:
	//   fmt=0, 0x0X
	//   fmt=1, 0x4X
	//   fmt=2, 0x8X
	//   fmt=3, 0xCX
	// see also: ngx_rtmp_recv
	if fmt <= RtmpFmtType2 {
		delta := uint32(bh[2]) | uint32(bh[1])<<8 | uint32(bh[0])<<16

		// for a message, if msg exists in cache, the delta must not changed.
		if !isFirstMsgOfChunk && chunk.timestampDelta != delta {
			core.Error.Println(ctx, "rtmpChunk msg exists, should not change the delta.")
			return RtmpChunkError
		}

		// fmt: 0
		// timestamp: 3 bytes
		// If the timestamp is greater than or equal to 16777215
		// (hexadecimal 0x00ffffff), this value MUST be 16777215, and the
		// 'extended timestamp header' MUST be present. Otherwise, this value
		// SHOULD be the entire timestamp.
		//
		// fmt: 1 or 2
		// timestamp delta: 3 bytes
		// If the delta is greater than or equal to 16777215 (hexadecimal
		// 0x00ffffff), this value MUST be 16777215, and the 'extended
		// timestamp header' MUST be present. Otherwise, this value SHOULD be
		// the entire delta.
		if chunk.hasExtendedTimestamp = bool(delta >= RtmpExtendedTimestamp); !chunk.hasExtendedTimestamp {
			// no extended-timestamp, apply the delta.
			chunk.timestampDelta = delta

			// Extended timestamp: 0 or 4 bytes
			// This field MUST be sent when the normal timsestamp is set to
			// 0xffffff, it MUST NOT be sent if the normal timestamp is set to
			// anything else. So for values less than 0xffffff the normal
			// timestamp field SHOULD be used in which case the extended timestamp
			// MUST NOT be present. For values greater than or equal to 0xffffff
			// the normal timestamp field MUST NOT be used and MUST be set to
			// 0xffffff and the extended timestamp MUST be sent.
			if fmt == RtmpFmtType0 {
				// 6.1.2.1. Type 0
				// For a type-0 rtmpChunk, the absolute timestamp of the message is sent
				// here.
				chunk.timestamp = uint64(delta)
			} else {
				// 6.1.2.2. Type 1
				// 6.1.2.3. Type 2
				// For a type-1 or type-2 rtmpChunk, the difference between the previous
				// rtmpChunk's timestamp and the current rtmpChunk's timestamp is sent here.
				// @remark for continuous rtmpChunk, timestamp never change.
				if isFirstMsgOfChunk {
					chunk.timestamp += uint64(delta)
				}
			}
		}

		if fmt <= RtmpFmtType1 {
			payloadLength := uint32(bh[5]) | uint32(bh[4])<<8 | uint32(bh[3])<<16
			mtype := uint8(bh[6])

			// for a message, if msg exists in cache, the size must not changed.
			if !isFirstMsgOfChunk && chunk.payloadLength != payloadLength {
				core.Error.Println(ctx, "rtmpChunk msg exists, payload length should not be changed.")
				return RtmpChunkError
			}
			// for a message, if msg exists in cache, the type must not changed.
			if !isFirstMsgOfChunk && chunk.messageType != mtype {
				core.Error.Println(ctx, "rtmpChunk msg exists, type should not be changed.")
				return RtmpChunkError
			}
			chunk.payloadLength = payloadLength
			chunk.messageType = mtype

			if fmt == RtmpFmtType0 {
				// little-endian
				chunk.streamId = uint32(bh[7]) | uint32(bh[8])<<8 | uint32(bh[9])<<16 | uint32(bh[10])<<24
			}
		}
	} else {
		// update the timestamp even fmt=3 for first rtmpChunk packet
		if isFirstMsgOfChunk && !chunk.hasExtendedTimestamp {
			chunk.timestamp += uint64(chunk.timestampDelta)
		}
	}

	// read extended-timestamp
	if chunk.hasExtendedTimestamp {
		// try to read 4 bytes from stream,
		// which maybe the body or the extended-timestamp.
		var b []byte
		if b, err = in.Peek(4); err != nil {
			return
		}

		// parse the extended-timestamp
		timestamp := uint32(b[3]) | uint32(b[2])<<8 | uint32(b[1])<<16 | uint32(b[0])<<24
		// always use 31bits timestamp, for some server may use 32bits extended timestamp.
		// @see https://github.com/ossrs/srs/issues/111
		timestamp &= 0x7fffffff

		// RTMP specification and ffmpeg/librtmp is false,
		// but, adobe changed the specification, so flash/FMLE/FMS always true.
		// default to true to support flash/FMLE/FMS.
		//
		// ffmpeg/librtmp may donot send this filed, need to detect the value.
		// @see also: http://blog.csdn.net/win_lin/article/details/13363699
		// compare to the rtmpChunk timestamp, which is set by rtmpChunk message header
		// type 0,1 or 2.
		//
		// @remark, nginx send the extended-timestamp in sequence-header,
		// and timestamp delta in continue C1 chunks, and so compatible with ffmpeg,
		// that is, there is no continue chunks and extended-timestamp in nginx-rtmp.
		//
		// @remark, srs always send the extended-timestamp, to keep simple,
		// and compatible with adobe products.
		ctimestamp := uint32(chunk.timestamp) & 0x7fffffff

		// if ctimestamp<=0, the rtmpChunk previous packet has no extended-timestamp,
		// always use the extended timestamp.
		// @remark for the first rtmpChunk of message, always use the extended timestamp.
		if isFirstMsgOfChunk || ctimestamp <= 0 || ctimestamp == timestamp {
			chunk.timestamp = uint64(timestamp)
			if core.Conf.Debug.RtmpDumpRecv {
				core.Trace.Println(ctx, "read matched extended-timestamp.")
			}
			// consume from buffer.
			if _, err = io.CopyN(ioutil.Discard, in, 4); err != nil {
				return
			}
		}
	}

	// the extended-timestamp must be unsigned-int,
	//         24bits timestamp: 0xffffff = 16777215ms = 16777.215s = 4.66h
	//         32bits timestamp: 0xffffffff = 4294967295ms = 4294967.295s = 1193.046h = 49.71d
	// because the rtmp protocol says the 32bits timestamp is about "50 days":
	//         3. Byte Order, Alignment, and Time Format
	//                Because timestamps are generally only 32 bits long, they will roll
	//                over after fewer than 50 days.
	//
	// but, its sample says the timestamp is 31bits:
	//         An application could assume, for example, that all
	//        adjacent timestamps are within 2^31 milliseconds of each other, so
	//        10000 comes after 4000000000, while 3000000000 comes before
	//        4000000000.
	// and flv specification says timestamp is 31bits:
	//        Extension of the Timestamp field to form a SI32 value. This
	//        field represents the upper 8 bits, while the previous
	//        Timestamp field represents the lower 24 bits of the time in
	//        milliseconds.
	// in a word, 31bits timestamp is ok.
	// convert extended timestamp to 31bits.
	chunk.timestamp &= 0x7fffffff

	// copy header to msg
	chunk.partialMessage.MessageType = RtmpMessageType(chunk.messageType)
	chunk.partialMessage.Timestamp = chunk.timestamp
	chunk.partialMessage.PreferCid = chunk.cid
	chunk.partialMessage.StreamId = chunk.streamId

	// update rtmpChunk information.
	chunk.fmt = fmt
	chunk.isFresh = false
	return
}
