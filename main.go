package main

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"time"
)

var listen = flag.String("listen", ":7654", "addr port")
var config = flag.String("config", "/etc/mini-deployer.json", "config file location")

type configFile struct {
	Cert              string            `json:"cert"`
	Key               string            `json:"key"`
	Log               string            `json:"log"`
	Commands          map[string]string `json:"commands"`
	Whitelist         []string          `json:"whitelist"`
	Timeout           int64             `json:"timeout"`
	DisableAutoreload bool              `json:"disable_autoreload"`
	GitlabToken       string            `json:"gitlab_token"`
	GithubSecret      string            `json:"github_secret"`
	JwtHmac           string            `json:"jwt_hmac"`
	JwtClaim          string            `json:"jwt_claim"`
	JwtClaimAny       []string          `json:"jwt_claim_any"`
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
	cfg, err := os.ReadFile(*config)
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
		fmt.Printf("Deployer started\n")
		if Cfg.DisableAutoreload {
			fmt.Printf("# curl http://localhost:%s/reload to manual reload\n", port)
		}
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
		mux.HandleFunc("/"+addr, runHandler(cmd, addr))
	}
	mux.HandleFunc("/reload", reloadHandler(srv))

	if !Cfg.DisableAutoreload {
		if cancelAutoreload != nil {
			cancelAutoreload()
		}
		var ctx context.Context
		ctx, cancelAutoreload = context.WithCancel(context.Background())
		go autoReload(ctx, srv)
	}
}

func checkGithubSig(secret string, header string, body []byte) bool {
	s := hmac.New(sha256.New, []byte(secret))
	s.Write(body)
	hash := "sha256=" + hex.EncodeToString(s.Sum(nil))
	return hash == header
}

func checkWhitelist(addr string, req *http.Request, postBody []byte) bool {
	if Cfg.GitlabToken != "" && req.Header.Get("X-Gitlab-Token") == Cfg.GitlabToken {
		return true
	}
	if Cfg.GithubSecret != "" && req.Header.Get("X-Hub-Signature-256") != "" && postBody != nil {
		return checkGithubSig(Cfg.GithubSecret, req.Header.Get("X-Hub-Signature-256"), postBody)
	}
	if Cfg.JwtHmac != "" && Cfg.JwtClaim != "" && req.Header.Get("Authorization") != "" {
		return checkJwt(req.Header.Get("Authorization"))
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

func checkJwt(auth string) bool {
	header := strings.SplitN(auth, " ", 2)
	if len(header) != 2 && header[0] != "Bearer" {
		log.Printf("Authorization header error")
		return false
	}
	var payload map[string]interface{}
	err := JwtUnmarshal(header[1], &payload, []byte(Cfg.JwtHmac))
	if err != nil {
		log.Printf("Jwt err %s", err)
		return false
	}
	if claim, ok := payload[Cfg.JwtClaim].(string); ok && claim != "" {
		for _, acceptable := range Cfg.JwtClaimAny {
			if claim == acceptable {
				return true
			}
		}
		log.Printf("Jwt forbidden %s", claim)
		return false
	} else {
		log.Printf("Jwt claim not found")
		return false
	}
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

func JwtUnmarshal(jwt string, v any, secret []byte) error {
	hasher := hmac.New(sha256.New, secret)

	parts := strings.SplitN(jwt, ".", 3)
	if len(parts) != 3 {
		return fmt.Errorf("jwt parts count mismatch")
	}

	var headerB64, payloadB64, sigB64 string = parts[0], parts[1], parts[2]
	signature, err := base64.RawURLEncoding.DecodeString(sigB64)
	if err != nil {
		return fmt.Errorf("jwt sig b64 unpack err")
	}

	hasher.Write([]byte(headerB64 + "." + payloadB64))
	signature2 := hasher.Sum(nil)
	if !hmac.Equal(signature, signature2[:hasher.Size()]) {
		return fmt.Errorf("jwt signature validation error")
	}

	payload, err := base64.RawURLEncoding.DecodeString(payloadB64)
	if err != nil {
		return fmt.Errorf("jwt payload b64 unpack err: %s", err)
	}

	err = json.Unmarshal(payload, &v)
	if err != nil {
		return fmt.Errorf("jwt payload json unpack err: %s", err)
	}

	return nil
}
