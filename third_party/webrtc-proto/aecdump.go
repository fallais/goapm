// Package webrtcproto provides a minimal, hand-rolled reader for
// WebRTC's .aecdump format described in debug.proto.
//
// Why hand-rolled instead of generated code? The schema is small,
// versioned slowly, and we only care about a handful of fields; using
// google.golang.org/protobuf/encoding/protowire keeps the project free of
// a protoc toolchain dependency. If we later need write support or full
// schema fidelity, run `go install
// google.golang.org/protobuf/cmd/protoc-gen-go` and regenerate per the
// instructions in ../tools/tools.go.
package webrtcproto

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"

	"google.golang.org/protobuf/encoding/protowire"
)

// EventType mirrors the Event.Type enum in debug.proto.
type EventType int32

const (
	EventInit          EventType = 0
	EventReverseStream EventType = 1
	EventStream        EventType = 2
	EventConfig        EventType = 3
	EventUnknown       EventType = 4
)

func (t EventType) String() string {
	switch t {
	case EventInit:
		return "INIT"
	case EventReverseStream:
		return "REVERSE_STREAM"
	case EventStream:
		return "STREAM"
	case EventConfig:
		return "CONFIG"
	default:
		return fmt.Sprintf("EVENT(%d)", int32(t))
	}
}

// Init carries the fields from the Init message that we need.
type Init struct {
	SampleRate               int32
	NumInputChannels         int32
	NumOutputChannels        int32
	NumReverseChannels       int32
	ReverseSampleRate        int32
	OutputSampleRate         int32
	NumReverseOutputChannels int32
	TimestampMS              int64
}

// ReverseStream is one far-end frame, deinterleaved.
type ReverseStream struct {
	// Channels is the per-channel float32 audio. Empty when the dump uses
	// the legacy interleaved int16 path; LegacyData is filled instead.
	Channels   [][]float32
	LegacyData []byte // raw int16, native endian — for very old dumps
}

// Stream is one near-end frame, deinterleaved.
type Stream struct {
	InputChannels      [][]float32
	OutputChannels     [][]float32
	LegacyInputData    []byte
	LegacyOutputData   []byte
	DelayMS            int32
	AppliedInputVolume float32
}

// ConfigEvent captures the runtime config snapshot the dump records.
// Only the fields we care about are surfaced; unknown fields are silently
// dropped to keep us forward-compatible with new tag numbers.
type ConfigEvent struct {
	AECEnabled bool
	AECMEnabled bool
	AGCEnabled bool
	HPFEnabled bool
	NSEnabled  bool
	NSLevel    int32
}

// Event is the discriminated union from debug.proto. Exactly one of the
// pointer fields is non-nil after Decode, matching Type.
type Event struct {
	Type          EventType
	Init          *Init
	ReverseStream *ReverseStream
	Stream        *Stream
	Config        *ConfigEvent
}

// Reader streams Events out of an .aecdump file. The format is a stream
// of length-prefixed serialized Event messages: 4-byte little-endian
// length, then that many bytes of protobuf payload.
type Reader struct {
	r   io.Reader
	buf []byte
}

// NewReader wraps any io.Reader as an .aecdump reader. Caller is
// responsible for closing the underlying source.
func NewReader(r io.Reader) *Reader { return &Reader{r: r} }

// Next reads one event. Returns io.EOF at end of stream.
func (rd *Reader) Next() (*Event, error) {
	var hdr [4]byte
	if _, err := io.ReadFull(rd.r, hdr[:]); err != nil {
		if errors.Is(err, io.EOF) {
			return nil, io.EOF
		}
		return nil, err
	}
	size := binary.LittleEndian.Uint32(hdr[:])
	if size == 0 {
		return nil, errors.New("aecdump: zero-length event")
	}
	if cap(rd.buf) < int(size) {
		rd.buf = make([]byte, size)
	}
	rd.buf = rd.buf[:size]
	if _, err := io.ReadFull(rd.r, rd.buf); err != nil {
		return nil, fmt.Errorf("aecdump: short event payload: %w", err)
	}
	return decodeEvent(rd.buf)
}

