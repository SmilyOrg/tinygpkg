package binary

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

var ErrInvalidMagic = errors.New("invalid magic")

var ExtensionTWKB = []byte{'T', 'W', 'K', 'B'}

// See http://www.geopackage.org/spec/#gpb_data_blob_format
type Header struct {
	HeaderTop
	HeaderSrs
	ExtensionCode []byte
}

func Read(r io.Reader) (*Header, error) {
	b := &Header{}
	err := binary.Read(r, binary.LittleEndian, &b.HeaderTop)
	if err != nil {
		return nil, err
	}
	if b.Magic != [2]byte{0x47, 0x50} {
		return nil, ErrInvalidMagic
	}
	bo := b.ByteOrder()
	err = binary.Read(r, bo, &b.HeaderSrs)
	if err != nil {
		return nil, err
	}
	envsize := int64(b.EnvelopeContentsIndicatorCode().Size())
	n, err := io.CopyN(io.Discard, r, envsize)
	if err != nil {
		return nil, err
	}
	if n != envsize {
		return nil, io.EOF
	}
	if b.Type() == ExtendedType {
		b.ExtensionCode = make([]byte, 4)
		n, err := r.Read(b.ExtensionCode)
		if err != nil {
			return nil, err
		}
		if n != 4 {
			return nil, io.EOF
		}
	}
	return b, nil
}

func (b *Header) Write(w io.Writer) error {
	if b.EnvelopeContentsIndicatorCode() != NoEnvelope {
		return fmt.Errorf("unsupported envelope %s", b.EnvelopeContentsIndicatorCode().String())
	}
	if b.Type() == ExtendedType && len(b.ExtensionCode) != 4 {
		return fmt.Errorf("invalid extension code length %d", len(b.ExtensionCode))
	}
	err := binary.Write(w, binary.LittleEndian, &b.HeaderTop)
	if err != nil {
		return err
	}
	err = binary.Write(w, b.ByteOrder(), &b.HeaderSrs)
	if err != nil {
		return err
	}
	if b.Type() == ExtendedType {
		n, err := w.Write(b.ExtensionCode)
		if n != 4 {
			return io.EOF
		}
		if err != nil {
			return err
		}
	}
	return nil
}

type HeaderTop struct {
	Magic   [2]byte
	Version uint8
	Flags   uint8
}

type HeaderSrs struct {
	SrsId int32
}

type Type uint8

const (
	UnknownType Type = iota
	StandardType
	ExtendedType
)

func (b *Header) String() string {
	return fmt.Sprintf("Header{HeaderTop: %s, HeaderSrs: %s, ExtensionCode: %v}", b.HeaderTop.String(), b.HeaderSrs.String(), b.ExtensionCode)
}

func (h *HeaderTop) String() string {
	return fmt.Sprintf("HeaderTop{Magic: %v, Version: %d, Flags: %08b}", h.Magic, h.Version, h.Flags)
}

func (h *HeaderSrs) String() string {
	return fmt.Sprintf("HeaderSrs{SrsId: %d}", h.SrsId)
}

func (t Type) String() string {
	switch t {
	case StandardType:
		return "Standard"
	case ExtendedType:
		return "Extended"
	default:
		return fmt.Sprintf("Unknown(%d)", t)
	}
}

func (h *HeaderTop) Type() Type {
	if h.Flags&0b0010_0000 == 0 {
		return StandardType
	}
	return ExtendedType
}

func (h *HeaderTop) SetType(t Type) {
	switch t {
	case StandardType:
		h.Flags &^= 0b0010_0000
	case ExtendedType:
		h.Flags |= 0b0010_0000
	}
}

func (h *HeaderTop) Empty() bool {
	return (h.Flags&0b0001_0000)>>4 == 1
}

type EnvelopeContentsIndicatorCode uint8

const (
	NoEnvelope EnvelopeContentsIndicatorCode = iota
	XY
	XYZ
	XYM
	XYZM
)

func (c EnvelopeContentsIndicatorCode) String() string {
	switch c {
	case NoEnvelope:
		return "no envelope, 0 bytes"
	case XY:
		return "envelope is [minx, maxx, miny, maxy], 32 bytes"
	case XYZ:
		return "envelope is [minx, maxx, miny, maxy, minz, maxz], 48 bytes"
	case XYM:
		return "envelope is [minx, maxx, miny, maxy, minm, maxm], 48 bytes"
	case XYZM:
		return "envelope is [minx, maxx, miny, maxy, minz, maxz, minm, maxm], 64 bytes"
	default:
		return fmt.Sprintf("invalid envelope contents indicator code: %d", c)
	}
}

func (c EnvelopeContentsIndicatorCode) Size() int {
	switch c {
	case NoEnvelope:
		return 0
	case XY:
		return 32
	case XYZ:
		return 48
	case XYM:
		return 48
	case XYZM:
		return 64
	default:
		return 0
	}
}

func (h *HeaderTop) EnvelopeContentsIndicatorCode() EnvelopeContentsIndicatorCode {
	return EnvelopeContentsIndicatorCode((h.Flags & 0b0000_1110) >> 1)
}

func (h *HeaderTop) SetEnvelopeContentsIndicatorCode(c EnvelopeContentsIndicatorCode) {
	h.Flags &^= 0b0000_1110
	h.Flags |= uint8(c) << 1
}

func (h *HeaderTop) ByteOrder() binary.ByteOrder {
	switch h.Flags & 0b0000_0001 {
	case 0:
		return binary.BigEndian
	case 1:
		return binary.LittleEndian
	default:
		return nil
	}
}
