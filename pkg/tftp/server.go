package tftp

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
)

type Server struct {
	Conf       *Config      // server configuration
	Log        *Logger      // for logging to files and console
	Files      *FileManager // file handling is delegated to FileManager
	ListenSock *net.UDPConn

	Running              bool
	ReceivedRequestCount uint
}

func (svr *Server) Init() (err error) {
	if svr.Running {
		log.Panic("init but running")
	}

	// Init configuration object:
	svr.Conf = new(Config)
	err = svr.Conf.Init()
	if err != nil {
		return
	}

	// Init logger:
	svr.Log = new(Logger)
	err = svr.Log.Init(svr.Conf.MainLogFileName, svr.Conf.RequestsLogFileName)
	if err != nil {
		return
	}

	// Init file manager:
	svr.Files = new(FileManager)
	err = svr.Files.Init()
	if err != nil {
		return
	}

	//
	go svr.AdminRestInterface()

	// create the server's listening socket:
	if addr, e := svr.Conf.ListenAddr(); e != nil {
		return e
	} else {
		svr.ListenSock, e = net.ListenUDP("udp", addr)
		if e != nil {
			return fmt.Errorf("Could not listen on UDP socket %v : %w", addr, e)
		}
	}
	return
}

func (svr *Server) DeInit() (err error) {
	if svr.Conf != nil {
		svr.Conf.DeInit()
	}
	if svr.Log != nil {
		svr.Log.DeInit()
	}
	if svr.Files != nil {
		svr.Files.DeInit()
	}
	if svr.ListenSock != nil {
		svr.ListenSock.Close()
	}
	return
}

func (svr *Server) AdminRestInterface() {
	status := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8") // normal header
		if b, err := json.Marshal(svr); err != nil {
			fmt.Fprint(w, err.Error())
		} else {
			fmt.Fprint(w, string(b))
		}
	}
	http.HandleFunc("/", status)
	http.HandleFunc("/shutdown", func(w http.ResponseWriter, r *http.Request) {
		log.Println("[REST] /shutdown")
		svr.Running = false
	})
	http.HandleFunc("/clear", func(w http.ResponseWriter, r *http.Request) {
		log.Println("[REST] /clear")
		for k := range svr.Files.files {
			delete(svr.Files.files, k)
		}
	})
	log.Println("Admin REST Interface at", svr.Conf.AdminRestAddress)
	log.Fatal(http.ListenAndServe(svr.Conf.AdminRestAddress, nil))

}

func (svr *Server) AcceptLoop() (err error) {
	logHdr := fmt.Sprintf("[%v] ", svr.ListenSock.LocalAddr())
	log.Println(logHdr, "Ready to accept clients..")
	var pkt_buf []byte = make([]byte, MaxPacketSize)
	svr.Running = true
	for svr.Running {
		pkt, addr, err := readPacket(svr.ListenSock, pkt_buf, 5)
		if err != nil { // error on socket
			return err
		}
		if pkt == nil { // timeout on socket
			continue
		}

		switch (*pkt).(type) {
		case *PacketRequest:
			go svr.ProcessRequest((*pkt).(*PacketRequest), addr)

		// The listening socket deals only with incoming requests, and hand them
		// of to session sockets.
		// Any other packet is erroneous and ignored below.
		default:
			log.Printf("Ignored: received a %T packet on listening socket\n", pkt)
		}

	}
	log.Println("tftpd shutting down")
	return
}

// Sends an error packet to the client, and do not wait for a response.
func (svr *Server) SendError(clientAddr *net.UDPAddr, code uint16, msg string) {
	pktErr := PacketError{code, msg}
	buf := pktErr.Serialize()
	_, _ = svr.ListenSock.WriteToUDP(buf, clientAddr)
}

func (svr *Server) ProcessRequest(reqPacket *PacketRequest, clientAddr *net.UDPAddr) {
	svr.ReceivedRequestCount++
	// Implementation note:
	// In case of immediate error, we write back to the client from the listen thread,
	// on the listening socket , and NOT from the processRequest goroutine:
	// the rationale for this is
	// 1) the client expects the error packet to come from the same socket it sent its request to,
	// and
	// 2) I am not certain it is universal across all IP stacks that a UDP socket can
	// be reading/blocked and writing at the same time from different threads.
	if reqPacket.Mode != "octet" {
		svr.SendError(clientAddr, errIllegalOp, "Mode not supported")
		// log the request anyways:
		svr.Log.LogRequest(clientAddr.String(), reqPacket.String(),
			"Ignored: client request not in octet mode.")
		return
	}
	if reqPacket.Op != OpRRQ && reqPacket.Op != OpWRQ {
		svr.SendError(clientAddr, errIllegalOp, "Unknown request type")
		// log the request anyways:
		svr.Log.LogRequest(clientAddr.String(), reqPacket.String(),
			"Ignored: Unknown request type.")
		return
	}

	go svr.processRequest(reqPacket, clientAddr)
}

