// bcplus project main.go
package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"sync"

	c "github.com/CmdrVasquess/BCplus/cmdr"
	gxy "github.com/CmdrVasquess/BCplus/galaxy"
	"github.com/fractalqb/namemap"
	l "github.com/fractalqb/qblog"
)

func init() {
	assetPathRoot = os.Args[0]
	assetPathRoot = filepath.Dir(filepath.Clean(assetPathRoot))
	assetPathRoot = filepath.Join(assetPathRoot, "bcplus.d")
	var err error
	if assetPathRoot, err = filepath.Abs(assetPathRoot); err != nil {
		panic(err)
	}
	glog.Logf(l.Info, "assets: %s", assetPathRoot)
	nmNavItem = namemap.MustLoad(assetPath("nm/navitems.xsx")).
		FromStd().
		To(false, "lang:").
		Verify("nav items", "std → lang:")
	nmRnkCombat = namemap.MustLoad(assetPath("nm/rnk_combat.xsx")).
		FromStd().
		To(false, "lang:").
		Verify("combat ranks", "std → lang:")
	nmRnkTrade = namemap.MustLoad(assetPath("nm/rnk_trade.xsx")).
		FromStd().
		To(false, "lang:").
		Verify("trade ranks", "std → lang:")
	nmRnkExplor = namemap.MustLoad(assetPath("nm/rnk_explore.xsx")).
		FromStd().
		To(false, "lang:").
		Verify("explorer ranks", "std → lang:")
	nmRnkCqc = namemap.MustLoad(assetPath("nm/rnk_cqc.xsx")).
		FromStd().
		To(false, "lang:").
		Verify("CQC ranks", "std → lang:")
	nmRnkFed = namemap.MustLoad(assetPath("nm/rnk_feds.xsx")).
		FromStd().
		To(false, "lang:").
		Verify("federation ranks", "std → lang:")
	nmRnkImp = namemap.MustLoad(assetPath("nm/rnk_imps.xsx")).
		FromStd().
		To(false, "lang:").
		Verify("imperial ranks", "std → lang:")
	nmShipType = namemap.MustLoad(assetPath("nm/shiptype.xsx")).
		FromStd().
		To(false, "lang:").
		Verify("ship types", "std → lang:")
	nmMatType = namemap.MustLoad(assetPath("nm/resctypes.xsx")).
		FromStd().
		To(false, "lang:").
		Verify("rescource types", "std → lang:")
	nmMats = namemap.MustLoad(assetPath("nm/materials.xsx")).
		FromStd().
		To(false, "lang:").
		Verify("materials", "std → lang:")
	nmMatsXRef = nmMats.Base().FromStd().To(false, "wikia")
	nmMatsId = nmMats.Base().FromStd().To(false, "id")
	nmMatsIdRev = nmMats.Base().From("id", false).To(true)
	nmBdyCats = namemap.MustLoad(assetPath("nm/body-cats.xsx")).
		FromStd().
		To(false, "lang:").
		Verify("materials", "std → lang:")
	nmSynthLvl = namemap.MustLoad(assetPath("nm/synthlevel.xsx")).
		FromStd().
		To(false, "short:").
		Verify("materials", "std → short:")
}

type bcEvent struct {
	source rune
	data   interface{}
}

var theStateLock = sync.RWMutex{}
var theGalaxy *gxy.Galaxy
var theGame = c.NewGmState()
var credsKey []byte
var eventq = make(chan bcEvent, 128)

var jrnlDir string
var dataDir string

var nmNavItem namemap.FromTo
var nmRnkCombat namemap.FromTo
var nmRnkTrade namemap.FromTo
var nmRnkExplor namemap.FromTo
var nmRnkCqc namemap.FromTo
var nmRnkFed namemap.FromTo
var nmRnkImp namemap.FromTo
var nmShipType namemap.FromTo
var nmMatType namemap.FromTo
var nmMats namemap.FromTo
var nmMatsXRef namemap.FromTo
var nmMatsId namemap.FromTo
var nmMatsIdRev namemap.FromTo
var nmBdyCats namemap.FromTo
var nmSynthLvl namemap.FromTo

//go:generate ./genversion.sh
func BCpDescribe(wr io.Writer) {
	fmt.Fprintf(wr, "CMDR Vasquess: BC+ v%d.%d.%d / %s (%s)",
		BCpMajor,
		BCpMinor,
		BCpBugfix,
		runtime.Version(),
		BCpDate)
}

func BCpDescStr() string {
	buf := bytes.NewBuffer(nil)
	BCpDescribe(buf)
	return buf.String()
}

func saveState(beta bool) {
	if theGame.Cmdr.Name == "" {
		glog.Logf(l.Info, "empty state, nothing to save")
	} else {
		var fnm string
		if beta {
			fnm = theGame.Cmdr.Name + "-beta.json"
		} else {
			fnm = theGame.Cmdr.Name + ".json"
		}
		fnm = filepath.Join(dataDir, fnm)
		tnm := fnm + "~"
		if w, err := os.Create(tnm); err == nil {
			defer w.Close()
			glog.Logf(l.Info, "save state to %s", fnm)
			err := theGame.Save(w)
			w.Close()
			if err != nil {
				glog.Log(l.Error, err)
			} else if err = os.Rename(tnm, fnm); err != nil {
				glog.Log(l.Error, err)
			}
		} else {
			glog.Logf(l.Error, "cannot save game status to '%s': %s", fnm, err)
		}
	}
	theGalaxy.Close()
}

