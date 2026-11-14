package main

import (
	"golearn/rpc/models"
	"log"
	"net/rpc"
)

func Run() error {
	// 1. 建立tcp连接
	conn, err := rpc.Dial("tcp", "127.0.0.1:7999")
	if err != nil {
		return err
	}
	defer conn.Close()

	// 2.准备请求参数
	args := models.CaculateArgs{
		A: 20,
		B: 30,
	}

	// 2.1 调用加法
	var reply models.CaculateReply
	err = conn.Call(
		"Caculator.Add",
		args,
		&reply,
	)
	if err != nil {
		return err
	}
	log.Println("Add result:", reply.Result)

	// 2.2 调用乘法
	err = conn.Call(
		"Caculator.Multiply",
		args,
		&reply,
	)
	if err != nil {
		return err
	}
	log.Println("Multiply result:", reply.Result)
	return nil
}

func main() {
	if err := Run(); err != nil {
		log.Fatal("Run failed, err:", err)
	}
}
