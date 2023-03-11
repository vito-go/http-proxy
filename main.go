package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/url"
	"strings"
)

func main() {
	port := flag.Int64("p", 9444, "specify the port to listen")
	flag.Parse()
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	listen, err := net.Listen("tcp", fmt.Sprintf(":%d", *port))
	if err != nil {
		panic(err)
	}
	log.Printf("http proxy server start. address: [:%d]\n", *port)
	for i := 0; true; i++ {
		client, err := listen.Accept()
		if err != nil {
			panic(err)
		}
		go handle(client, i)
	}
}

func handle(client net.Conn, seq int) {
	defer client.Close()
	const count = 1 << 10 // 1kb is sufficient to read from the target address
	var b [count]byte
	n, err := client.Read(b[:])
	if err != nil {
		return
	}

	var method, host string
	bIdx := bytes.IndexByte(b[:], '\n')
	if bIdx < 0 {
		return
	}
	if _, err = fmt.Sscanf(string(b[:bIdx]), "%s%s", &method, &host); err != nil {
		return
	}
	address := host
	if strings.HasPrefix(host, "http") {
		hostPortURL, err := url.Parse(host)
		if err != nil {
			return
		}
		if strings.Contains(hostPortURL.Host, ":") {
			address = hostPortURL.Host
		} else {
			if hostPortURL.Opaque == "443" {
				address = hostPortURL.Scheme + ":443"
			} else {
				address = hostPortURL.Host + ":80"
			}
		}
	}
	log.Printf("seq: %-8d ==> new request. addr: %s, method: %s, target: %s \n",
		seq, client.RemoteAddr(), method, address)
	server, err := net.Dial("tcp", address)
	if err != nil {
		return
	}
	defer server.Close()
	if method == "CONNECT" || strings.HasSuffix(address, "443") {
		if _, err = fmt.Fprint(client, "HTTP/1.1 200 Connection established\r\n\r\n"); err != nil {
			return
		}
	} else {
		if _, err = server.Write(b[:n]); err != nil {
			return
		}
	}
	go io.Copy(client, server)
	io.Copy(server, client)
	log.Printf("seq: %-8d <== request done. addr: %s, method: %s, target: %s \n",
		seq, client.RemoteAddr(), method, address)
}
