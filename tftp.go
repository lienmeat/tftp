package tftp

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/lienmeat/tftp/udpserver"
	log "github.com/sirupsen/logrus"
)

var RequestLog = log.New()

func SetupRequestLog(filename string) (*os.File, error) {
	RequestLog.SetLevel(log.InfoLevel)
	rl, err := os.OpenFile(filename, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0775)
	if err == nil {
		RequestLog.SetOutput(rl)
	} else {
		return rl, fmt.Errorf("err opening %s file: %s", filename, err.Error())
	}
	return rl, nil
}

//Transfer keeps track of the data associated with a file in transfer
type Transfer struct {
	//file data for this transfer
	File File
	//block currently on/expecting in next data or ack
	Block uint
	//read or write transfer
	Op uint16
	//finished?
	Done bool
	//error on transfer?
	Error bool
}

func (t Transfer) OpString() string {
	if t.Op == OpWRQ {
		return "put"
	}
	return "get"
}

//TFTProtocolHandler handles UDPPackets to implement the TFTP business logic
type TFTPProtocolHandler struct {
	Files          *FileRepo
	TIDs           *TIDRepo
	writeTransfers *WriteTransferRepo
}

func NewTFTPProtocolHandler() *TFTPProtocolHandler {
	return &TFTPProtocolHandler{
		TIDs:           NewTIDRepo(6000, 9000),
		Files:          NewFileRepo(),
		writeTransfers: NewWriteTransferRepo(),
	}
}

func (h *TFTPProtocolHandler) HandlePackets(ctx context.Context, incoming chan *udpserver.UDPPacket, responses chan *udpserver.UDPPacket) {
	for {
		select {
		case <-ctx.Done():
			return
		case p := <-incoming:
			go h.newWorker(ctx, p)
		}
	}
}

func (h *TFTPProtocolHandler) newWorker(ctx context.Context, packet *udpserver.UDPPacket) {
	iTid := h.TIDs.New()
	addr := "0.0.0.0:" + strconv.Itoa(int(iTid))
	connection, err := udpserver.Connect(addr)
	if err != nil {
		log.WithFields(log.Fields{
			"addr": addr,
		}).Error("could not connect")
	}
	defer func() {
		connection.Close()
		h.TIDs.Del(iTid)
	}()

	in := udpserver.DispatchListeners(ctx, connection, 2)
	out := udpserver.DispatchResponseWriters(ctx, connection, 1)
	in <- packet
	h.transferWorker(ctx, in, out)
}

//transferWorker runs until it has handled one complete transfer, receiving UDPPackets for it's TID/port
//via a channel and sending response packets out on another
func (h *TFTPProtocolHandler) transferWorker(ctx context.Context, in <-chan *udpserver.UDPPacket, out chan<- *udpserver.UDPPacket) {

	transfer := Transfer{
		File: NewFile(""),
	}

	var retries = 5
	var lastResponse *udpserver.UDPPacket
	for retries > 0 {
		select {
		case <-ctx.Done():
			return
		case p := <-in:
			retries = 5
			parsed, err := ParsePacket(p.Data())
			if err != nil {
				RequestLog.WithFields(log.Fields{
					"address": p.Address(),
					"packet":  string(p.Data()),
				}).Errorf("unknown request")
				return
			}
			resp := processPacket(h.Files, &transfer, h.writeTransfers, p, parsed)
			if resp != nil {
				lastResponse = udpserver.NewUDPPacket(p.Address(), resp.Serialize())
				out <- lastResponse
			}
			if transfer.Done && !transfer.Error {
				if transfer.Op == OpWRQ {
					h.Files.Set(transfer.File)
				}
				RequestLog.WithFields(log.Fields{
					"address":  p.Address(),
					"filename": transfer.File.Filename,
					"size":     len(transfer.File.Data),
					"op":       transfer.OpString(),
				}).Info("transfer complete")
				return
			}
		case <-time.After(time.Second * 3):
			//replay the last sent packet if we haven't gotten a response after 3 seconds
			if len(lastResponse.Data()) > 0 {
				out <- lastResponse
				retries--
			}
		}
	}
	if retries == 0 {
		return
	}
}