func (svr *Server) processRequest(reqPacket *PacketRequest, clientAddr *net.UDPAddr) {
	// Create a session socket 'sock' for processing this request:
	// Exchange with this client is done with this new socket; The listen socket
	// svr.ListenSock is reserved for listening for incoming requests.
	sock, err := createSessionSocket(svr.Conf.LocalInterface, clientAddr)
	if err != nil {
		log.Println("ERROR: Could not create socket for session with ",
			clientAddr, ":", err)
		return
	}
	sockClose := func() {
		s := sock.LocalAddr().String()
		if e := sock.Close(); e != nil {
			log.Println("while closing socket", s, ":", e)
		} else {
			log.Println(s, "closed")
		}
	}
	defer sockClose()

	// log the request:
	svr.Log.LogRequest(clientAddr.String(), reqPacket.String(),
		fmt.Sprintf("Processing request %v<-->%v", sock.LocalAddr(), sock.RemoteAddr()))

	switch reqPacket.Op {
	case OpRRQ:
		err = svr.ProcessReadRequest(sock, reqPacket, clientAddr)

	case OpWRQ:
		err = svr.ProcessWriteRequest(sock, reqPacket, clientAddr)
	default:
		// spurious request types were already handled from ProcessRequest()
	}
	if err != nil {
		log.Printf("[%v] session with %v aborted: %v", sock.LocalAddr(),
			clientAddr, err.Error())
	}

}

func (svr *Server) ProcessWriteRequest(sock *net.UDPConn, req *PacketRequest, clientAddr *net.UDPAddr) (err error) {

	// Files.Put() returns an iterator on the file to write:
	fileIter, err := svr.Files.Put(req.Filename)
	if err != nil {
		svr.SendError(clientAddr, errFileAlreadyExists, err.Error())
		return err
	}

	for blockNumber := uint16(1); ; blockNumber++ {

		dataBuf, err := lockStepReceiveData(sock, blockNumber, clientAddr,
			svr.Conf.MaxSendTries, svr.Conf.socketTimeoutSecs)
		if err != nil {
			return err
		}
		fileIter.Write(dataBuf)
		// TODO: set a maximum file size, otherwise this for loop can go on forever

		if uint16(len(dataBuf)) < svr.Conf.DataPayloadSize {
			// the payload is not the max size => it means it was the last block in the transmission.

			// we need to send the final ACK (and we don't check if it is received)
			sendAck(sock, blockNumber, clientAddr, svr.Conf.socketTimeoutSecs)

			log.Println("Done: Received file", req.Filename, "from", clientAddr)
			break
		}
	}
	return nil
}

func sendAck(sock *net.UDPConn, blockNumber uint16, clientAddr *net.UDPAddr,
	socketTimeoutSecs uint) (timeout bool, err error) {
	ackPacket := PacketAck{blockNumber}
	if n, e := writeBuf(sock, ackPacket.Serialize(), socketTimeoutSecs); e != nil {
		return false, fmt.Errorf("sending ACK: %w", e) // fail on write error.
	} else {
		if n == 0 {
			return true, nil // Timed out
		}
		log.Printf("[%v] Sent ACK#%v to %v (%vB)\n", sock.LocalAddr(), blockNumber, clientAddr, n)
	}
	return false, nil
}
func lockStepReceiveData(sock *net.UDPConn, blockNumber uint16, clientAddr *net.UDPAddr,
	MaxSendTries uint, socketTimeoutSecs uint) ([]byte, error) {

	//  Try loop
	for triesLeft := MaxSendTries; triesLeft >= 0; triesLeft-- {
		if triesLeft == 0 {
			return nil, fmt.Errorf(
				"no data from client after sending ack#%v %v times",
				blockNumber-1, MaxSendTries)
		}

		// Send the ack for the previous block.
		// In the case of the first block, we are thus sending an ACK for block #0 , which is
		// TFTP's way to initiate the lockstep transmission:

		if timeout, e := sendAck(sock, blockNumber-1, clientAddr, socketTimeoutSecs); e != nil {
			return nil, e // fail on write error.
		} else {
			if timeout {
				continue // Timed out. try again
			}
		}

		// Receive the packet:
		var readPacketBuf = make([]byte, MaxPacketSize)
		if responsePkt, _, e := readPacket(sock, readPacketBuf, socketTimeoutSecs); e != nil {
			return nil, e // fail on read error
		} else {
			if responsePkt == nil {
				continue // Timed out. try again
			} else {
				resend, err := processDataPacket(blockNumber, responsePkt, clientAddr)
				if err != nil {
					return nil, err // invalid response
				}
				if resend {
					continue // client needs a resend of the previous ACK
				}
				// success: we received a data packet,
				// and its block number is the previous one + 1 .
				dataBuf := (*responsePkt).(*PacketData).Data
				log.Printf("[%v] Received data block#%v from %v %vB\n",
					sock.LocalAddr(), blockNumber, clientAddr, len(dataBuf))
				return dataBuf, nil
			}
		}

	}
	return nil, nil // unreachable.
}
func processDataPacket(blockNumber uint16, responsePkt *Packet,
	clientAddr *net.UDPAddr) (resend bool, err error) {
	switch (*responsePkt).(type) {
	case *PacketData:
		dataPkt := (*responsePkt).(*PacketData)
		switch dataPkt.BlockNum {
		case blockNumber:
			return false, nil
		case blockNumber - 1:
			return true, nil
		default:
			return false, fmt.Errorf("invalid data packet from client: "+
				"current block is #%v,client asked for #%v",
				blockNumber, dataPkt.BlockNum)
		}
	default:
		return false, fmt.Errorf(
			"received non data packet after sending ACK#%blockNumber: %v",
			blockNumber, responsePkt)
	}
}

