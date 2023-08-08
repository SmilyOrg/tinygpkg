package gpkg

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"reflect"
	"testing"

	"photofield/geo/gpkg"
)

func TestRead(t *testing.T) {
	tests := []struct {
		name    string
		data    []byte
		want    *gpkg.Header
		wantErr error
	}{
		{
			name: "valid binary",
			data: []byte{
				0x47, 0x50, // magic
				0x00,                   // version
				0x00,                   // flags
				0x00, 0x00, 0x00, 0x01, // srs_id
				0x00, // envelope contents indicator code
			},
			want: &gpkg.Header{
				HeaderTop: gpkg.HeaderTop{
					Magic:   [2]byte{0x47, 0x50},
					Version: 0,
					Flags:   0,
				},
				HeaderSrs: gpkg.HeaderSrs{
					SrsId: 1,
				},
			},
			wantErr: nil,
		},
		{
			name: "eof",
			data: []byte{
				0x00, 0x00,
			},
			want:    nil,
			wantErr: io.ErrUnexpectedEOF,
		},
		{
			name: "invalid magic",
			data: []byte{
				0x12, 0x34, // invalid magic
				0x00,                   // version
				0x00,                   // flags
				0x00, 0x00, 0x00, 0x01, // srs_id
			},
			want:    nil,
			wantErr: gpkg.ErrInvalidMagic,
		},
		{
			name: "unsupported type",
			data: []byte{
				0x47, 0x50, // magic
				0x00,                   // version
				0b0010_0000,            // flags (extended type)
				0x00, 0x00, 0x00, 0x01, // srs_id
			},
			want:    nil,
			wantErr: gpkg.ErrUnsupportedType,
		},
		{
			name: "unsupported version",
			data: []byte{
				0x47, 0x50, // magic
				0x01,                   // unsupported version
				0x00,                   // flags
				0x00, 0x00, 0x00, 0x01, // srs_id
			},
			want:    nil,
			wantErr: gpkg.ErrUnsupportedVersion,
		},
		{
			name: "short buffer",
			data: []byte{
				0x47, 0x50, // magic
				0x00,                   // version
				0b0000_0100,            // flags
				0x00, 0x00, 0x00, 0x01, // srs_id
			},
			want:    nil,
			wantErr: io.EOF,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := bytes.NewReader(tt.data)
			got, err := gpkg.Read(r)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Read() = %v, want %v", got, tt.want)
			}
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("Read() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestBinary_Write(t *testing.T) {
	tests := []struct {
		name    string
		b       *gpkg.Header
		want    []byte
		wantErr bool
	}{
		{
			name: "valid binary",
			b: &gpkg.Header{
				HeaderTop: gpkg.HeaderTop{
					Magic:   [2]byte{0x47, 0x50},
					Version: 0,
					Flags:   0,
				},
				HeaderSrs: gpkg.HeaderSrs{
					SrsId: 15,
				},
			},
			want: []byte{
				0x47, 0x50, // magic
				0x00,                   // version
				0x00,                   // flags
				0x00, 0x00, 0x00, 0x0F, // srs_id
			},
			wantErr: false,
		},
		{
			name: "unsupported envelope",
			b: &gpkg.Header{
				HeaderTop: gpkg.HeaderTop{
					Magic:   [2]byte{0x47, 0x50},
					Version: 0,
					Flags:   0b0000_0010,
				},
				HeaderSrs: gpkg.HeaderSrs{
					SrsId: 1,
				},
			},
			want:    nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			err := tt.b.Write(&buf)
			if (err != nil) != tt.wantErr {
				t.Errorf("Binary.Write() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if !bytes.Equal(buf.Bytes(), tt.want) {
					t.Errorf("Binary.Write() = %v, want %v", buf.Bytes(), tt.want)
				}
			}
		})
	}
}

func TestBinaryReadWrite(t *testing.T) {
	binary := &gpkg.Header{
		HeaderTop: gpkg.HeaderTop{
			Magic:   [2]byte{0x47, 0x50},
			Version: 0,
			Flags:   0,
		},
		HeaderSrs: gpkg.HeaderSrs{
			SrsId: 4326,
		},
	}

	var buf bytes.Buffer
	err := binary.Write(&buf)
	if err != nil {
		t.Fatalf("unexpected error writing binary: %v", err)
	}

	readBinary, err := gpkg.Read(&buf)
	if err != nil {
		t.Fatalf("unexpected error reading binary: %v", err)
	}

	if !reflect.DeepEqual(binary, readBinary) {
		t.Fatalf("expected binary to be %v, but got %v", binary, readBinary)
	}
}

func TestHeaderTop_Type(t *testing.T) {
	tests := []struct {
		name string
		h    gpkg.HeaderTop
		want gpkg.Type
	}{
		{
			name: "standard type",
			h: gpkg.HeaderTop{
				Flags: 0b0000_0000,
			},
			want: gpkg.StandardType,
		},
		{
			name: "extended type",
			h: gpkg.HeaderTop{
				Flags: 0b0010_0000,
			},
			want: gpkg.ExtendedType,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.h.Type(); got != tt.want {
				t.Errorf("HeaderTop.Type() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestHeaderTop_SetType(t *testing.T) {
	tests := []struct {
		name string
		h    gpkg.HeaderTop
		t    gpkg.Type
		want gpkg.HeaderTop
	}{
		{
			name: "set standard type",
			h: gpkg.HeaderTop{
				Flags: 0b0010_0000,
			},
			t: gpkg.StandardType,
			want: gpkg.HeaderTop{
				Flags: 0b0000_0000,
			},
		},
		{
			name: "set extended type",
			h: gpkg.HeaderTop{
				Flags: 0b0000_0000,
			},
			t: gpkg.ExtendedType,
			want: gpkg.HeaderTop{
				Flags: 0b0010_0000,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.h.SetType(tt.t)
			if !reflect.DeepEqual(tt.h, tt.want) {
				t.Errorf("HeaderTop.SetType() = %v, want %v", tt.h, tt.want)
			}
		})
	}
}

func TestEnvelopeContentsIndicatorCode_String(t *testing.T) {
	tests := []struct {
		name string
		c    gpkg.EnvelopeContentsIndicatorCode
		want string
	}{
		{
			name: "no envelope",
			c:    gpkg.NoEnvelope,
			want: "no envelope, 0 bytes",
		},
		{
			name: "XY envelope",
			c:    gpkg.XY,
			want: "envelope is [minx, maxx, miny, maxy], 32 bytes",
		},
		{
			name: "XYZ envelope",
			c:    gpkg.XYZ,
			want: "envelope is [minx, maxx, miny, maxy, minz, maxz], 48 bytes",
		},
		{
			name: "XYM envelope",
			c:    gpkg.XYM,
			want: "envelope is [minx, maxx, miny, maxy, minm, maxm], 48 bytes",
		},
		{
			name: "XYZM envelope",
			c:    gpkg.XYZM,
			want: "envelope is [minx, maxx, miny, maxy, minz, maxz, minm, maxm], 64 bytes",
		},
		{
			name: "invalid code",
			c:    5,
			want: "invalid envelope contents indicator code: 5",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.c.String(); got != tt.want {
				t.Errorf("EnvelopeContentsIndicatorCode.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEnvelopeContentsIndicatorCode_Size(t *testing.T) {
	tests := []struct {
		name string
		c    gpkg.EnvelopeContentsIndicatorCode
		want int
	}{
		{
			name: "no envelope",
			c:    gpkg.NoEnvelope,
			want: 0,
		},
		{
			name: "XY envelope",
			c:    gpkg.XY,
			want: 32,
		},
		{
			name: "XYZ envelope",
			c:    gpkg.XYZ,
			want: 48,
		},
		{
			name: "XYM envelope",
			c:    gpkg.XYM,
			want: 48,
		},
		{
			name: "XYZM envelope",
			c:    gpkg.XYZM,
			want: 64,
		},
		{
			name: "invalid code",
			c:    5,
			want: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.c.Size(); got != tt.want {
				t.Errorf("EnvelopeContentsIndicatorCode.Size() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestHeaderTop_EnvelopeContentsIndicatorCode(t *testing.T) {
	tests := []struct {
		name string
		h    gpkg.HeaderTop
		want gpkg.EnvelopeContentsIndicatorCode
	}{
		{
			name: "no envelope",
			h: gpkg.HeaderTop{
				Flags: 0b0000_0000,
			},
			want: gpkg.NoEnvelope,
		},
		{
			name: "XY envelope",
			h: gpkg.HeaderTop{
				Flags: 0b0000_0010,
			},
			want: gpkg.XY,
		},
		{
			name: "XYZ envelope",
			h: gpkg.HeaderTop{
				Flags: 0b0000_0100,
			},
			want: gpkg.XYZ,
		},
		{
			name: "XYM envelope",
			h: gpkg.HeaderTop{
				Flags: 0b0000_0110,
			},
			want: gpkg.XYM,
		},
		{
			name: "XYZM envelope",
			h: gpkg.HeaderTop{
				Flags: 0b0000_1000,
			},
			want: gpkg.XYZM,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.h.EnvelopeContentsIndicatorCode(); got != tt.want {
				t.Errorf("HeaderTop.EnvelopeContentsIndicatorCode() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestHeaderTop_SetEnvelopeContentsIndicatorCode(t *testing.T) {
	tests := []struct {
		name string
		h    gpkg.HeaderTop
		c    gpkg.EnvelopeContentsIndicatorCode
		want gpkg.HeaderTop
	}{
		{
			name: "set no envelope",
			h: gpkg.HeaderTop{
				Flags: 0b0000_0010,
			},
			c: gpkg.NoEnvelope,
			want: gpkg.HeaderTop{
				Flags: 0b0000_0000,
			},
		},
		{
			name: "set XY envelope",
			h: gpkg.HeaderTop{
				Flags: 0b0000_0000,
			},
			c: gpkg.XY,
			want: gpkg.HeaderTop{
				Flags: 0b0000_0010,
			},
		},
		{
			name: "set XYZ envelope",
			h: gpkg.HeaderTop{
				Flags: 0b0000_0000,
			},
			c: gpkg.XYZ,
			want: gpkg.HeaderTop{
				Flags: 0b0000_0100,
			},
		},
		{
			name: "set XYM envelope",
			h: gpkg.HeaderTop{
				Flags: 0b0000_0000,
			},
			c: gpkg.XYM,
			want: gpkg.HeaderTop{
				Flags: 0b0000_0110,
			},
		},
		{
			name: "set XYZM envelope",
			h: gpkg.HeaderTop{
				Flags: 0b0000_0000,
			},
			c: gpkg.XYZM,
			want: gpkg.HeaderTop{
				Flags: 0b0000_1000,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.h.SetEnvelopeContentsIndicatorCode(tt.c)
			if !reflect.DeepEqual(tt.h, tt.want) {
				t.Errorf("HeaderTop.SetEnvelopeContentsIndicatorCode() = %v, want %v", tt.h, tt.want)
			}
		})
	}
}

func TestHeaderTop_ByteOrder(t *testing.T) {
	tests := []struct {
		name string
		h    gpkg.HeaderTop
		want binary.ByteOrder
	}{
		{
			name: "big endian",
			h: gpkg.HeaderTop{
				Flags: 0b0000_0000,
			},
			want: binary.BigEndian,
		},
		{
			name: "little endian",
			h: gpkg.HeaderTop{
				Flags: 0b0000_0001,
			},
			want: binary.LittleEndian,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.h.ByteOrder(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("HeaderTop.ByteOrder() = %v, want %v", got, tt.want)
			}
		})
	}
}
