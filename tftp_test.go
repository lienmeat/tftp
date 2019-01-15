package tftp

import (
	"bytes"
	"math"
	"reflect"
	"testing"

	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestProcessOpRRQ(t *testing.T) {
	log.SetLevel(log.DebugLevel)
	testFile := File{Filename: "test", Data: bytes.Repeat([]byte("test,test,test,test"), 1024*1024*35)}

	type args struct {
		file     File
		transfer *Transfer
		packet   *PacketRequest
	}
	tests := []struct {
		name         string
		args         args
		wantResponse Packet
		wantTransfer *Transfer
	}{
		{
			name: "ok",
			args: args{
				file:     testFile,
				transfer: &Transfer{File: NewFile("")},
				packet:   &PacketRequest{Op: OpRRQ, Filename: "test", Mode: "octet"},
			},
			wantResponse: &PacketData{BlockNum: 1, Data: testFile.Data[:BlockSize]},
			wantTransfer: &Transfer{File: testFile, Block: 2, Op: OpRRQ, Done: false, Error: false},
		},
		{
			name: "already sent data",
			args: args{
				file:     testFile,
				transfer: &Transfer{File: testFile, Block: 2, Op: OpRRQ, Done: false, Error: false},
				packet:   &PacketRequest{Op: OpRRQ, Filename: "test", Mode: "octet"},
			},
			wantResponse: nil,
			wantTransfer: &Transfer{File: testFile, Block: 2, Op: OpRRQ, Done: false, Error: false},
		},
		{
			name: "no such file",
			args: args{
				file:     NewFile("not test"),
				transfer: &Transfer{File: NewFile("")},
				packet:   &PacketRequest{Op: OpRRQ, Filename: "test", Mode: "octet"},
			},
			wantResponse: &PacketError{Code: 1, Msg: "file not found"},
			wantTransfer: &Transfer{File: NewFile(""), Done: true, Error: true},
		},
		{
			name: "bad file",
			args: args{
				file: File{
					Filename: "test",
					Data:     nil,
				},
				transfer: &Transfer{File: NewFile("")},
				packet:   &PacketRequest{Op: OpRRQ, Filename: "test", Mode: "octet"},
			},
			wantResponse: &PacketError{Code: 0, Msg: "no blocks in file"},
			wantTransfer: &Transfer{File: NewFile(""), Done: true, Error: true},
		},
		{
			name: "empty file",
			args: args{
				file:     NewFile("test"),
				transfer: &Transfer{File: NewFile("")},
				packet:   &PacketRequest{Op: OpRRQ, Filename: "test", Mode: "octet"},
			},
			wantResponse: &PacketData{BlockNum: 1, Data: []byte{}},
			wantTransfer: &Transfer{File: NewFile("test"), Block: 2, Op: OpRRQ, Done: true, Error: false},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			fr := NewFileRepo()
			fr.Set(tt.args.file)

			response := processOpRRQ(fr, tt.args.transfer, tt.args.packet)

			assert.Equal(t, tt.wantResponse, response)

			assert.Equal(t, tt.wantTransfer, tt.args.transfer)
		})
	}
}

func Test_processOpWRQ(t *testing.T) {
	type args struct {
		transfer *Transfer
		packet   *PacketRequest
	}
	tests := []struct {
		name         string
		args         args
		wantResponse Packet
		wantTransfer *Transfer
	}{
		{
			name: "ok",
			args: args{
				transfer: &Transfer{File: NewFile("")},
				packet:   &PacketRequest{Op: OpWRQ, Filename: "test", Mode: "octet"},
			},
			wantResponse: &PacketAck{BlockNum: 0},
			wantTransfer: &Transfer{File: NewFile("test"), Block: 1, Op: OpWRQ, Done: false, Error: false},
		},
		{
			name: "transfer already started",
			args: args{
				transfer: &Transfer{File: NewFile("something"), Block: 1},
				packet:   &PacketRequest{Op: OpWRQ, Filename: "test", Mode: "octet"},
			},
			wantResponse: nil,
			wantTransfer: &Transfer{File: NewFile("something"), Block: 1},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			response := processOpWRQ(tt.args.transfer, tt.args.packet)

			assert.Equal(t, tt.wantResponse, response)

			assert.Equal(t, tt.wantTransfer, tt.args.transfer)
		})
	}
}

