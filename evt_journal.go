package main

import (
	"encoding/json"
	"errors"
	str "strings"
	"sync"
	"time"

	gxy "github.com/CmdrVasquess/BCplus/galaxy"
	l "github.com/fractalqb/qblog"
)

type event = map[string]interface{}

type journalHanlder func(*GmState, map[string]interface{}, time.Time)

var dispatch = map[string]journalHanlder{
	"Fileheader":        fileheader,
	"Loadout":           loadout,
	"Rank":              rank,
	"Location":          location,
	"Progress":          progress,
	"Materials":         materials,
	"FSDJump":           fsdjump,
	"Docked":            docked,
	"ShipyardBuy":       shipBuy,
	"ShipyardNew":       shipNew,
	"ShipyardSell":      shipSell,
	"ShipyardSwap":      shipSwap,
	"ShipyardTransfer":  shipXfer,
	"SupercruiseEntry":  scEntry,
	"Scan":              scan,
	"MaterialCollected": matCollect,
	"MaterialDiscarded": matDiscard,
	//	"MaterialDiscovered": matDiscover,
	"EngineerCraft": engyCraft,
	"Synthesis":     synthesis,
}

func init() {
	dispatch["LoadGame"] = loadGame
}

func eventTime(evt map[string]interface{}) (time.Time, error) {
	ets, ok := evt["timestamp"].(string)
	if !ok {
		return time.Time{}, errors.New("event without timestamp")
	}
	if t, err := time.Parse(time.RFC3339, ets); err != nil {
		return time.Time{}, err
	} else {
		return t, nil
	}
}

var acceptHistory = false

func DispatchJournal(lock *sync.RWMutex, state *GmState, event []byte) {
	if len(event) == 0 {
		ejlog.Logf(l.Warn, "empty journal event")
		return
	}
	var jsonEvt map[string]interface{}
	if err := json.Unmarshal(event, &jsonEvt); err != nil {
		ejlog.Logf(l.Warn, "cannot parse journal event: %s", err)
		ejlog.Logf(l.Error, "Event has %d byte:[%s]", len(event), string(event))
		return
	}
	evt, ok := jsonEvt["event"].(string)
	if !ok {
		ejlog.Logf(l.Warn, "cannot determine journal event from: %s", string(event))
		return
	}
	hdlr, ok := dispatch[evt]
	if ok {
		t, err := eventTime(jsonEvt)
		if err != nil {
			ejlog.Log(l.Error, err)
		}
		var cmdrSwitch = evt == "Fileheader" || evt == "LoadGame"
		if state.isOffline() && !cmdrSwitch {
			ejlog.Logf(l.Info, "retain event: %s @%s", evt, t)
			state.evtBacklog = append(state.evtBacklog, jsonEvt)
		} else if acceptHistory || !t.Before(time.Time(state.T)) {
			ejlog.Logf(l.Info, "process event: %s @%s", evt, t)
			lock.Lock()
			defer lock.Unlock()
			hdlr(state, jsonEvt, t)
			if !cmdrSwitch {
				state.T = Timestamp(t)
			}
			select {
			case wscSendTo <- true:
				ejlog.Log(l.Debug, "sent web-socket event")
			default:
				ejlog.Log(l.Debug, "no web-socket event sent – channel blocked")
			}
		} else {
			ejlog.Logf(lNotice, "historic event: %s < %s", t, time.Time(state.T))
		}

	} else if t, err := eventTime(jsonEvt); err == nil {
		ejlog.Logf(l.Debug, "no handler for event: %s (%s)", evt, t)
	} else {
		ejlog.Logf(l.Debug, "no handler for event: %s", evt)
	}
}

func attArray(e event, name string) ([]interface{}, bool) {
	v, ok := e[name].([]interface{})
	return v, ok
}

func attObj(e event, name string) (map[string]interface{}, bool) {
	v, ok := e[name].(map[string]interface{})
	return v, ok
}

