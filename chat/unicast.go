package chat

import (
	"net"
	"time"
)

type UnicastMessage struct {
	Time time.Time `json:"time"`
	Msg  string    `json:"msg"`
}

func StartUnicastListener(listener net.Listener, onMessage func([]byte)) (net.Listener, error) {
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return // listener closed or error
			}

			go func(c net.Conn) {
				defer c.Close()
				buf := make([]byte, 4096)
				n, err := c.Read(buf)
				if err != nil {
					return
				}
				onMessage(buf[:n])
			}(conn)
		}
	}()

	return listener, nil
}