func Test_processOpAck(t *testing.T) {
	log.SetLevel(log.DebugLevel)
	//need a file bigger than 35M to test block uint16 overflow
	testFile := File{Filename: "test", Data: bytes.Repeat([]byte("test,test,test,test"), 1024*1024*35)}
	d := []byte("a")
	b := []byte("b")
	bytes.Repeat(d, 1024*1024*35)
	d = append(d, bytes.Repeat(b, 1024*1024*35-10)...)
	type args struct {
		transfer *Transfer
		packet   *PacketAck
	}
	tests := []struct {
		name         string
		args         args
		wantResponse Packet
		wantTransfer *Transfer
	}{
		{
			name: "ok 1",
			args: args{
				transfer: &Transfer{
					File: File{
						Filename: "test",
						Data:     d,
					},
					Op:    OpRRQ,
					Block: 2,
				},
				packet: &PacketAck{BlockNum: 1},
			},
			wantResponse: &PacketData{BlockNum: 2, Data: d[BlockSize : 2*BlockSize]},
			wantTransfer: &Transfer{File: File{Filename: "test", Data: d}, Block: 3, Op: OpRRQ, Done: false, Error: false},
		},
		{
			name: "ok 2",
			args: args{
				transfer: &Transfer{
					File: File{
						Filename: "test",
						Data:     d,
					},
					Op:    OpRRQ,
					Block: 3,
				},
				packet: &PacketAck{BlockNum: 2},
			},
			wantResponse: &PacketData{BlockNum: 3, Data: d[2*BlockSize : 3*BlockSize]},
			wantTransfer: &Transfer{File: File{Filename: "test", Data: d}, Block: 4, Op: OpRRQ, Done: false, Error: false},
		},
		{
			name: "ok last",
			args: args{
				transfer: &Transfer{
					File: File{
						Filename: "test",
						Data:     bytes.Repeat([]byte("abcdefghijklmnopqrstuvwxyz"), 39),
					},
					Op:    OpRRQ,
					Block: 2,
				},

				packet: &PacketAck{BlockNum: 1},
			},
			wantResponse: &PacketData{BlockNum: 2, Data: bytes.Repeat([]byte("abcdefghijklmnopqrstuvwxyz"), 39)[512:]},
			wantTransfer: &Transfer{File: File{Filename: "test", Data: bytes.Repeat([]byte("abcdefghijklmnopqrstuvwxyz"), 39)}, Block: 3, Op: OpRRQ, Done: false, Error: false},
		},
		{
			name: "final ack after last block",
			args: args{
				transfer: &Transfer{
					File: File{
						Filename: "test",
						Data:     bytes.Repeat([]byte("abcdefghijklmnopqrstuvwxyz"), 39),
					},
					Op:    OpRRQ,
					Block: 3,
				},

				packet: &PacketAck{BlockNum: 2},
			},
			wantResponse: &PacketData{BlockNum: 3, Data: []byte{}},
			wantTransfer: &Transfer{File: File{Filename: "test", Data: bytes.Repeat([]byte("abcdefghijklmnopqrstuvwxyz"), 39)}, Block: 3, Op: OpRRQ, Done: true, Error: false},
		},
		{
			name: "wrong block in ack",
			args: args{
				transfer: &Transfer{
					File: File{
						Filename: "test",
						Data:     d,
					},
					Op:    OpRRQ,
					Block: 3,
				},
				packet: &PacketAck{BlockNum: 1},
			},
			wantResponse: nil,
			wantTransfer: &Transfer{File: File{Filename: "test", Data: d}, Block: 3, Op: OpRRQ, Done: false, Error: false},
		},
		{
			name: "block past uint16 overflow",
			args: args{
				transfer: &Transfer{File: testFile, Block: math.MaxUint16 + 1, Op: OpRRQ, Done: false, Error: false},
				packet:   &PacketAck{BlockNum: math.MaxUint16},
			},
			wantResponse: &PacketData{BlockNum: 0, Data: func() []byte { d, _ := testFile.ReadBlock(math.MaxUint16 + 1); return d }()},
			wantTransfer: &Transfer{File: testFile, Block: math.MaxUint16 + 2, Op: OpRRQ, Done: false, Error: false},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			response := processOpAck(tt.args.transfer, tt.args.packet)

			if !reflect.DeepEqual(tt.wantResponse, response) {
				t.Error("wrong response")
				//log.Debugf("response:%+v", string(response.(*PacketData).Data))
				//log.Debugf("expected:%+v", string(tt.wantResponse.(*PacketData).Data))
			}
			//assert.Equal(t, tt.wantResponse, response) //too verbose, crashes ide on large files

			if !reflect.DeepEqual(tt.wantTransfer, tt.args.transfer) {
				t.Error("wrong transfer data")
			}
		})
	}
}