func attStr(e event, name string) (string, bool) {
	v, ok := e[name].(string)
	return v, ok
}

func optStr(e event, name string, dflt string) string {
	res, ok := attStr(e, name)
	if !ok {
		res = dflt
	}
	return res
}

func updStr(e event, name string, dst *string) bool {
	if v, ok := attStr(e, name); ok {
		*dst = v
		return true
	} else {
		return false
	}
}

func setStr(e event, name string, dst *string) {
	if !updStr(e, name, dst) {
		ejlog.Fatalf("no attribute %s in event %s", name, e)
	}
}

func attBool(e event, name string) (bool, bool) {
	v, ok := e[name].(bool)
	return v, ok
}

func updBool(e event, name string, dst *bool) bool {
	if v, ok := attBool(e, name); ok {
		*dst = v
		return true
	} else {
		return false
	}
}

func attInt(e event, name string) (int, bool) {
	v, ok := e[name].(float64)
	return int(v), ok
}

func updInt(e event, name string, dst *int) bool {
	if v, ok := attInt(e, name); ok {
		*dst = v
		return true
	} else {
		return false
	}
}

func setInt(e event, name string, dst *int) {
	if !updInt(e, name, dst) {
		ejlog.Fatalf("no attribute %s in event %s", name, e)
	}
}

func attUint8(e event, name string) (uint8, bool) {
	v, ok := e[name].(float64)
	return uint8(v), ok
}

func updUint8(e event, name string, dst *uint8) bool {
	if v, ok := attUint8(e, name); ok {
		*dst = v
		return true
	} else {
		return false
	}
}

func setUint8(e event, name string, dst *uint8) {
	if !updUint8(e, name, dst) {
		ejlog.Fatalf("no attribute %s in event %s", name, e)
	}
}

func attInt16(e event, name string) (int16, bool) {
	v, ok := e[name].(float64)
	return int16(v), ok
}

func attUint16(e event, name string) (uint16, bool) {
	v, ok := e[name].(float64)
	return uint16(v), ok
}

func updUint16(e event, name string, dst *uint16) bool {
	if v, ok := attUint16(e, name); ok {
		*dst = v
		return true
	} else {
		return false
	}
}

func setUint16(e event, name string, dst *uint16) {
	if !updUint16(e, name, dst) {
		ejlog.Fatalf("no attribute %s in event %s", name, e)
	}
}

func attInt64(e event, name string) (int64, bool) {
	v, ok := e[name].(float64)
	return int64(v), ok
}

func updInt64(e event, name string, dst *int64) bool {
	if v, ok := attInt64(e, name); ok {
		*dst = v
		return true
	} else {
		return false
	}
}

func setInt64(e event, name string, dst *int64) {
	if !updInt64(e, name, dst) {
		ejlog.Fatalf("no attribute %s in event %s", name, e)
	}
}

func attF32(e event, name string) (float32, bool) {
	v, ok := e[name].(float64)
	return float32(v), ok
}

func fileheader(gstat *GmState, evt map[string]interface{}, t time.Time) {
	if !gstat.isOffline() {
		saveState()
	}
	gstat.clear()
	if gvers, ok := attStr(evt, "gameversion"); ok {
		gstat.IsBeta = str.Contains(str.ToLower(gvers), "beta")
	} else {
		gstat.IsBeta = true
		ejlog.Log(l.Warn, "fileheader without gameversion => assume beta")
	}
}

func loadout(gstat *GmState, evt map[string]interface{}, t time.Time) {
	cmdr := &gstat.Cmdr
	shipId, ok := attInt(evt, "ShipID")
	if !ok {
		ejlog.Log(l.Error, "ignore loadout without ship id")
		return
	}
	ship := cmdr.ShipById(shipId)
	if ship == nil {
		ship = &Ship{ID: shipId}
		cmdr.Ships = append(cmdr.Ships, ship)
	}
	setStr(evt, "Ship", &ship.Type)
	ship.Type = str.ToLower(ship.Type)
	setStr(evt, "ShipName", &ship.Name)
	setStr(evt, "ShipIdent", &ship.Ident)
	ship.Ident = str.ToUpper(ship.Ident)
}