func decodeEvent(b []byte) (*Event, error) {
	ev := &Event{Type: EventUnknown}
	for len(b) > 0 {
		num, typ, n := protowire.ConsumeTag(b)
		if n < 0 {
			return nil, fmt.Errorf("aecdump: bad tag: %v", protowire.ParseError(n))
		}
		b = b[n:]
		switch num {
		case 1: // type
			v, n := protowire.ConsumeVarint(b)
			if n < 0 {
				return nil, fmt.Errorf("aecdump: bad type varint")
			}
			ev.Type = EventType(int32(v))
			b = b[n:]
		case 2:
			payload, n := protowire.ConsumeBytes(b)
			if n < 0 {
				return nil, errors.New("aecdump: bad Init bytes")
			}
			b = b[n:]
			init, err := decodeInit(payload)
			if err != nil {
				return nil, err
			}
			ev.Init = init
		case 3:
			payload, n := protowire.ConsumeBytes(b)
			if n < 0 {
				return nil, errors.New("aecdump: bad ReverseStream bytes")
			}
			b = b[n:]
			rs, err := decodeReverseStream(payload)
			if err != nil {
				return nil, err
			}
			ev.ReverseStream = rs
		case 4:
			payload, n := protowire.ConsumeBytes(b)
			if n < 0 {
				return nil, errors.New("aecdump: bad Stream bytes")
			}
			b = b[n:]
			s, err := decodeStream(payload)
			if err != nil {
				return nil, err
			}
			ev.Stream = s
		case 5:
			payload, n := protowire.ConsumeBytes(b)
			if n < 0 {
				return nil, errors.New("aecdump: bad Config bytes")
			}
			b = b[n:]
			c, err := decodeConfig(payload)
			if err != nil {
				return nil, err
			}
			ev.Config = c
		default:
			n := protowire.ConsumeFieldValue(num, typ, b)
			if n < 0 {
				return nil, fmt.Errorf("aecdump: malformed field %d: %v", num, protowire.ParseError(n))
			}
			b = b[n:]
		}
	}
	return ev, nil
}

func decodeInit(b []byte) (*Init, error) {
	out := &Init{}
	for len(b) > 0 {
		num, typ, n := protowire.ConsumeTag(b)
		if n < 0 {
			return nil, fmt.Errorf("init tag: %v", protowire.ParseError(n))
		}
		b = b[n:]
		switch num {
		case 1:
			v, n := protowire.ConsumeVarint(b)
			out.SampleRate = int32(v)
			b = b[n:]
		case 3:
			v, n := protowire.ConsumeVarint(b)
			out.NumInputChannels = int32(v)
			b = b[n:]
		case 4:
			v, n := protowire.ConsumeVarint(b)
			out.NumOutputChannels = int32(v)
			b = b[n:]
		case 5:
			v, n := protowire.ConsumeVarint(b)
			out.NumReverseChannels = int32(v)
			b = b[n:]
		case 6:
			v, n := protowire.ConsumeVarint(b)
			out.ReverseSampleRate = int32(v)
			b = b[n:]
		case 7:
			v, n := protowire.ConsumeVarint(b)
			out.OutputSampleRate = int32(v)
			b = b[n:]
		case 8:
			v, n := protowire.ConsumeVarint(b)
			out.NumReverseOutputChannels = int32(v)
			b = b[n:]
		case 9:
			v, n := protowire.ConsumeVarint(b)
			out.TimestampMS = int64(v)
			b = b[n:]
		default:
			n := protowire.ConsumeFieldValue(num, typ, b)
			if n < 0 {
				return nil, fmt.Errorf("init field %d: %v", num, protowire.ParseError(n))
			}
			b = b[n:]
		}
	}
	return out, nil
}

func decodeReverseStream(b []byte) (*ReverseStream, error) {
	out := &ReverseStream{}
	for len(b) > 0 {
		num, typ, n := protowire.ConsumeTag(b)
		if n < 0 {
			return nil, fmt.Errorf("rev tag: %v", protowire.ParseError(n))
		}
		b = b[n:]
		switch num {
		case 1:
			payload, n := protowire.ConsumeBytes(b)
			out.LegacyData = append([]byte(nil), payload...)
			b = b[n:]
		case 2:
			payload, n := protowire.ConsumeBytes(b)
			b = b[n:]
			out.Channels = append(out.Channels, bytesToFloat32(payload))
		default:
			n := protowire.ConsumeFieldValue(num, typ, b)
			b = b[n:]
		}
	}
	return out, nil
}

