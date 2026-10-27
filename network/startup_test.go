package network

import "testing"

func TestInvalidListenAddressPanicsSynchronously(t *testing.T) {
	for _, test := range []struct {
		name  string
		start func()
	}{
		{"TCP", func() { (&TCPServer{Addr: "127.0.0.1:invalid"}).Start() }},
		{"WebSocket", func() { (&WSServer{Addr: "127.0.0.1:invalid"}).Start() }},
	} {
		t.Run(test.name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Fatal("listen failure did not abort startup")
				}
			}()
			test.start()
		})
	}
}
