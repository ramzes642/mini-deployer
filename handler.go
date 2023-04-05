package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"time"
)

func runHandler(cmd, addr string) func(writer http.ResponseWriter, request *http.Request) {
	return func(writer http.ResponseWriter, request *http.Request) {
		var postBody []byte
		var e error

		if err := request.ParseForm(); err != nil {
			log.Printf("form parse error: %s", err)
			return
		}

		if request.Method == http.MethodPost && request.ContentLength != 0 {
			postBody, e = io.ReadAll(request.Body)
			if e != nil {
				log.Printf("post read error")
				return
			}
		}

		if !checkWhitelist(request.RemoteAddr, request, postBody) {
			log.Printf("Access to %s for ip %s is forbidden", addr, request.RemoteAddr)
			writer.WriteHeader(403)
			writer.Write([]byte("Forbidden to " + request.RemoteAddr))
			return
		}

		c := exec.Command("sh", "-c", cmd)
		c.Env = os.Environ()
		wr := NewLogWriter("["+addr+"]", writer)
		c.Stdout = wr
		c.Stderr = wr

		if len(postBody) > 0 {
			c.Stdin = bytes.NewReader(postBody)
		}

		if request.Method == http.MethodPost {
			for key, value := range request.PostForm {
				c.Env = append(c.Env, fmt.Sprintf("POST_%s=%s", key, value[0]))
			}
		}

		err := c.Start()
		if err != nil {
			writer.WriteHeader(500)
			wr.Write([]byte(fmt.Sprintf("run err: %s", err)))
		}
		done := make(chan error, 1)
		go func() { done <- c.Wait() }()

		// Start a timer
		timeout := time.After(time.Duration(Cfg.Timeout) * time.Second)
		select {
		case <-timeout:
			// Timeout happened first, kill the process and print a message.
			c.Process.Kill()
			writer.WriteHeader(500)
			wr.Write([]byte(fmt.Sprintf("Command timed out")))
		case err := <-done:
			if err != nil {
				writer.WriteHeader(500)
				wr.Write([]byte(fmt.Sprintf("run err: %s", err)))
			} else if c.ProcessState.ExitCode() != 0 {
				writer.WriteHeader(500)
				wr.Write([]byte(fmt.Sprintf("run err code: %d", c.ProcessState.ExitCode())))
			} else {
				//wr.Write([]byte("Command done ok"))
			}
		}
	}
}

func reloadHandler(srv *http.Server) func(writer http.ResponseWriter, request *http.Request) {
	return func(writer http.ResponseWriter, request *http.Request) {
		if !checkWhitelist(request.RemoteAddr, request, nil) {
			log.Printf("Access to reload for ip %s is forbidden", request.RemoteAddr)
			writer.WriteHeader(403)
			writer.Write([]byte("Forbidden"))
			return
		}

		err := ReadConfig()
		if err != nil {
			writer.Write([]byte(fmt.Sprintf("reload err: %s", err)))
			return
		}
		writer.Write([]byte(fmt.Sprintf("reload ok")))
		time.AfterFunc(time.Second, func() {
			srv.Shutdown(context.Background())
		})
	}
}
