package main

import (
	"golearn/rpc/models"
	"log"
	"net"
	"net/rpc"
)

type Caculator struct{}

func (c *Caculator) Add(
	args models.CaculateArgs,
	reply *models.CaculateReply,
) error {
	reply.Result = args.A + args.B
	return nil
}

func (c *Caculator) Multiply(
	args models.CaculateArgs,
	reply *models.CaculateReply,
) error {
	reply.Result = args.A * args.B
	return nil
}

func Start() error {
	// 1. create server
	svr := rpc.NewServer()

	// 2. register service
	caculator := new(Caculator)
	err := svr.Register(caculator)
	if err != nil {
		return err
	}

	// 3. Listener
	listener, err := net.Listen("tcp", "127.0.0.1:7999")
	if err != nil {
		return err
	}
	defer listener.Close()
	log.Println("RPC Server listening on 127.0.0.1:7999")

	// 4.循环accept
	for {
		conn, err := listener.Accept()
		if err != nil {
			return err
		}

		// BIO: 分配goroutine去处理
		go svr.ServeConn(conn)
	}
}

func main() {
	if err := Start(); err != nil {
		log.Fatal("Start failed, err:", err)
	}
}