func Test_processOpData(t *testing.T) {
	log.SetLevel(log.DebugLevel)
	//need a file bigger than 35M to test block uint16 rollover
	testFile := File{Filename: "test", Data: bytes.Repeat([]byte("test,test,test,test"), 1024*1024*35)}
	d := []byte("a")
	b := []byte("b")
	d = append(bytes.Repeat(d, 1024*1024*35), bytes.Repeat(b, 1024*1024*35-10)...)
	type args struct {
		transfer *Transfer
		packet   *PacketData
	}
	tests := []struct {
		name         string
		args         args
		wantResponse Packet
		wantTransfer *Transfer
	}{
		{
			name: "ok 1",
			args: args{
				transfer: &Transfer{
					File:  NewFile("test"),
					Op:    OpWRQ,
					Block: 1,
				},
				packet: &PacketData{BlockNum: 1, Data: bytes.Repeat([]byte("abcd"), int(BlockSize/4))},
			},
			wantResponse: &PacketAck{BlockNum: 1},
			wantTransfer: &Transfer{File: File{Filename: "test", Data: bytes.Repeat([]byte("abcd"), int(BlockSize/4))}, Block: 2, Op: OpWRQ, Done: false, Error: false},
		},
		{
			name: "last block",
			args: args{
				transfer: &Transfer{
					File: File{
						Filename: "test",
						Data:     bytes.Repeat([]byte("abcd"), int(BlockSize/4)),
					},
					Op:    OpWRQ,
					Block: 2,
				},
				packet: &PacketData{BlockNum: 2, Data: bytes.Repeat([]byte("abcd"), int(BlockSize/4)-1)},
			},
			wantResponse: &PacketAck{BlockNum: 2},
			wantTransfer: &Transfer{File: File{Filename: "test", Data: append(bytes.Repeat([]byte("abcd"), int(BlockSize/4)), bytes.Repeat([]byte("abcd"), int(BlockSize/4)-1)...)}, Block: 2, Op: OpWRQ, Done: true, Error: false},
		},
		{
			name: "block past uint16 overflow",
			args: args{
				transfer: &Transfer{
					File:  File{Filename: "test", Data: testFile.Data[:math.MaxUint16*512]},
					Block: math.MaxUint16 + 1,
					Op:    OpWRQ,
					Done:  false,
					Error: false,
				},
				packet: &PacketData{
					BlockNum: 0,
					Data:     func() []byte { d, _ := testFile.ReadBlock(math.MaxUint16 + 1); return d }(),
				},
			},
			wantResponse: &PacketAck{BlockNum: 0},
			wantTransfer: &Transfer{
				File: File{
					Filename: "test",
					Data:     testFile.Data[:math.MaxUint16*512+512],
				},
				Block: math.MaxUint16 + 2,
				Op:    OpWRQ,
				Done:  false,
				Error: false,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			response := processOpData(tt.args.transfer, tt.args.packet)

			assert.Equal(t, tt.wantResponse, response)

			assert.Equal(t, tt.wantTransfer, tt.args.transfer)
			//log.Debugf("want transfer: block=%d, op=%d, complete=%v, error=%v", tt.wantTransfer.Block, tt.wantTransfer.Op, tt.wantTransfer.Done, tt.wantTransfer.Error)
			//log.Debugf("transfer: block=%d, op=%d, complete=%v, error=%v", tt.args.transfer.Block, tt.args.transfer.Op, tt.args.transfer.Done, tt.args.transfer.Error)
		})
	}
}