func (svr *Server) ProcessReadRequest(sock *net.UDPConn, req *PacketRequest,
	clientAddr *net.UDPAddr) (err error) {

	// Files.Get() returns an iterator on the file to read:
	var fileIter *FileIterator
	fileIter, err = svr.Files.Get(req.Filename, int(svr.Conf.DataPayloadSize))
	if err != nil {
		svr.SendError(clientAddr, errFileNotFound, err.Error())
		return err
	}

	// Read loop:
	var lastSentBufLen int
	for blockNumber := uint16(1); ; blockNumber++ {

		// Retrieve the next buffer of data from the file:
		var fileBuf []byte
		fileBuf, err = fileIter.Read()
		if err != nil {
			svr.SendError(clientAddr, errAccessViolation, err.Error())
			return err
		}

		// fileBuf==nil means that there is no more data to be sent ;
		// However, if the last packet we sent had a payload size equal to the
		// maximum payload size, then we send an extra empty packet to signify
		// the end of transmission.
		// Also: same logic applies for an empty file: we need to send at least
		// one data packet.
		if fileBuf == nil {
			if lastSentBufLen == int(svr.Conf.DataPayloadSize) || blockNumber == 1 {
				fileBuf = []byte{}
			} else {
				break
			}
		}

		// Send the data packet and handle its ACK and also other scenarios:
		dataPacket := PacketData{blockNumber, fileBuf}
		if err = lockStepSendData(sock, &dataPacket, clientAddr, svr.Conf.MaxSendTries,
			svr.Conf.socketTimeoutSecs); err != nil {
			return err
		}

		// we need to remember how big a payload we just sent:
		lastSentBufLen = len(fileBuf)
	}

	log.Println("Done: Sent file", req.Filename, "to", clientAddr)
	return
}

func lockStepSendData(sock *net.UDPConn, dataPkt *PacketData, clientAddr *net.UDPAddr,
	MaxSendTries uint, socketTimeoutSecs uint) error {

	writePacketBuf := dataPkt.Serialize()

	//  Try loop
	for triesLeft := MaxSendTries; triesLeft >= 0; triesLeft-- {
		if triesLeft == 0 {
			return fmt.Errorf(
				"no response from client after sending data block#%v %v times",
				dataPkt.BlockNum, MaxSendTries)
		}

		// Send the packet:
		if n, e := writeBuf(sock, writePacketBuf, socketTimeoutSecs); e != nil {
			return fmt.Errorf("writing to client: %w", e) // fail on write error.
		} else {
			if n == 0 {
				continue // Timed out. try again
			}
			log.Printf("[%v] Sent data block#%v to %v (%vB)\n", sock.LocalAddr(), dataPkt.BlockNum, clientAddr, n)
		}

		// Read the ack:
		var readPacketBuf = make([]byte, MaxPacketSize)
		if responsePkt, _, e := readPacket(sock, readPacketBuf, socketTimeoutSecs); e != nil {
			return e // fail on read error
		} else {
			if responsePkt == nil {
				continue // Timed out. try again
			} else {
				resend, err := processAckPacket(dataPkt, responsePkt, clientAddr)
				if err != nil {
					return err // invalid response
				}
				if resend {
					continue // client needs a resend
				}
				log.Printf("[%v] Received ACK#%v from %v\n", sock.LocalAddr(),
					dataPkt.BlockNum, clientAddr)
				return nil // success
			}
		}
	}
	return nil // unreachable.
}

func processAckPacket(dataPacket *PacketData, responsePkt *Packet,
	clientAddr *net.UDPAddr) (resend bool, err error) {
	switch (*responsePkt).(type) {
	case *PacketAck:
		ackPkt := (*responsePkt).(*PacketAck)
		switch ackPkt.BlockNum {
		case dataPacket.BlockNum:
			return false, nil
		case dataPacket.BlockNum - 1:
			return true, nil
		default:
			return false, fmt.Errorf("invalid ACK from client: "+
				"current block is #%v,client asked for #%v",
				dataPacket.BlockNum, ackPkt.BlockNum)
		}
	default:
		return false, fmt.Errorf(
			"received non ACK after sending data block #%v: %v",
			dataPacket.BlockNum, responsePkt)
	}
}
