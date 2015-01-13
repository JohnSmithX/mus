//this is a shadowsocks server
package manager

import (
	"net"
	"sync"
	"fmt"
	"strings"
	"encoding/json"
	ss "github.com/shadowsocks/shadowsocks-go/shadowsocks"
)


type ComChan chan int

//command for loop
const (
	NULL int = iota
	STOP
)


type Server struct {
	mu sync.Mutex

	port          string
	method        string
	password      string
	limit         int64
	timeout       int64
	current       int64       	//current flow size
	listener      net.Listener
	comChan       ComChan          	//command channel
	local		  map[string]*local //1 to 1 : remote addr -> local
	format        string
	started       bool
	cipher        *ss.Cipher
}


func NewServer(port, method, password string, limit, timeout int64) (server *Server,err error) {
	if port == "" {
		err = newError("Cannot create a server without port")
		return
	}

	server = &Server{
		port: port,
		method: method,
		password: password,
		timeout: timeout,
		limit: limit,
		current: 0,
	}

	err = server.initServer()
	return
}

func (self *Server) initServer() (err error) {
	errFormat := fmt.Sprintf(serverFormat, self.port)
	ln, err := net.Listen("tcp", ":" + self.port)
	if err != nil {
		err = newError(errFormat, "create listner error:", err)
		return
	}

	cipher, err := ss.NewCipher(self.method, self.password)
	if err != nil {
		err = newError(errFormat, "create cipher error:", err)
		return
	}

	self.format = errFormat
	self.listener = ln
	self.cipher = cipher
	self.comChan = make(ComChan)
	self.local =  make(map[string]*local)
	self.started = false
	return
}

func (self *Server) doWithLock(fn func()) {
	self.mu.Lock()
	defer self.mu.Unlock()
	fn()
}

func (self *Server) addLocal(conn net.Conn) (local *local, err error) {

	cipher := self.cipher.Copy()
	ssconn := ss.NewConn(conn, cipher)

	local, err = newLocal(self, ssconn)
	if err != nil {
		err = newError(self.format, "create local error:", err)
		return
	}

	ip := strings.Split(conn.RemoteAddr().String(), ":")[0]
	self.doWithLock(func() {
		self.local[ip] = local
	})

	return
}

func (self *Server) isStarted() bool {
	return self.started
}

func (self *Server) isOverFlow() bool {
	return self.current > self.limit
}

func (self *Server) destroy() (err error) {
	//first stop the loop
	//second close the chan
	//third close the listener

	self.Stop()
	close(self.comChan)
	if err := self.listener.Close(); err != nil {
		err = newError(self.format, "close with error:", err)
	}
	return
}
//record flow
func (self *Server) recFlow(flow int) (err error) {
	self.current += int64(flow)
	return
}

func (self *Server) listen() {
	self.started = true
	Log.Info("server at port: %s started", self.port)
	defer func() {
		self.started = false
		Log.Info("server at port: %s stoped", self.port)
	}()
loop:
	for {

		if self.isOverFlow() {
			break loop
		}

		select{
		case com := <- self.comChan:
			if com == STOP {
				break loop
			}
		default:
		}

		conn, err := self.listener.Accept()
		if err != nil {
			err = newError(self.format, "listener accpet error:", err)
			Debug(err)
			continue
		}
		go self.handleConnect(conn)

	}
	return
}

func (self *Server) handleConnect(conn net.Conn) (flow int, err error) {

	defer func () {
		Debug(err)
		conn.Close()
	}()

	local, err := self.addLocal(conn)

	if err != nil {
		return
	}

	flow, err = local.run()
	if err != nil {
		return
	}
	//record flow
	err = self.recFlow(flow)
	return
}

//interface
func (self *Server) JSON() string {
	data, _ := json.Marshal(self)
	return string(data)
}

func (self *Server) ReStart() (err error) {
	if self.isStarted() {
		err = self.Stop()
		if err != nil {
			return
		}
	}
	err = self.Start()
	return
}

func (self *Server) Start() (err error) {
	if self.isStarted() {
		err = newError(self.format, "run server error:", "has started")
		return
	}

	go func () {
		self.listen()
	}()
	return
}

func (self *Server) Stop() (err error) {
	if !self.isStarted() {
		err = newError(self.format, "run server error:", "has stopped")
		return
	}
	go func() {
		select {
		case self.comChan <- STOP:
		}
	}()
	return
}

func (self *Server) Logs() {}

func (self *Server) Flow() {}
