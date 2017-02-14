package protocol

import "errors"

// when read RTMP rtmpChunk error.
var RtmpChunkError = errors.New("rtmp rtmpChunk error")

// when got RTMP msg invalid payload.
var RtmpPayloadError = errors.New("rtmp msg payload error")

// the amf0 object error.
var Amf0Error = errors.New("amf0 error")

// the rtmp request url error.
var RequestUrlError = errors.New("rtmp request url error")
