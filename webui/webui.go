package webui

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"path/filepath"
	"sync"

	gxc "git.fractalqb.de/fractalqb/goxic"
	"git.fractalqb.de/fractalqb/qbsllm"
	"github.com/CmdrVasquess/BCplus/cmdr"

	"github.com/CmdrVasquess/BCplus/common"
	"github.com/CmdrVasquess/BCplus/galaxy"
)

type UIUpdate = uint32

func Update(uiu UIUpdate, ui UIUpdate) bool {
	if uiu&ui != ui {
		return false
	}
	return currentTopic == ui
}

const (
	UIReload UIUpdate = (1 << iota)
	UIHdr
	UISysPop
	UISysNat
	UIShips
	UISynth
	UISurface
	UIMissions
	UITravel
)

var currentTopic uint32

const (
	certFile = "webui.cert"
	keyFile  = "webui.key"
)

var (
	log       = qbsllm.New(qbsllm.Lnormal, "bc+wui", nil, nil)
	LogConfig = qbsllm.Config(log)
	pgOffline []byte
)

type gxtTopic struct {
	*gxc.Template
	HeaderData []int
	TopicData  []int
}

var (
	theGalaxy    *galaxy.Repo
	theCmdr      func() *cmdr.State
	theStateLock *sync.RWMutex
	theBCpQ      chan<- common.BCpEvent
	nmap         *common.NameMaps
	sysResolve   chan<- common.SysResolve
	theTheme     string
)

type Init struct {
	DataDir     string
	ResourceDir string
	CommonName  string
	Lang        string
	Port        uint
	BCpVersion  string
	Galaxy      *galaxy.Repo
	CmdrGetter  func() *cmdr.State
	StateLock   *sync.RWMutex
	BCpQ        chan<- common.BCpEvent
	Names       *common.NameMaps
	SysResolve  chan<- common.SysResolve
	Theme       string
}

func (i *Init) configure() {
	theGalaxy = i.Galaxy
	theCmdr = i.CmdrGetter
	theStateLock = i.StateLock
	theBCpQ = i.BCpQ
	nmap = i.Names
	sysResolve = i.SysResolve
	theTheme = i.Theme
}

func offlineFilter(h func(http.ResponseWriter, *http.Request)) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		if theCmdr() == nil {
			w.Write(pgOffline)
		} else {
			theStateLock.RLock()
			defer theStateLock.RUnlock()
			h(w, r)
		}
	}
}

func bindTpcHeader(bt *gxc.BounT, gxt *gxtTopic) {
	var hdr Header
	newHeader(&hdr)
	bt.BindGen(gxt.HeaderData, func(wr io.Writer) int {
		enc := json.NewEncoder(wr)
		err := enc.Encode(hdr)
		if err != nil {
			panic(err)
		}
		return 1
	})
}

type topic struct {
	key  string
	path string
	gxt  *gxtTopic
	hdlr func(http.ResponseWriter, *http.Request)
	nav  string
}

var topics = []*topic{
	&topic{key: tkeyShips, path: "/ships", gxt: &gxtShips, hdlr: tpcShips},
	//&topic{key: tkeySysPop, path: "/syspop", gxt: &gxtSysPop, hdlr: tpcSysPop},
	&topic{key: tkeyMissions, path: "/missions", gxt: &gxtMissions, hdlr: tpcMissions},
	&topic{key: tkeySurface, path: "/surface", gxt: &gxtSurface, hdlr: tpcSurface},
	&topic{key: tkeySysNat, path: "/sysnat", gxt: &gxtSysNat, hdlr: tpcSysNat},
	&topic{key: tkeySynth, path: "/synth", gxt: &gxtSynth, hdlr: tpcSynth},
	&topic{key: tkeyTravel, path: "/travel", gxt: &gxtTravel, hdlr: tpcTravel},
}

func getTopic(key string) *topic {
	for _, t := range topics {
		if t.key == key {
			return t
		}
	}
	panic(fmt.Errorf("requested unknown topic '%s'", key))
}

func Run(init *Init) chan<- interface{} {
	log.Infos("Initialize Web UI…")
	init.configure()
	loadTemplates(init.ResourceDir, init.Lang, init.BCpVersion)
	err := mustTLSCert(init.DataDir, init.CommonName)
	if err != nil {
		log.Panica("`err`", err)
	}
	htStatic := http.FileServer(http.Dir(filepath.Join(init.ResourceDir, "s")))
	http.Handle("/s/", http.StripPrefix("/s", htStatic))
	http.HandleFunc("/ws", serveWs)
	//	http.HandleFunc("/", func(wr http.ResponseWriter, rq *http.Request) {
	//		wr.Header().Set("Content-Type", "text/html; charset=utf-8")
	//		wr.Write(pgOffline) // TODO error
	//	})
	http.HandleFunc("/", offlineFilter(topics[0].hdlr))
	for _, tdef := range topics {
		http.HandleFunc(tdef.path, offlineFilter(tdef.hdlr))
	}
	go wscHub()
	go http.ListenAndServeTLS(fmt.Sprintf(":%d", init.Port),
		filepath.Join(init.DataDir, certFile),
		filepath.Join(init.DataDir, keyFile),
		nil)
	//go http.ListenAndServe(fmt.Sprintf(":%d", init.Port), nil)
	addls, err := ownAddrs()
	if err != nil {
		log.Panica("`err`", err)
	} else {
		log.Infos("Local Web UI address:")
		log.Infof("\thttps://localhost:%d/", init.Port)
		log.Infos("This host's addresses to connect to Web UI from remote:")
		for _, addr := range addls {
			log.Infof("\thttps://%s:%d/", addr, init.Port)
		}
	}
	return wscSendTo
}

func ownAddrs() (res []string, err error) {
	ifaddrs, err := net.InterfaceAddrs()
	if err != nil {
		return nil, err
	}
	for _, addr := range ifaddrs {
		if nip, ok := addr.(*net.IPNet); ok {
			if nip.IP.IsLoopback() {
				continue
			}
			if ip := nip.IP.To4(); ip != nil {
				res = append(res, nip.IP.String())
			} else if ip := nip.IP.To16(); ip != nil {
				res = append(res, fmt.Sprintf("[%s]", nip.IP.String()))
			}
		}
	}
	return res, err
}
