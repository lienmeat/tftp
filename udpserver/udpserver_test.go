package udpserver

import (
	"context"
	"net"
	"runtime"
	"strconv"
	"testing"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func Test_listener(t *testing.T) {
	log.SetLevel(log.DebugLevel)
	ctx := context.Background()
	ctx, done := context.WithCancel(ctx)

	conn, err := Connect("0.0.0.0:8000")
	if err != nil {
		panic(err)
	}

	cConn, err := Connect("127.0.0.1:8001")
	if err != nil {
		panic(err)
	}

	in := make(chan *UDPPacket)

	go listener(ctx, conn, in)

	addr := &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 8000}
	d := []byte("ok")
	cConn.WriteToUDP(d, addr)

	out := <-in
	assert.Equal(t, &UDPPacket{addr: &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 8001}, data: d}, out)

	d = []byte("ok2")
	cConn.WriteToUDP(d, addr)

	done()
	out = <-in

	assert.Equal(t, &UDPPacket{addr: &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 8001}, data: d}, out)

	d = []byte("ok3")
	cConn.WriteToUDP(d, addr)

	select {
	case <-in:
		t.Error("should not receive on channel, listener didn't exit")
	case <-time.After(time.Millisecond):
		return
	}
}

func Test_responder(t *testing.T) {
	log.SetLevel(log.DebugLevel)
	ctx := context.Background()
	ctx, done := context.WithCancel(ctx)

	conn, err := Connect("0.0.0.0:8002")
	if err != nil {
		panic(err)
	}

	cConn, err := Connect("0.0.0.0:8003")
	if err != nil {
		panic(err)
	}

	responces := make(chan *UDPPacket)

	go responder(ctx, conn, responces)

	addr := &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 8003}
	data := []byte("ok")

	responces <- NewUDPPacket(addr, data)

	buff := make([]byte, 1028)

	n, _, err := cConn.ReadFromUDP(buff)
	if err != nil {
		t.Error(err)
	}

	assert.Equal(t, data, buff[:n])

	data = []byte("ok2")
	responces <- NewUDPPacket(addr, data)

	buff = make([]byte, 1028)

	n, _, err = cConn.ReadFromUDP(buff)
	if err != nil {
		t.Error(err)
	}

	assert.Equal(t, data, buff[:n])

	done()

	//it can take a tiny bit for responder to hang up
	time.Sleep(time.Millisecond * 5)

	data = []byte("ok3")

	select {
	case responces <- NewUDPPacket(addr, data):
		t.Error("should not send on channel, responder didn't exit")
	case <-time.After(time.Millisecond):
		return
	}
}

func Test_connect(t *testing.T) {
	log.SetLevel(log.DebugLevel)
	type args struct {
		address string
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name:    "ipv4",
			args:    args{address: "0.0.0.0:8004"},
			wantErr: false,
		},
		{
			name:    "ipv6",
			args:    args{address: "[::1]:8005"},
			wantErr: false,
		},
		{
			name:    "can't connect",
			args:    args{address: "192.0.0.1:8006"}, //some address you can't connect to here...
			wantErr: true,
		},
		{
			name:    "can't resolve",
			args:    args{address: "a:0:8007"},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Connect(tt.args.address)
			if (err != nil) != tt.wantErr {
				t.Errorf("connect() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}

func Benchmark_listenerMany(b *testing.B) {
	var err error
	var addr *net.UDPAddr
	var sc *net.UDPConn
	var cc *net.UDPConn

	if addr == nil {
		addr, err = net.ResolveUDPAddr("udp", "127.0.0.1:9090")
		if err != nil {
			panic(err)
		}
	}

	if cc == nil {
		cc, err = Connect("0.0.0.0:9091")
		if err != nil {
			panic(err)
		}
	}

	b.ReportAllocs()
	in := make(chan *UDPPacket)
	ctx := context.Background()
	for k := 1; k <= runtime.NumCPU()*4; k *= 2 {
		b.Run(strconv.Itoa(k), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				b.StopTimer()
				sc, err = Connect("0.0.0.0:9090")
				if err != nil {
					panic(err)
				}
				b_listener(ctx, k, in, addr, sc, cc, b)
				sc.Close()
			}
		})
	}
}

func b_listener(ctx context.Context, listeners int, in chan *UDPPacket, addr *net.UDPAddr, sc *net.UDPConn, cc *net.UDPConn, b *testing.B) {
	for i := 0; i < listeners; i++ {
		go listener(ctx, sc, in)
	}
	go func() {
		for i := 0; i < 100; i++ {
			cc.WriteToUDP([]byte("something"), addr)
		}
	}()
	b.StartTimer()
	for c := 0; c < 100; c++ {
		<-in
	}
	return
}

func Benchmark_responderMany(b *testing.B) {
	var err error
	var addr *net.UDPAddr
	var sc *net.UDPConn
	var cc *net.UDPConn

	if addr == nil {
		addr, err = net.ResolveUDPAddr("udp", "127.0.0.1:9090")
		if err != nil {
			panic(err)
		}
	}

	if cc == nil {
		cc, err = Connect("0.0.0.0:9091")
		if err != nil {
			panic(err)
		}
	}

	b.ReportAllocs()
	in := make(chan *UDPPacket)
	ctx := context.Background()
	for k := 1; k <= runtime.NumCPU()*4; k *= 2 {
		b.Run(strconv.Itoa(k), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				b.StopTimer()
				sc, err = Connect("0.0.0.0:9090")
				if err != nil {
					panic(err)
				}
				b_responder(ctx, k, in, addr, sc, cc, b)
				sc.Close()
			}
		})
	}
}

func b_responder(ctx context.Context, listeners int, in chan *UDPPacket, addr *net.UDPAddr, sc *net.UDPConn, cc *net.UDPConn, b *testing.B) {
	for i := 0; i < listeners; i++ {
		go responder(ctx, sc, in)
	}
	b.StartTimer()
	for i := 0; i < 100; i++ {
		in <- NewUDPPacket(addr, []byte("something"))
	}
	return
}