func decodeStream(b []byte) (*Stream, error) {
	out := &Stream{}
	for len(b) > 0 {
		num, typ, n := protowire.ConsumeTag(b)
		if n < 0 {
			return nil, fmt.Errorf("stream tag: %v", protowire.ParseError(n))
		}
		b = b[n:]
		switch num {
		case 1:
			payload, n := protowire.ConsumeBytes(b)
			out.LegacyInputData = append([]byte(nil), payload...)
			b = b[n:]
		case 2:
			payload, n := protowire.ConsumeBytes(b)
			out.LegacyOutputData = append([]byte(nil), payload...)
			b = b[n:]
		case 3:
			v, n := protowire.ConsumeVarint(b)
			out.DelayMS = int32(v)
			b = b[n:]
		case 7:
			payload, n := protowire.ConsumeBytes(b)
			b = b[n:]
			out.InputChannels = append(out.InputChannels, bytesToFloat32(payload))
		case 8:
			payload, n := protowire.ConsumeBytes(b)
			b = b[n:]
			out.OutputChannels = append(out.OutputChannels, bytesToFloat32(payload))
		case 9:
			v, n := protowire.ConsumeFixed32(b)
			out.AppliedInputVolume = math.Float32frombits(v)
			b = b[n:]
		default:
			n := protowire.ConsumeFieldValue(num, typ, b)
			b = b[n:]
		}
	}
	return out, nil
}

func decodeConfig(b []byte) (*ConfigEvent, error) {
	out := &ConfigEvent{}
	for len(b) > 0 {
		num, typ, n := protowire.ConsumeTag(b)
		if n < 0 {
			return nil, fmt.Errorf("config tag: %v", protowire.ParseError(n))
		}
		b = b[n:]
		switch num {
		case 1:
			v, n := protowire.ConsumeVarint(b)
			out.AECEnabled = v != 0
			b = b[n:]
		case 6:
			v, n := protowire.ConsumeVarint(b)
			out.AECMEnabled = v != 0
			b = b[n:]
		case 9:
			v, n := protowire.ConsumeVarint(b)
			out.AGCEnabled = v != 0
			b = b[n:]
		case 12:
			v, n := protowire.ConsumeVarint(b)
			out.HPFEnabled = v != 0
			b = b[n:]
		case 13:
			v, n := protowire.ConsumeVarint(b)
			out.NSEnabled = v != 0
			b = b[n:]
		case 14:
			v, n := protowire.ConsumeVarint(b)
			out.NSLevel = int32(v)
			b = b[n:]
		default:
			n := protowire.ConsumeFieldValue(num, typ, b)
			b = b[n:]
		}
	}
	return out, nil
}

// bytesToFloat32 decodes a little-endian float32 byte array.
func bytesToFloat32(b []byte) []float32 {
	n := len(b) / 4
	out := make([]float32, n)
	for i := 0; i < n; i++ {
		out[i] = math.Float32frombits(binary.LittleEndian.Uint32(b[i*4:]))
	}
	return out
}

// --- Writer side (used by tests to synthesize tiny .aecdump fixtures) ---

// WriteEvent serializes an Event to w, length-prefixed. Only the fields
// our reader honors are written; everything else is dropped.
func WriteEvent(w io.Writer, ev *Event) error {
	payload, err := encodeEvent(ev)
	if err != nil {
		return err
	}
	var hdr [4]byte
	binary.LittleEndian.PutUint32(hdr[:], uint32(len(payload)))
	if _, err := w.Write(hdr[:]); err != nil {
		return err
	}
	_, err = w.Write(payload)
	return err
}

func encodeEvent(ev *Event) ([]byte, error) {
	var b []byte
	b = protowire.AppendTag(b, 1, protowire.VarintType)
	b = protowire.AppendVarint(b, uint64(ev.Type))
	if ev.Init != nil {
		body := encodeInit(ev.Init)
		b = protowire.AppendTag(b, 2, protowire.BytesType)
		b = protowire.AppendVarint(b, uint64(len(body)))
		b = append(b, body...)
	}
	if ev.ReverseStream != nil {
		body := encodeReverseStream(ev.ReverseStream)
		b = protowire.AppendTag(b, 3, protowire.BytesType)
		b = protowire.AppendVarint(b, uint64(len(body)))
		b = append(b, body...)
	}
	if ev.Stream != nil {
		body := encodeStream(ev.Stream)
		b = protowire.AppendTag(b, 4, protowire.BytesType)
		b = protowire.AppendVarint(b, uint64(len(body)))
		b = append(b, body...)
	}
	if ev.Config != nil {
		body := encodeConfig(ev.Config)
		b = protowire.AppendTag(b, 5, protowire.BytesType)
		b = protowire.AppendVarint(b, uint64(len(body)))
		b = append(b, body...)
	}
	return b, nil
}

