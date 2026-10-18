package main

import (
	"embed"
	"encoding/json"
	"flag"
	"fmt"
	"html"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"time"
)

//go:embed web/*
var assets embed.FS

var version = "dev"

type wakeRequest struct {
	MAC  string `json:"mac"`
	IP   string `json:"ip"`
	Mask string `json:"mask"`
}

func magicPacket(mac string) ([]byte, error) {
	addr, err := net.ParseMAC(mac)
	if err != nil || len(addr) != 6 {
		return nil, fmt.Errorf("Enter a valid 6-byte MAC address")
	}
	packet := make([]byte, 102)
	for i := 0; i < 6; i++ {
		packet[i] = 0xff
	}
	for i := 6; i < len(packet); i += 6 {
		copy(packet[i:], addr)
	}
	return packet, nil
}

func destination(ipText, maskText string) (net.IP, error) {
	if ipText == "" {
		if maskText != "" {
			return nil, fmt.Errorf("An IP address is required when specifying a subnet mask")
		}
		return net.IPv4bcast, nil
	}
	ip := net.ParseIP(ipText).To4()
	if ip == nil {
		return nil, fmt.Errorf("Enter a valid IPv4 address")
	}
	if maskText == "" {
		return ip, nil
	}
	maskIP := net.ParseIP(maskText).To4()
	if maskIP == nil {
		return nil, fmt.Errorf("Enter a valid subnet mask")
	}
	mask := net.IPMask(maskIP)
	ones, bits := mask.Size()
	if bits != 32 || ones > 30 {
		return nil, fmt.Errorf("The subnet mask must be contiguous with a prefix length of 0-30")
	}
	broadcast := make(net.IP, 4)
	for i := range broadcast {
		broadcast[i] = ip[i] | ^mask[i]
	}
	return broadcast, nil
}

func sendPacket(packet []byte, ip net.IP) error {
	conn, err := net.DialUDP("udp4", nil, &net.UDPAddr{IP: ip, Port: 9})
	if err != nil {
		return err
	}
	defer conn.Close()
	if err := conn.SetWriteDeadline(time.Now().Add(3 * time.Second)); err != nil {
		return err
	}
	_, err = conn.Write(packet)
	return err
}

func wakeHandler(send func([]byte, net.IP) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		reply := func(status int, message string) {
			w.WriteHeader(status)
			_ = json.NewEncoder(w).Encode(map[string]string{"message": message})
		}
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", "POST")
			reply(http.StatusMethodNotAllowed, "Use a POST request")
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 1024)
		defer r.Body.Close()
		decoder := json.NewDecoder(r.Body)
		decoder.DisallowUnknownFields()
		var req wakeRequest
		if err := decoder.Decode(&req); err != nil {
			reply(400, "Invalid request format")
			return
		}
		if err := decoder.Decode(&struct{}{}); err != io.EOF {
			reply(400, "Invalid request format")
			return
		}
		packet, err := magicPacket(req.MAC)
		if err != nil {
			reply(400, err.Error())
			return
		}
		ip, err := destination(req.IP, req.Mask)
		if err != nil {
			reply(400, err.Error())
			return
		}
		if err := send(packet, ip); err != nil {
			log.Printf("Failed to send wake packet: %v", err)
			reply(500, "Failed to send packet. Check the server network and try again")
			return
		}
		reply(200, "Wake packet sent")
	}
}

func main() {
	listen := flag.String("listen", ":8080", "HTTP listen address")
	flag.Parse()
	mux := http.NewServeMux()
	mux.HandleFunc("/api/wake", wakeHandler(sendPacket))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		file, contentType := "web/index.html", "text/html; charset=utf-8"
		switch r.URL.Path {
		case "/":
		case "/favicon.svg":
			file, contentType = "web/favicon.svg", "image/svg+xml"
		case "/apple-touch-icon.png", "/apple-touch-icon-precomposed.png":
			file, contentType = "web/apple-touch-icon.png", "image/png"
		default:
			http.NotFound(w, r)
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			w.WriteHeader(405)
			return
		}
		content, _ := assets.ReadFile(file)
		if file == "web/index.html" {
			content = []byte(strings.ReplaceAll(string(content), "{{VERSION}}", html.EscapeString(version)))
		}
		w.Header().Set("Content-Type", contentType)
		if r.Method != http.MethodHead {
			_, _ = w.Write(content)
		}
	})
	server := &http.Server{Addr: *listen, Handler: mux, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 10 * time.Second, IdleTimeout: 60 * time.Second}
	log.Printf("Wake on LAN started, listening on %s", *listen)
	log.Fatal(server.ListenAndServe())
}
