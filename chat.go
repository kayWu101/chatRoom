package main

import (
	"fmt"
	"net"
	"time"
)

//登录用户结构体
type Client struct {
	C    chan string
	Name string
	Addr string
}

//创建全局map，用来存放在线用户信息
var onlineMap map[string]Client

//创建全局消息channel,用来传递消息
var messageChan = make(chan string)

func writeMsgToClient(conn net.Conn, client Client) {
	//监听用户自带channel上是否有信息
	for msg := range client.C {
		conn.Write([]byte(msg + "\n"))
	}
}
func makeMsg(client Client, keyWord string) string {
	msg := "[" + client.Addr + "]" + " " + client.Name + ":" + keyWord
	return msg
}
func handlerConnect(conn net.Conn) {
	defer conn.Close()
	//用来标志是否退出的通道
	exitChan := make(chan bool)
	//标识用户是否活跃的通道，如果用户没有任何操作，规定时间过后将会被强制下线
	activeChan := make(chan bool)
	addr := conn.RemoteAddr().String()
	client := Client{make(chan string), addr, addr}
	//将连接到的客户端加入到在线用户map中
	onlineMap[addr] = client
	//创建一个给当前用户发送消息的go程
	go writeMsgToClient(conn, client)

	//发送用户上线信息到全局channel中
	messageChan <- makeMsg(client, "login")

	//创建一个go程，用来处理用户发送的信息
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := conn.Read(buf)
			if n == 0 {
				//当前用户退出时，往exitChan通道里写入数据
				exitChan <- true
				fmt.Printf("监测到%s用户退出", client.Addr)
				return
			}
			if err != nil {
				fmt.Println("conn.Read err:", err)
				return
			}
			msg := string(buf[:n-1])
			//当客户端输入who时，在当前客户端页面显示在线用户列表
			if msg == "who" && len(msg) == 3 {
				conn.Write([]byte("online user list:\n"))
				for addr, client := range onlineMap {
					userInfo := addr + ":" + client.Name + "\n"
					conn.Write([]byte(userInfo))
				}
			} else if len(msg) >= 8 && msg[:7] == "rename|" { //修改用户名
				client.Name = msg[7:]
				client.C <- "rename ok"
				//更新在线用户map
				onlineMap[client.Addr] = client
			} else if msg == "exit" && len(msg) == 4 { //退出聊天室
				exitChan <- true
			} else {
				messageChan <- makeMsg(client, msg) //广播消息
			}
			//增加当前用户活跃标识
			activeChan <- true
		}
	}()

	//监听是否退出
	for {
		select {
		case <-exitChan:
			close(client.C)                          //当外部go程return时，内部go程并没有停止，要手动关闭通道
			delete(onlineMap, client.Addr)           //从在线用户列表中删除当前用户
			messageChan <- makeMsg(client, "logout") //告知其他在线用户当前用户退出聊天室
			return                                   //结束当前进程
		case <-activeChan: //如果执行这个case，说明用户此刻活跃，即强制退出时间从新开始

		case <-time.After(time.Second * 5): //设置当前进程连续5秒不活跃的话，将会被强制退出
			delete(onlineMap, client.Addr)
			messageChan <- makeMsg(client, "logout by timeOut! ")
			return
		}
	}
}

func manager() {
	//初始化用户列表map
	onlineMap = make(map[string]Client)
	//监听全局消息管道messageChan,有数据，则向在线用户广播数据，没有则阻塞
	for {
		msg := <-messageChan
		for _, client := range onlineMap {
			client.C <- msg
		}
	}
}

//网络聊天室
func main() {
	//创建监听器
	listener, err := net.Listen("tcp", "127.0.0.1:8000")
	if err != nil {
		fmt.Println("net.Listen err:", err)
		return
	}
	defer listener.Close()

	//启动manager函数，用来管理用户map和message管道
	go manager()

	//定义一个循环一直读取客户端请求
	for {
		conn, err := listener.Accept()
		if err != nil {
			fmt.Println("listener.Accept err:", err)
			return
		}
		//启动携程，可同时处理多个连接
		go handlerConnect(conn)
	}
}