func loadGame(gstat *GmState, evt map[string]interface{}, t time.Time) {
	cmdrNm, ok := attStr(evt, "Commander")
	if !gstat.isOffline() {
		saveState()
		if cmdrNm == gstat.Cmdr.Name {
			gstat.next1stJump = true
			return
		} else {
			ejlog.Logf(l.Error, "switched cmdrs in non-offline state: '%s' → '%s'",
				gstat.Cmdr.Name,
				cmdrNm)
			gstat.clear()
		}
	}
	eventBacklog := gstat.evtBacklog
	gstat.evtBacklog = nil
	if !ok {
		ejlog.Fatalf("load game without commander in %s", evt)
	}
	loadState(cmdrNm)
	gstat.Cmdr.Name = cmdrNm
	if eventBacklog != nil {
		blc := 0
		ejlog.Log(l.Info, "process event backlog…")
		for _, evt := range eventBacklog {
			enm, _ := attStr(evt, "event")
			hdlr, _ := dispatch[enm]
			t, _ := eventTime(evt)
			if acceptHistory || !t.Before(time.Time(gstat.T)) {
				hdlr(gstat, evt, t)
			}
			blc++
		}
		ejlog.Logf(l.Info, "%d events from backlog done!", blc)
	}
	cmdr := &gstat.Cmdr
	setInt64(evt, "Credits", &cmdr.Credits)
	setInt64(evt, "Loan", &cmdr.Loan)
	shipId := int(evt["ShipID"].(float64))
	cmdr.CurShip.Ship = cmdr.ShipById(shipId)
	if cmdr.CurShip.Ship == nil {
		ship := &Ship{
			ID:    shipId,
			Type:  str.ToLower(optStr(evt, "Ship", "")),
			Name:  optStr(evt, "ShipName", ""),
			Ident: optStr(evt, "ShipIdent", "")}
		cmdr.CurShip.Ship = ship
		cmdr.Ships = append(cmdr.Ships, ship)
	}
	cmdr.CurShip.Loc.Location = nil
}

func rank(gstat *GmState, evt map[string]interface{}, t time.Time) {
	rnks := &gstat.Cmdr.Ranks
	setUint8(evt, "Combat", &rnks[RnkCombat])
	setUint8(evt, "Trade", &rnks[RnkTrade])
	setUint8(evt, "Explore", &rnks[RnkExplore])
	setUint8(evt, "CQC", &rnks[RnkCqc])
	setUint8(evt, "Empire", &rnks[RnkImp])
	setUint8(evt, "Federation", &rnks[RnkFed])
}

func progress(gstat *GmState, evt map[string]interface{}, t time.Time) {
	prgs := &gstat.Cmdr.RnkPrgs
	setUint8(evt, "Combat", &prgs[RnkCombat])
	setUint8(evt, "Trade", &prgs[RnkTrade])
	setUint8(evt, "Explore", &prgs[RnkExplore])
	setUint8(evt, "CQC", &prgs[RnkCqc])
	setUint8(evt, "Empire", &prgs[RnkImp])
	setUint8(evt, "Federation", &prgs[RnkFed])
}

func materials(gstat *GmState, evt map[string]interface{}, t time.Time) {
	cmdr := &gstat.Cmdr
	cmdr.MatsRaw.clearHave()
	if mats, ok := attArray(evt, "Raw"); ok {
		for _, m := range mats {
			mat := m.(map[string]interface{})
			matNm, _ := attStr(mat, "Name")
			matNo, _ := attInt16(mat, "Count")
			cmdr.MatsRaw.SetHave(matNm, matNo)
		}
	}
	cmdr.MatsMan.clearHave()
	if mats, ok := attArray(evt, "Manufactured"); ok {
		for _, m := range mats {
			mat := m.(map[string]interface{})
			matNm, _ := attStr(mat, "Name")
			matNo, _ := attInt16(mat, "Count")
			cmdr.MatsMan.SetHave(matNm, matNo)
		}
	}
	cmdr.MatsEnc.clearHave()
	if mats, ok := attArray(evt, "Encoded"); ok {
		for _, m := range mats {
			mat := m.(map[string]interface{})
			matNm, _ := attStr(mat, "Name")
			matNo, _ := attInt16(mat, "Count")
			cmdr.MatsEnc.SetHave(matNm, matNo)
		}
	}
}

