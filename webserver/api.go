package webserver

import (
	"encoding/json"
	"fmt"
	"log"
	"mime"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sipclientgo/global"
	"sipclientgo/sip"
	"sipclientgo/system"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

func StartWS(ipv4 string, htp int) {
	global.HttpTcpPort = htp
	global.ClientIPv4 = net.ParseIP(ipv4)

	triedAlready := false
tryAgain:
	err := system.TestListening(global.ClientIPv4, global.HttpTcpPort)
	if err != nil {
		if triedAlready {
			fmt.Println(err)
			os.Exit(2)
		}
		global.ClientIPv4 = system.GetLocalIPv4(true)
		triedAlready = true
		goto tryAgain
	}

	r := http.NewServeMux()
	ws := fmt.Sprintf("%s:%d", global.ClientIPv4.String(), global.HttpTcpPort)
	srv := &http.Server{Addr: ws, Handler: r, ReadTimeout: 5 * time.Second, WriteTimeout: 10 * time.Second, IdleTimeout: 15 * time.Second}

	r.HandleFunc("/api/v1/session", serveSession)
	r.HandleFunc("/api/v1/stats", serveStats)
	r.HandleFunc("/", webHandler)

	global.WtGrp.Add(1)
	atomic.AddInt32(&global.WtGrpC, 1)
	go func() {
		defer global.WtGrp.Done()
		defer atomic.AddInt32(&global.WtGrpC, -1)
		log.Fatal(srv.ListenAndServe())
	}()

	fmt.Print("Loading API Webserver...")
	fmt.Println("Success: HTTP", ws)

	loadDataLocally()
}

func webHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		if r.URL.Path == "/" {
			serveHome(w)
			return
		} else if r.URL.Path == "/portal" {
			servePortal(w, r)
			return
		} else if r.URL.Path == "/portalData" {
			servePortalData(w)
			return
		} else if strings.HasPrefix(r.URL.Path, "/portal/") {
			serveStaticFiles(w, r)
			return
		} else if r.URL.Path == "/ws" {
			handleWSConnection(w, r)
			return
		}
	case http.MethodPost:
		if r.URL.Path == "/portal" {
			servePortalPost(w, r)
			return
		} else if r.URL.Path == "/portalData" {
			handlePortalData(w, r)
			return
		}
	case http.MethodDelete:
		if r.URL.Path == "/portal" {
			servePortalDelete(w, r)
			return
		}
	case http.MethodPut:
		if r.URL.Path == "/register" {
			imsi := r.URL.Query().Get("imsi")
			if err := sip.UEs.DoRegister(imsi, false); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			w.WriteHeader(http.StatusOK)
			return
		} else if r.URL.Path == "/unregister" {
			imsi := r.URL.Query().Get("imsi")
			if err := sip.UEs.DoRegister(imsi, true); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			w.WriteHeader(http.StatusOK)
			return
		} else if r.URL.Path == "/call" {
			urvalues := r.URL.Query()
			imsi := urvalues.Get("imsi")
			cdpn := urvalues.Get("cdpn")
			if err := sip.UEs.DoCall(imsi, cdpn); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			w.WriteHeader(http.StatusOK)
			return
		}
	}
	http.Error(w, "Not Found Resource", http.StatusNotFound)
}

func serveHome(w http.ResponseWriter) {
	_, _ = w.Write(fmt.Appendf(nil, "<h1>%s API Webserver</h1>", global.B2BUAName))
}

func serveStaticFiles(w http.ResponseWriter, r *http.Request) {
	filePath := filepath.Join("webserver/", r.URL.Path)
	mimeType := mime.TypeByExtension(filepath.Ext(filePath))
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}
	w.Header().Set("Content-Type", mimeType)
	http.ServeFile(w, r, filePath)
}

func servePortal(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "webserver/portal/index.html")
}

func handlePortalData(w http.ResponseWriter, r *http.Request) {
	var pd portalData
	err := json.NewDecoder(r.Body).Decode(&pd)
	r.Body.Close()
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := loadData(&pd); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	saveDataLocally()

	w.WriteHeader(http.StatusOK)
}

func servePortalData(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(buildDataJson()); err != nil {
		http.Error(w, "Error encoding response", http.StatusInternalServerError)
		return
	}
}

func servePortalPost(w http.ResponseWriter, r *http.Request) {
	var ue sip.UserEquipment
	err := json.NewDecoder(r.Body).Decode(&ue)
	r.Body.Close()
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	err = sip.UEs.AddUE(&ue)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	saveDataLocally()

	w.Header().Set("Content-Type", "application/json")

	response := sip.UEs.GetUEs()

	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Error encoding response", http.StatusInternalServerError)
		return
	}
}