//processPacket returns the correct response if any for a given packet, and modifies
//the transfer state
func processPacket(files *FileRepo, transfer *Transfer, writeTransfers *WriteTransferRepo, raw *udpserver.UDPPacket, p Packet) (response Packet) {
	switch p.(type) {
	case *PacketRequest:
		r := p.(*PacketRequest)
		if r.Op == OpRRQ {
			RequestLog.WithFields(log.Fields{
				"filename": r.Filename,
				"address":  raw.Address(),
				"op":       "get",
			}).Infof("get %s transfer requested", r.Filename)
			return processOpRRQ(files, transfer, r)
		}
		if writeTransfers.CanWrite(r.Filename) {
			RequestLog.WithFields(log.Fields{
				"filename": r.Filename,
				"address":  raw.Address(),
				"op":       "put",
			}).Infof("put %s transfer requested", r.Filename)
			return processOpWRQ(transfer, r)
		} else {
			return &PacketError{
				Msg:  "Write already in progress for " + r.Filename,
				Code: 6,
			}
		}
	case *PacketAck:
		return processOpAck(transfer, p.(*PacketAck))
	case *PacketData:
		return processOpData(transfer, p.(*PacketData))
	case *PacketError:
		transfer.Done = true
		transfer.Error = true
		return nil
	}
	return nil
}

func processOpRRQ(files *FileRepo, transfer *Transfer, r *PacketRequest) (response Packet) {
	if transfer.Block != 0 || transfer.Op != 0 {
		return
	}
	file, ok := files.Get(r.Filename)
	if !ok {
		//file doesn't exist
		response = &PacketError{
			Code: 1,
			Msg:  "file not found",
		}
		transfer.Done = true
		transfer.Error = true
	} else {
		d, ok := file.ReadBlock(1)
		if !ok {
			response = &PacketError{
				Code: 0,
				Msg:  "no blocks in file",
			}
			transfer.Done = true
			transfer.Error = true
		} else {
			transfer.Op = r.Op
			transfer.File = file
			transfer.Block = 2
			response = &PacketData{
				BlockNum: 1,
				Data:     d,
			}
			if len(d) < int(BlockSize) {
				transfer.Done = true
			}
		}
	}
	return
}

func processOpWRQ(transfer *Transfer, r *PacketRequest) (response Packet) {
	if transfer.Op == 0 && transfer.Block == 0 {
		transfer.Op = r.Op
		transfer.File.Filename = r.Filename
		transfer.Block = 1
		response = &PacketAck{BlockNum: 0}
	}
	return
}

func processOpAck(transfer *Transfer, r *PacketAck) (response Packet) {
	//compare the request block # to the overflowed transfer block #
	block := uint16(transfer.Block)
	if block-1 != r.BlockNum {
		return
	}
	if d, ok := transfer.File.ReadBlock(transfer.Block); ok {
		//even if this block is short, clients still send a final ack
		//so we need to not complete until we receive that next ack
		transfer.Block++
		response = &PacketData{
			BlockNum: r.BlockNum + 1,
			Data:     d,
		}
	} else {
		//catch-all for last-block ack, or if the last block was exactly 512 bytes
		//block is past the end of the file in either case, return an empty block to stop the transfer
		transfer.Done = true
		response = &PacketData{
			BlockNum: r.BlockNum + 1,
			Data:     []byte{},
		}
	}
	return
}

func processOpData(transfer *Transfer, r *PacketData) (response Packet) {
	//compare the request block # to the overflowed transfer block #
	block := uint16(transfer.Block)
	if block != r.BlockNum {
		return
	}
	if _, err := transfer.File.WriteBlock(transfer.Block, r.Data); err == nil {
		if len(r.Data) < int(BlockSize) {
			transfer.Done = true
		} else {
			transfer.Block++
		}
		response = &PacketAck{BlockNum: r.BlockNum}
	}
	return
}

//TIDRepo is a concurrent-safe storage of TIDs that transfer workers are using to get packets routed to them via
type TIDRepo struct {
	min  int32
	size int32
	tt   []bool
	sync.RWMutex
}

func NewTIDRepo(min, max int32) *TIDRepo {
	return &TIDRepo{
		min:  min,
		size: max - min,
		tt:   make([]bool, max-min),
	}
}

func (r *TIDRepo) New() int32 {
	rand.Seed(time.Now().UnixNano())
	r.Lock()
	defer r.Unlock()
	for i := 0; i < 100; i++ {
		n := rand.Int31n(r.size)
		if !r.tt[n] {
			r.tt[n] = true
			return r.min + n
		}
	}
	return -1
}

func (r *TIDRepo) Del(tid int32) {
	r.Lock()
	defer r.Unlock()
	r.tt[int(tid-r.min)] = false
}

//WriteTransferRepo stores if a file is currently being written to via a transfer
type WriteTransferRepo struct {
	transfers map[string]bool
	sync.Mutex
}

func NewWriteTransferRepo() *WriteTransferRepo {
	return &WriteTransferRepo{
		transfers: map[string]bool{},
	}
}

func (r *WriteTransferRepo) CanWrite(filename string) bool {
	r.Lock()
	defer r.Unlock()
	_, ok := r.transfers[filename]
	if !ok {
		r.transfers[filename] = true
	}
	return !ok
}

func (r *WriteTransferRepo) Del(filename string) {
	r.Lock()
	defer r.Unlock()
	delete(r.transfers, filename)
}
