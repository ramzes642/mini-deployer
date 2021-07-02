package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"
)

var listen = flag.String("listen", ":7654", "addr port")
var config = flag.String("config", "/etc/deployer.json", "config file location")

type configFile struct {
	Cert              string            `json:"cert"`
	Key               string            `json:"key"`
	Log               string            `json:"log"`
	Commands          map[string]string `json:"commands"`
	Whitelist         []string          `json:"whitelist"`
	Timeout           int64             `json:"timeout"`
	DisableAutoreload bool              `json:"disable_autoreload"`
	GitlabToken       string            `json:"gitlab_token"`
}

var Cfg configFile

func main() {
	log.SetFlags(log.LstdFlags)
	flag.Parse()

	err := ReadConfig()
	if err != nil {
		log.Fatalf("config read err: %s", err)
	}
	RunHttp()
}

func getMtime(filename string) time.Time {
	if filename == "" {
		return time.Time{}
	}
	lastChange, e := os.Stat(filename)
	if e != nil {
		log.Printf("cannot stat config file")
		return time.Time{}
	}
	lastChange.ModTime()
	return lastChange.ModTime()
}

func autoReload(ctx context.Context, srv *http.Server) {
	tm := time.NewTicker(time.Second)
	lastChangeConfig := getMtime(*config)
	lastChangeCert := getMtime(Cfg.Cert)
	for {
		select {
		case <-ctx.Done():
			return
		case <-tm.C:
			if lastChangeConfig != getMtime(*config) {
				lastChangeConfig = getMtime(*config)
				log.Printf("Detected config change")
				e := ReadConfig()
				if e != nil {
					log.Printf("config reload error: %s", e)
					continue
				} else {
					srv.Shutdown(context.Background())
				}
				return
			}
			if lastChangeCert != getMtime(Cfg.Cert) {
				lastChangeCert = getMtime(Cfg.Cert)
				log.Printf("Detected certificate change")
				srv.Shutdown(context.Background())
				return
			}
		}
	}

}

var cancelAutoreload context.CancelFunc

func ReadConfig() error {
	cfg, err := ioutil.ReadFile(*config)
	if err != nil {
		return fmt.Errorf("could not open config file: %s", err)
	}
	err = json.Unmarshal(cfg, &Cfg)
	if err != nil {
		return fmt.Errorf("could not unmarshal config file: %s", err)
	}
	if Cfg.Timeout == 0 {
		Cfg.Timeout = 10
	}

	if Cfg.Log != "" {
		f, err := os.OpenFile(Cfg.Log, os.O_APPEND|os.O_CREATE, 0666)
		if err != nil {
			log.Fatalf("could not open log file for writing")
		}
		log.SetOutput(f)
	}
	log.Printf("Config reloaded ok")
	return nil
}

func RunHttp() {
	for {
		port := strings.Split(*listen, ":")[1]
		fmt.Printf("Deployer started\n# curl http://localhost:%s/reload\t to manual reload\n\n", port)
		mux := http.NewServeMux()

		srv := &http.Server{Addr: *listen, Handler: logRequest(mux), ErrorLog: log.Default()}
		registerHandlers(srv, mux)

		if Cfg.Key != "" && Cfg.Cert != "" {
			srv.ListenAndServeTLS(Cfg.Cert, Cfg.Key)
		} else {
			srv.ListenAndServe()
		}
		time.Sleep(time.Second)
		log.Printf("Http restart")
	}

}

func registerHandlers(srv *http.Server, mux *http.ServeMux) {
	for addr, cmd := range Cfg.Commands {
		cmd := cmd
		addr := addr
		mux.HandleFunc("/"+addr, func(writer http.ResponseWriter, request *http.Request) {
			if !checkWhitelist(request.RemoteAddr, request) {
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

			err := c.Start()
			if err != nil {
				writer.WriteHeader(500)
				wr.Write([]byte(fmt.Sprintf("run err: %s", err)))
			}
			done := make(chan error)
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
					wr.Write([]byte("Command done ok"))
				}
			}

		})
	}
	mux.HandleFunc("/reload", func(writer http.ResponseWriter, request *http.Request) {
		if !checkWhitelist(request.RemoteAddr, request) {
			log.Printf("Access to reload for ip %s is forbidden", request.RemoteAddr)
			writer.WriteHeader(403)
			writer.Write([]byte("Forbidden"))
			return
		}

		err := ReadConfig()
		if err != nil {
			writer.Write([]byte(fmt.Sprintf("reload err: %s", err)))
		}
		go srv.Shutdown(context.Background())
	})

	if !Cfg.DisableAutoreload {
		if cancelAutoreload != nil {
			cancelAutoreload()
		}
		var ctx context.Context
		ctx, cancelAutoreload = context.WithCancel(context.Background())
		go autoReload(ctx, srv)
	}
}

func checkWhitelist(addr string, req *http.Request) bool {
	if Cfg.GitlabToken != "" && req.Header.Get("X-Gitlab-Token") == Cfg.GitlabToken {
		return true
	}

	addrParts := strings.Split(addr, ":")
	addr = strings.Join(addrParts[0:len(addrParts)-1], ":")
	if addr[0] == '[' {
		addr = addr[1 : len(addr)-1]
	}
	reqIp := net.ParseIP(addr)
	for _, ip := range Cfg.Whitelist {
		_, ipnetA, e := net.ParseCIDR(ip)
		if e == nil && ipnetA.Contains(reqIp) {
			return true
		} else if ip == addr {
			return true
		}
	}
	return false
}

type LogWriter struct {
	prefix string
	w      http.ResponseWriter
}

func NewLogWriter(prefix string, w http.ResponseWriter) *LogWriter {
	lw := &LogWriter{prefix: prefix, w: w}
	return lw
}

func (lw LogWriter) Write(p []byte) (n int, err error) {
	log.Printf("%s %s", lw.prefix, p)
	lw.w.Write(p)
	return len(p), nil
}

func logRequest(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("%s %s %s\n", r.RemoteAddr, r.Method, r.URL)
		handler.ServeHTTP(w, r)
	})
}