func fsdjump(gstat *GmState, evt map[string]interface{}, t time.Time) {
	spos := evt["StarPos"].([]interface{})
	snm, _ := attStr(evt, "StarSystem")
	snm = str.ToUpper(snm)
	ssys := theGalaxy.GetSystem(snm)
	ssys.Coos.Set(spos[0].(float64), spos[1].(float64), spos[2].(float64))
	gstat.addJump(ssys, Timestamp(t))
	gstat.next1stJump = false
	if lji := len(gstat.JumpHist) - 1; lji > 0 {
		lj := gstat.JumpHist[lji]
		if !lj.First {
			t0 := time.Time(gstat.JumpHist[lji-1].Arrive)
			t1 := time.Time(lj.Arrive)
			dt := t1.Sub(t0)
			if gstat.Tj2j == 0 || dt < gstat.Tj2j {
				gstat.Tj2j = dt
			}
		}
	}
	_, boost := evt["BoostUsed"]
	cmdr := &gstat.Cmdr
	cmdr.Loc.Location = ssys
	if ship := cmdr.CurShip.Ship; ship != nil {
		jd := float32(evt["JumpDist"].(float64))
		if boost {
			ship.Jump.BoostSum += jd
			ship.Jump.BoostCount++
		} else {
			ship.Jump.DistSum += jd
			ship.Jump.DistCount++
			if jd > ship.Jump.DistMax {
				ship.Jump.DistMax = jd
			}
		}
	}
}

func location(gstat *GmState, evt map[string]interface{}, t time.Time) {
	spos := evt["StarPos"].([]interface{})
	snm, _ := attStr(evt, "StarSystem")
	snm = str.ToUpper(snm)
	ssys := theGalaxy.GetSystem(snm)
	ssys.Coos.Set(spos[0].(float64), spos[1].(float64), spos[2].(float64))
	gstat.Cmdr.Loc.Location = ssys
	var body *gxy.SysBody
	if bodyNm, ok := attStr(evt, "Body"); ok {
		body = ssys.GetBody(bodyNm)
	}
	if docked, ok := attBool(evt, "Docked"); ok && docked {
		statNm, _ := attStr(evt, "StationName")
		statTy, _ := attStr(evt, "StationType")
		port := ssys.GetStation(statNm)
		port.Type = statTy
		if body != nil {
			port.SetBody(body)
		}
	} else if body == nil {
		gstat.Cmdr.Loc.Location = body
	} else {
		gstat.Cmdr.Loc.Location = ssys
	}
}

func docked(gstat *GmState, evt map[string]interface{}, t time.Time) {
	cmdr := &gstat.Cmdr
	snm, _ := attStr(evt, "StarSystem")
	snm = str.ToUpper(snm)
	ssys := theGalaxy.GetSystem(snm)
	portNm, _ := attStr(evt, "StationName")
	port := ssys.GetStation(portNm)
	if port != nil {
		cmdr.Loc.Location = port
	} else {
		cmdr.Loc.Location = ssys
	}
}

func shipXfer(gstat *GmState, evt map[string]interface{}, t time.Time) {
	cmdr := &gstat.Cmdr
	if cmdr.Loc.Location == nil {
		return
	}
	shId, _ := attInt(evt, "ShipID")
	if cmdr.CurShip.Ship != nil && cmdr.CurShip.ID == shId {
		ejlog.Log(l.Warn, "ship trxnfer for commanders current ship")
		return
	}
	ship := cmdr.ShipById(shId)
	if ship == nil {
		shTy, _ := attStr(evt, "ShipType")
		shTy = str.ToLower(shTy)
		ship = &Ship{ID: shId, Type: shTy}
	}
	ship.Loc = cmdr.Loc
}

