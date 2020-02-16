package tftp

import (
	"fmt"
	"log"
	"net"
	"reflect"
	"strconv"
	"time"
)

func resolveUDPAddr(localIpAddr string, localPort uint16) (*net.UDPAddr, error) {
	sockAddr := localIpAddr + ":" + strconv.Itoa(int(localPort))
	return net.ResolveUDPAddr("udp", sockAddr)
}

func readPacket(sock *net.UDPConn, buf []byte, timeoutSecond uint) (*Packet, *net.UDPAddr, error) {
	if err := sock.SetReadDeadline(time.Now().Add(time.Duration(timeoutSecond) * time.Second)); err != nil {
		return nil, nil, fmt.Errorf("[%v] Could not set read deadline of +%vms on socket: %w",
			sock.LocalAddr(), timeoutSecond, err)
	}
	n, addr, err := sock.ReadFromUDP(buf)
	if err != nil || n <= 0 {
		if n == 0 && addr == nil {
			return nil, nil, nil
		}
		return nil, addr, fmt.Errorf("[%v] Could not read from %v: %w ", sock.LocalAddr(), addr, err)
	}
	var pkt Packet
	if pkt, err = ParsePacket(buf[0:n]); err != nil {
		log.Printf("[%v] Ignored: received invalid packet from %v : %v\n", sock.LocalAddr(), addr, err)
		return nil, addr, nil
	}
	log.Printf("[%v] Read %vB from %v (%v)\n", sock.LocalAddr(), n, addr, reflect.TypeOf(pkt))
	return &pkt, addr, nil
}

func writeBuf(sock *net.UDPConn, buf []byte, timeoutSecond uint) (int, error) {
	if err := sock.SetWriteDeadline(time.Now().Add(time.Duration(timeoutSecond) * time.Second)); err != nil {
		return 0, fmt.Errorf("[%v] Could not set write deadline of +%vms on socket: %w",
			sock.LocalAddr(), timeoutSecond, err)
	}

	n, err := sock.Write(buf)
	if err != nil || n <= 0 {
		if n == 0 {
			return 0, nil // timeout
		}
		return n, err // write error
	} else {
		return n, nil // success
	}
}

func createSessionSocket(localInterface string, remoteAddr *net.UDPAddr) (*net.UDPConn, error) {
	if sockAddr, e := resolveUDPAddr(localInterface, 0); e != nil {
		return nil, fmt.Errorf("Invalid address [%v:0] : %w", localInterface, e) // wrap error
	} else {
		if sock, e := net.DialUDP("udp", sockAddr, remoteAddr); e != nil {
			return nil, fmt.Errorf("Session socket %v: %w", sockAddr, e)
		} else {
			return sock, nil
		}
	}
}
