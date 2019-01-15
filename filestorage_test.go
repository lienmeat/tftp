package tftp

import (
	"bytes"
	"math"
	"testing"

	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestFile_WriteBlock(t *testing.T) {
	log.SetLevel(log.DebugLevel)
	type fields struct {
		data []byte
	}
	type args struct {
		n     uint
		block []byte
	}
	tests := []struct {
		name     string
		fields   fields
		args     args
		wantSize uint
		wantErr  bool
		wantData []byte
	}{
		{
			name: "first block",
			fields: fields{
				data: []byte{},
			},
			args: args{
				n:     1,
				block: bytes.Repeat([]byte("a"), 512),
			},
			wantSize: 512,
			wantErr:  false,
			wantData: bytes.Repeat([]byte("a"), 512),
		},
		{
			name: "first block not full",
			fields: fields{
				data: []byte{},
			},
			args: args{
				n:     1,
				block: bytes.Repeat([]byte("a"), 200),
			},
			wantSize: 200,
			wantErr:  false,
			wantData: bytes.Repeat([]byte("a"), 200),
		},
		{
			name: "2nd block",
			fields: fields{
				data: bytes.Repeat([]byte("a"), 512),
			},
			args: args{
				n:     2,
				block: bytes.Repeat([]byte("a"), 512),
			},
			wantSize: 1024,
			wantErr:  false,
			wantData: bytes.Repeat([]byte("a"), 1024),
		},
		{
			name: "2nd block not full",
			fields: fields{
				data: bytes.Repeat([]byte("a"), 512),
			},
			args: args{
				n:     2,
				block: bytes.Repeat([]byte("a"), 200),
			},
			wantSize: 712,
			wantErr:  false,
			wantData: bytes.Repeat([]byte("a"), 712),
		},
		{
			name: "2nd block empty file",
			fields: fields{
				data: []byte{},
			},
			args: args{
				n:     2,
				block: bytes.Repeat([]byte("a"), 512),
			},
			wantSize: 0,
			wantErr:  true,
			wantData: []byte{},
		},
		{
			name: "block too large",
			fields: fields{
				data: []byte{},
			},
			args: args{
				n:     1,
				block: bytes.Repeat([]byte("a"), 513),
			},
			wantSize: 0,
			wantErr:  true,
			wantData: []byte{},
		},
		{
			name: "write past uint16 overflow",
			fields: fields{
				data: bytes.Repeat([]byte("a"), math.MaxUint16*512),
			},
			args: args{
				n:     math.MaxUint16 + 1,
				block: bytes.Repeat([]byte("b"), 5),
			},
			wantSize: math.MaxUint16*512 + 5,
			wantData: append(bytes.Repeat([]byte("a"), math.MaxUint16*512), bytes.Repeat([]byte("b"), 5)...),
			wantErr:  false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := &File{
				Data: tt.fields.data,
			}
			gotSize, err := f.WriteBlock(tt.args.n, tt.args.block)
			if (err != nil) != tt.wantErr {
				t.Errorf("File.WriteBlock() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			assert.Equal(t, tt.wantSize, gotSize)
			assert.Equal(t, tt.wantData, f.Data)
		})
	}
}

func TestFile_ReadBlock(t *testing.T) {
	type fields struct {
		data []byte
	}
	type args struct {
		n uint
	}
	tests := []struct {
		name      string
		fields    fields
		args      args
		wantBlock []byte
		wantOk    bool
	}{
		{
			name: "1st",
			fields: fields{
				data: bytes.Repeat([]byte("a"), 1024),
			},
			args: args{
				n: 1,
			},
			wantBlock: bytes.Repeat([]byte("a"), 512),
			wantOk:    true,
		},
		{
			name: "1st small",
			fields: fields{
				data: bytes.Repeat([]byte("a"), 200),
			},
			args: args{
				n: 1,
			},
			wantBlock: bytes.Repeat([]byte("a"), 200),
			wantOk:    true,
		},
		{
			name: "1st empty file",
			fields: fields{
				data: []byte{},
			},
			args: args{
				n: 1,
			},
			wantBlock: []byte{},
			wantOk:    true,
		},
		{
			name: "nil file",
			fields: fields{
				data: nil,
			},
			args: args{
				n: 1,
			},
			wantBlock: []byte{},
			wantOk:    false,
		},
		{
			name: "2nd",
			fields: fields{
				data: append(bytes.Repeat([]byte("a"), 512), bytes.Repeat([]byte("b"), 512)...),
			},
			args: args{
				n: 2,
			},
			wantBlock: bytes.Repeat([]byte("b"), 512),
			wantOk:    true,
		},
		{
			name: "2nd small",
			fields: fields{
				data: append(bytes.Repeat([]byte("a"), 512), bytes.Repeat([]byte("b"), 200)...),
			},
			args: args{
				n: 2,
			},
			wantBlock: bytes.Repeat([]byte("b"), 200),
			wantOk:    true,
		},
		{
			name: "2nd no 2nd block",
			fields: fields{
				data: bytes.Repeat([]byte("a"), 512),
			},
			args: args{
				n: 2,
			},
			wantBlock: []byte{},
			wantOk:    false,
		},
		{
			name: "read past uint16 overflow",
			fields: fields{
				data: append(bytes.Repeat([]byte("a"), math.MaxUint16*512), bytes.Repeat([]byte("b"), 5)...),
			},
			args: args{
				n: math.MaxUint16 + 1,
			},
			wantBlock: bytes.Repeat([]byte("b"), 5),
			wantOk:    true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := &File{
				Data: tt.fields.data,
			}
			gotBlock, gotOk := f.ReadBlock(tt.args.n)
			assert.Equal(t, gotBlock, tt.wantBlock)
			assert.Equal(t, gotOk, tt.wantOk)
		})
	}
}