func shipBuy(gstat *GmState, evt map[string]interface{}, t time.Time) {
	cmdr := &gstat.Cmdr
	mny, _ := attInt64(evt, "ShipPrice")
	cmdr.Credits -= mny
	mny, ok := attInt64(evt, "SellPrice")
	if ok {
		cmdr.Credits += mny
		if oldId, ok := attInt(evt, "SellShipID"); ok {
			cmdr.SellShipId(oldId, Timestamp(t))
		}
	}
}

func shipNew(gstat *GmState, evt map[string]interface{}, t time.Time) {
	cmdr := &gstat.Cmdr
	shId, _ := attInt(evt, "ShipID")
	ship := cmdr.ShipById(shId)
	if ship == nil {
		ship = &Ship{ID: shId}
		cmdr.Ships = append(cmdr.Ships, ship)
		setStr(evt, "ShipType", &ship.Type)
		ship.Type = str.ToLower(ship.Type)
	}
	ship.Bought = (*Timestamp)(&t)
	if cmdr.CurShip.Ship != ship {
		if cmdr.CurShip.Ship != nil {
			cmdr.CurShip.Ship.Loc = cmdr.Loc
		}
		cmdr.CurShip.Ship = ship
		ship.Loc.Location = nil
	}
}

func shipSell(gstat *GmState, evt map[string]interface{}, t time.Time) {
	cmdr := &gstat.Cmdr
	shId, ok := attInt(evt, "SellShipID")
	if !ok {
		ejlog.Fatal("sell ship w/o id")
	}
	if mny, ok := attInt64(evt, "ShipPrice"); ok {
		cmdr.Credits += mny
	} else {
		ejlog.Log(l.Warn, "selling a ship without a price")
	}
	cmdr.SellShipId(shId, Timestamp(t))
}

func shipSwap(gstat *GmState, evt map[string]interface{}, t time.Time) {
	cmdr := &gstat.Cmdr
	oldId, ok := attInt(evt, "StoreShipID")
	if !ok {
		ejlog.Fatal("ship swap w/o id for old ship")
	}
	oldShip := cmdr.ShipById(oldId)
	if oldShip == nil {
		oldShip = &Ship{
			ID:   oldId,
			Type: str.ToLower(optStr(evt, "StoreOldShip", ""))}
		cmdr.Ships = append(cmdr.Ships, oldShip)
	}
	newId, ok := attInt(evt, "ShipID")
	if !ok {
		ejlog.Fatal("ship swap w/o id for new ship")
	}
	newShip := cmdr.ShipById(newId)
	if newShip == nil {
		newShip = &Ship{
			ID:   newId,
			Type: str.ToLower(optStr(evt, "ShipType", ""))}
		cmdr.Ships = append(cmdr.Ships, newShip)
	}
	cmdr.CurShip.Ship = newShip
	newShip.Loc.Location = nil
	oldShip.Loc = cmdr.Loc
}

func scEntry(gstat *GmState, evt map[string]interface{}, t time.Time) {
	cmdr := &gstat.Cmdr
	snm, _ := attStr(evt, "StarSystem")
	snm = str.ToUpper(snm)
	ssys := theGalaxy.GetSystem(snm)
	cmdr.Loc.Location = ssys
}

func stripSystemName(sysNm, bodyNm string) string {
	if str.HasPrefix(str.ToUpper(bodyNm), sysNm) {
		res := bodyNm[len(sysNm):]
		return str.TrimLeft(res, " ")
	} else {
		return bodyNm
	}
}