func servePortalDelete(w http.ResponseWriter, r *http.Request) {
	var imsilst []string
	err := json.NewDecoder(r.Body).Decode(&imsilst)
	r.Body.Close()
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	sip.UEs.DeleteUEs(imsilst...)

	saveDataLocally()

	// w.Header().Set("Content-Type", "application/json")

	// response := sip.UEs.GetUEs()

	// if err := json.NewEncoder(w).Encode(response); err != nil {
	// 	http.Error(w, "Error encoding response", http.StatusInternalServerError)
	// 	return
	// }
	w.WriteHeader(http.StatusOK)
}

func serveSession(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var lst []string
	for _, ue := range sip.UEs.GetUEs() {
		for _, ses := range ue.SesMap.Range() {
			lst = append(lst, ses.String())
		}
	}

	data := struct {
		Sessions []string
	}{Sessions: lst}

	response, _ := json.Marshal(data)
	_, err := w.Write(response)
	if err != nil {
		system.LogError(system.LTWebserver, err.Error())
	}
}

func serveStats(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	BToMB := func(b uint64) uint64 {
		return b / 1000 / 1000
	}

	data := struct {
		CPUCount        int
		GoRoutinesCount int
		Alloc           uint64
		System          uint64
		GCCycles        uint32
		WaitGroupLength int32
	}{CPUCount: runtime.NumCPU(),
		GoRoutinesCount: runtime.NumGoroutine(),
		Alloc:           BToMB(m.Alloc),
		System:          BToMB(m.Sys),
		GCCycles:        m.NumGC,
		WaitGroupLength: atomic.LoadInt32(&global.WtGrpC),
	}

	response, _ := json.Marshal(data)
	_, err := w.Write(response)
	if err != nil {
		system.LogError(system.LTWebserver, err.Error())
	}
}

type portalData struct {
	PcscfSocket string               `json:"pcscfSocket"`
	ImsDomain   string               `json:"imsDomain"`
	Clients     []*sip.UserEquipment `json:"clients"`
}

var savemu sync.Mutex

func saveDataLocally() {
	savemu.Lock()
	defer savemu.Unlock()

	data := buildDataJson()

	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		fmt.Println("Error marshalling data:", err)
		return
	}

	file, err := os.Create("data.json")
	if err != nil {
		fmt.Println("Error creating file:", err)
		return
	}
	defer file.Close()

	_, err = file.Write(jsonData)
	if err != nil {
		fmt.Println("Error writing to file:", err)
		return
	}
}

func loadDataLocally() {
	file, err := os.Open("data.json")
	if err != nil {
		return
	}
	defer file.Close()

	var pd portalData
	err = json.NewDecoder(file).Decode(&pd)
	if err != nil {
		fmt.Println("Error decoding JSON:", err)
		return
	}

	loadData(&pd)
}

func loadData(pd *portalData) error {
	udpaddr, err := system.BuildUDPAddrFromSocketString(pd.PcscfSocket)
	if err != nil {
		return err
	}

	global.PCSCFSocket = udpaddr
	global.ImsDomain = pd.ImsDomain

	if pd.Clients != nil {
		for _, ue := range pd.Clients {
			sip.UEs.AddUE(ue)
		}
	}
	return nil
}

func buildDataJson() portalData {
	var pcscfSocket string
	if global.PCSCFSocket != nil {
		pcscfSocket = global.PCSCFSocket.String()
	}

	data := portalData{PcscfSocket: pcscfSocket,
		ImsDomain: global.ImsDomain,
		Clients:   sip.UEs.GetUEs(),
	}

	return data
}

func handleWSConnection(w http.ResponseWriter, r *http.Request) {
	// Upgrader to upgrade HTTP connection to WebSocket
	var upgrader = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}

	// Upgrade initial GET request to a WebSocket
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		fmt.Println(err)
		return
	}
	// defer ws.Close()

	if global.WSServer != nil {
		global.WSServer.Close()
	}

	global.WSServer = ws

	global.WtGrp.Add(1)
	go listenToWS(global.WSServer)
}

func listenToWS(ws *websocket.Conn) {
	defer global.WtGrp.Done()
	defer ws.Close()
	// Listen for messages from the client
	for {
		var msg map[string]any
		err := ws.ReadJSON(&msg)
		if err != nil {
			fmt.Println("Error reading json.", err)
			break
		}
		fmt.Printf("Received: %v\n", msg)

		// Send the received message back to the client
		// err = ws.WriteJSON(msg)
		// if err != nil {
		// 	fmt.Println("Error writing json.", err)
		// 	break
		// }
	}
}