func loadCreds(cmdrNm string) error {
	if theGame.Creds == nil {
		theGame.Creds = &c.CmdrCreds{}
	} else {
		theGame.Creds.Clear()
	}
	filenm := filepath.Join(dataDir, cmdrNm+".pgp")
	glog.Logf(l.Info, "load credentials from %s", filenm)
	if _, err := os.Stat(filenm); os.IsNotExist(err) {
		glog.Logf(l.Warn, "commander %s's credentials do not exists", cmdrNm)
		return nil
	}
	f, err := os.Open(filenm)
	if err != nil {
		return err
	}
	defer f.Close()
	err = theGame.Creds.Read(f, credsKey)
	if err != nil {
		glog.Logf(l.Warn, "failed to read credentials for %s: %s", cmdrNm, err)
		theGame.Creds = nil
	}
	return nil
}

func loadState(cmdrNm string, beta bool) bool {
	var fnm string
	if beta {
		fnm = fmt.Sprintf("%s-beta.json", cmdrNm)
		if _, err := os.Stat(fnm); os.IsNotExist(err) {
			fnm = fmt.Sprintf("%s.json", cmdrNm)
		}
	} else {
		fnm = fmt.Sprintf("%s.json", cmdrNm)
	}
	fnm = filepath.Join(dataDir, fnm)
	glog.Logf(l.Info, "load state from %s", fnm)
	if r, err := os.Open(fnm); os.IsNotExist(err) {
		return false
	} else if err == nil {
		defer r.Close()
		theGame.Load(r)
		if len(credsKey) > 0 {
		}
		return true
	} else {
		panic("load commander: " + err.Error())
	}
}

var assetPathRoot string

func assetPath(relPathSlash string) string {
	relPathSlash = filepath.FromSlash(relPathSlash)
	return filepath.Join(assetPathRoot, relPathSlash)
}

const (
	esrcJournal = 'j'
	esrcUsr     = 'u'
)

func eventLoop() {
	glog.Log(l.Info, "starting main event loop…")
	for e := range eventq {
		switch e.source {
		case esrcJournal:
			func() {
				defer func() {
					if r := recover(); r != nil {
						glog.Logf(l.Error, "journal event error: %s", r)
					}
				}()
				DispatchJournal(&theStateLock, theGame, e.data.([]byte))
			}()
		case esrcUsr:
			func() {
				defer func() {
					if r := recover(); r != nil {
						glog.Logf(l.Error, "user event error: %s", r)
					}
				}()
				DispatchUser(&theStateLock, theGame, e.data.(map[string]interface{}))
			}()
		}
	}
}

func main() {
	flag.StringVar(&dataDir, "d", defaultDataDir(),
		"Verzeichnis zum Speichern der BC+ Daten")
	flag.StringVar(&jrnlDir, "j", defaultJournalDir(),
		"Verzeichnis mit den E:D Journal Dateien")
	flag.UintVar(&webGuiPort, "p", 1337,
		"Portnummer für die Web-GUI")
	pun := flag.Bool("l", false, "Lade beim Start die letze Journal Datei")
	verbose := flag.Bool("v", false, "Ausführliches Logging")
	verybose := flag.Bool("vv", false, "Sehr ausführliches logging")
	flag.BoolVar(&acceptHistory, "hist", false, "Akzeptiere vergangene Ereignisse")
	loadCmdr := flag.String("cmdr", "", "Lade beim Start die Daten des Kommandanten")
	promptKey := flag.Bool("pmk", false, "prompt for credential master key")
	showHelp := flag.Bool("h", false, "Zeige Hilfe an")
	flag.Parse()
	if *showHelp {
		BCpDescribe(os.Stdout)
		fmt.Println()
		flag.Usage()
		os.Exit(0)
	}
	if *verybose {
		glog.SetLevel(l.Trace)
	} else if *verbose {
		glog.SetLevel(l.Debug)
	}
	glog.Logf(l.Info, "Bordcomputer+ (%d.%.d.%d %s) on: %s\n",
		BCpMajor, BCpMinor, BCpBugfix, BCpDate,
		runtime.GOOS)
	glog.Logf(l.Info, "data    : %s\n", dataDir)
	var err error
	if *promptKey {
		credsKey = c.PromptCredsKey()
	}
	if _, err = os.Stat(dataDir); os.IsNotExist(err) {
		err = os.MkdirAll(dataDir, 0777)
		if err != nil {
			glog.Fatal("cannot create data dir: %s", err.Error())
		}
	}
	glog.Logf(l.Info, "journals: %s\n", jrnlDir)
	theGalaxy, err = gxy.OpenGalaxy(
		filepath.Join(dataDir, "systems.json"),
		assetPath("data/"))
	if err != nil {
		glog.Fatal(err)
	}
	c.SetTheGalaxy(theGalaxy)
	if len(*loadCmdr) > 0 {
		loadState(*loadCmdr, false)
	}
	stopWatch := make(chan bool)
	go WatchJournal(stopWatch, *pun, jrnlDir, func(line []byte) {
		cpy := make([]byte, len(line))
		copy(cpy, line)
		eventq <- bcEvent{esrcJournal, cpy}
	})
	go eventLoop()
	runWebGui()
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, os.Interrupt)
	<-sigs
	stopWatch <- true
	glog.Log(l.Info, "BC+ interrupted")
	theStateLock.RLock()
	saveState(theGame.IsBeta)
	theStateLock.RUnlock()
	glog.Log(l.Info, "bye…")
}