func encodeInit(in *Init) []byte {
	var b []byte
	if in.SampleRate != 0 {
		b = protowire.AppendTag(b, 1, protowire.VarintType)
		b = protowire.AppendVarint(b, uint64(in.SampleRate))
	}
	if in.NumInputChannels != 0 {
		b = protowire.AppendTag(b, 3, protowire.VarintType)
		b = protowire.AppendVarint(b, uint64(in.NumInputChannels))
	}
	if in.NumOutputChannels != 0 {
		b = protowire.AppendTag(b, 4, protowire.VarintType)
		b = protowire.AppendVarint(b, uint64(in.NumOutputChannels))
	}
	if in.NumReverseChannels != 0 {
		b = protowire.AppendTag(b, 5, protowire.VarintType)
		b = protowire.AppendVarint(b, uint64(in.NumReverseChannels))
	}
	if in.ReverseSampleRate != 0 {
		b = protowire.AppendTag(b, 6, protowire.VarintType)
		b = protowire.AppendVarint(b, uint64(in.ReverseSampleRate))
	}
	if in.OutputSampleRate != 0 {
		b = protowire.AppendTag(b, 7, protowire.VarintType)
		b = protowire.AppendVarint(b, uint64(in.OutputSampleRate))
	}
	if in.NumReverseOutputChannels != 0 {
		b = protowire.AppendTag(b, 8, protowire.VarintType)
		b = protowire.AppendVarint(b, uint64(in.NumReverseOutputChannels))
	}
	if in.TimestampMS != 0 {
		b = protowire.AppendTag(b, 9, protowire.VarintType)
		b = protowire.AppendVarint(b, uint64(in.TimestampMS))
	}
	return b
}

func encodeReverseStream(rs *ReverseStream) []byte {
	var b []byte
	for _, ch := range rs.Channels {
		raw := float32sToBytes(ch)
		b = protowire.AppendTag(b, 2, protowire.BytesType)
		b = protowire.AppendVarint(b, uint64(len(raw)))
		b = append(b, raw...)
	}
	return b
}

func encodeStream(s *Stream) []byte {
	var b []byte
	if s.DelayMS != 0 {
		b = protowire.AppendTag(b, 3, protowire.VarintType)
		b = protowire.AppendVarint(b, uint64(s.DelayMS))
	}
	for _, ch := range s.InputChannels {
		raw := float32sToBytes(ch)
		b = protowire.AppendTag(b, 7, protowire.BytesType)
		b = protowire.AppendVarint(b, uint64(len(raw)))
		b = append(b, raw...)
	}
	for _, ch := range s.OutputChannels {
		raw := float32sToBytes(ch)
		b = protowire.AppendTag(b, 8, protowire.BytesType)
		b = protowire.AppendVarint(b, uint64(len(raw)))
		b = append(b, raw...)
	}
	if s.AppliedInputVolume != 0 {
		b = protowire.AppendTag(b, 9, protowire.Fixed32Type)
		b = protowire.AppendFixed32(b, math.Float32bits(s.AppliedInputVolume))
	}
	return b
}

func encodeConfig(c *ConfigEvent) []byte {
	var b []byte
	put := func(tag int, v bool) {
		if v {
			b = protowire.AppendTag(b, protowire.Number(tag), protowire.VarintType)
			b = protowire.AppendVarint(b, 1)
		}
	}
	put(1, c.AECEnabled)
	put(6, c.AECMEnabled)
	put(9, c.AGCEnabled)
	put(12, c.HPFEnabled)
	put(13, c.NSEnabled)
	if c.NSLevel != 0 {
		b = protowire.AppendTag(b, 14, protowire.VarintType)
		b = protowire.AppendVarint(b, uint64(c.NSLevel))
	}
	return b
}

func float32sToBytes(x []float32) []byte {
	out := make([]byte, len(x)*4)
	for i, v := range x {
		binary.LittleEndian.PutUint32(out[i*4:], math.Float32bits(v))
	}
	return out
}
