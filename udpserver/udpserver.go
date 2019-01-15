package udpserver

import (
	"context"
	"fmt"
	"net"
	"runtime"

	log "github.com/sirupsen/logrus"
)

// maxBufferSize specifies the size of the buffers that
// are used to temporarily hold data from the UDP packets
// that we receive.
const maxBufferSize = 1024

//UDPPacket stores client address/port/tid along with the the packet data
//which is passed into ProtocolHandlers or around other components of udpserver
type UDPPacket struct {
	addr *net.UDPAddr
	data []byte
}

func NewUDPPacket(addr *net.UDPAddr, data []byte) *UDPPacket {
	return &UDPPacket{
		addr: addr,
		data: data,
	}
}

func (p *UDPPacket) SetData(data []byte) {
	p.data = data
}

func (p *UDPPacket) Data() []byte {
	return p.data
}

func (p *UDPPacket) Address() *net.UDPAddr {
	return p.addr
}

//ProtocolHandler is the interface that protocol handlers must implement
type ProtocolHandler interface {
	HandlePackets(ctx context.Context, incoming chan *UDPPacket, responses chan *UDPPacket)
}

func DispatchResponseWriters(ctx context.Context, connection *net.UDPConn, bufferSize int) chan *UDPPacket {
	ch := make(chan *UDPPacket, bufferSize)
	go responder(ctx, connection, ch)
	return ch
}

func responder(ctx context.Context, connection *net.UDPConn, ch <-chan *UDPPacket) {
	for {
		select {
		case p := <-ch:
			connection.WriteToUDP(p.Data(), p.Address())
		case <-ctx.Done():
			return
		}
	}
}

func DispatchListeners(ctx context.Context, connection *net.UDPConn, bufferSize int) chan *UDPPacket {
	ch := make(chan *UDPPacket, bufferSize)
	go listener(ctx, connection, ch)
	return ch
}

func listener(ctx context.Context, connection *net.UDPConn, in chan<- *UDPPacket) {
	buffer := make([]byte, maxBufferSize)
	for {
		log.Debug("waiting for packet")
		n, addr, err := connection.ReadFromUDP(buffer)
		if err == nil {
			log.Debugf("got packet %s", string(buffer[:n]))
			select {
			case <-ctx.Done():
				return
			case in <- NewUDPPacket(addr, buffer[:n]):
				log.Debugf("sent packet %s", string(buffer[:n]))
			}
		} else {
			//closed connection, exit
			log.WithFields(
				log.Fields{
					"context":    "listener()",
					"connection": connection,
					"error":      err,
				}).Debug("error reading from UDP connection")
			return
		}
		//clear out buffer
		buffer = make([]byte, maxBufferSize)
		select {
		case <-ctx.Done():
			return
		default:
			break
		}
	}
}

func Connect(address string) (*net.UDPConn, error) {
	udpAddress, err := net.ResolveUDPAddr("udp", address)
	if err != nil {
		msg := fmt.Sprintf("error resolving udp address %s: %s", address, err)
		log.Error(msg)
		return nil, fmt.Errorf(msg)
	}

	connection, err := net.ListenUDP("udp", udpAddress)
	if err != nil {
		msg := fmt.Sprintf("error listening on udp address %s: %s", address, err)
		log.Error(msg)
		return nil, fmt.Errorf(msg)
	}
	return connection, nil
}

func Server(ctx context.Context, address string, handler ProtocolHandler) (err error) {
	log.Info("Starting udp server at " + address)

	connection, err := Connect(address)
	if err != nil {
		return err
	}

	defer connection.Close()

	incoming := DispatchListeners(ctx, connection, runtime.NumCPU())
	responses := DispatchResponseWriters(ctx, connection, runtime.NumCPU())

	go handler.HandlePackets(ctx, incoming, responses)

	<-ctx.Done()
	return
}