func scan(gstat *GmState, evt map[string]interface{}, t time.Time) {
	if gstat.Cmdr.Loc.Location == nil {
		ejlog.Log(lNotice, "scan event without known star-system")
		return
	}
	ssys := gstat.Cmdr.Loc.System()
	if ssys == nil {
		ejlog.Log(l.Error, "commander's location has no system")
		return
	}
	bdyNm, _ := attStr(evt, "BodyName")
	body := ssys.GetBody(stripSystemName(ssys.Name(), bdyNm))
	body.Dist, _ = attF32(evt, "DistanceFromArrivalLS")
	_, ok := attStr(evt, "StarType")
	if ok {
		body.Cat = gxy.Star
		body.Landable = false
	} else {
		body.Cat = gxy.Planet
		updBool(evt, "Landable", &body.Landable)
		if mats, ok := evt["Materials"].([]interface{}); ok {
			if body.Mats == nil {
				body.Mats = make(map[string]float32)
			}
			for _, val := range mats {
				mat := val.(map[string]interface{})
				nm, _ := attStr(mat, "Name")
				rh, _ := attF32(mat, "Percent")
				body.Mats[nm] = rh
			}
		}
	}
}

func sumMat(cmdr *Commander, cat, name string, d int16) {
	var mats CmdrsMats
	switch cat {
	case "Raw":
		mats = cmdr.MatsRaw
	case "Manufactured":
		mats = cmdr.MatsMan
	case "Encoded":
		mats = cmdr.MatsEnc
	}
	cmat, _ := mats[name]
	if cmat == nil {
		cmat = &Material{Have: d, Need: 0}
		mats[name] = cmat
	} else {
		cmat.Have += d
	}
}

func matCollect(gstat *GmState, evt map[string]interface{}, t time.Time) {
	matCat, _ := attStr(evt, "Category")
	matNm, _ := attStr(evt, "Name")
	matNo, _ := attInt16(evt, "Count")
	sumMat(&gstat.Cmdr, matCat, matNm, matNo)
}

func matDiscard(gstat *GmState, evt map[string]interface{}, t time.Time) {
	matCat, _ := attStr(evt, "Category")
	matNm, _ := attStr(evt, "Name")
	matNo, _ := attInt16(evt, "Count")
	sumMat(&gstat.Cmdr, matCat, matNm, -matNo)
}

//func matDiscover(gstat *GmState, evt map[string]interface{}, t time.Time) {
//	matCat, _ := attStr(evt, "Category")
//	matNm, _ := attStr(evt, "Name")
//	discoNo, _ := attInt16(evt, "DiscoveryNumber")
//	// Do NOT sum discoNo! It's ~an ID
//}

func synthesis(gstat *GmState, evt map[string]interface{}, t time.Time) {
	cmdr := &gstat.Cmdr
	used := evt["Materials"].(map[string]interface{})
	for mat, jNo := range used {
		matNo := int16(jNo.(float64))
		switch theGalaxy.MatCategory(mat) {
		case gxy.Raw:
			sumMat(cmdr, "Raw", mat, -matNo)
		case gxy.Man:
			sumMat(cmdr, "Manufactured", mat, -matNo)
		case gxy.Enc:
			sumMat(cmdr, "Encoded", mat, -matNo)
		default:
			ejlog.Logf(l.Warn, "cannot categorize material '%s'", mat)
		}
	}
}

func engyCraft(gstat *GmState, evt map[string]interface{}, t time.Time) {
	cmdr := &gstat.Cmdr
	used := evt["Ingredients"].([]interface{})
	for _, i := range used {
		ingr := i.(map[string]interface{})
		mat, _ := attStr(ingr, "Name")
		matNo, _ := attInt16(ingr, "Count")
		switch theGalaxy.MatCategory(mat) {
		case gxy.Raw:
			sumMat(cmdr, "Raw", mat, -matNo)
		case gxy.Man:
			sumMat(cmdr, "Manufactured", mat, -matNo)
		case gxy.Enc:
			sumMat(cmdr, "Encoded", mat, -matNo)
		default:
			ejlog.Logf(l.Warn, "cannot categorize material '%s'", mat)
		}
	}
}
