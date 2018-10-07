package webui

import (
	"bytes"
	"encoding/json"

	"github.com/CmdrVasquess/BCplus/galaxy"
)

type HdrSysLoc struct {
	Name string
	Coos [3]float64
}

func (hloc *HdrSysLoc) MarshalJSON() ([]byte, error) {
	buf := bytes.NewBuffer(nil)
	enc := json.NewEncoder(buf)
	buf.WriteString(`{"Name":`)
	enc.Encode(hloc.Name)
	buf.WriteString(`,"Coos":`)
	if galaxy.V3dValid(hloc.Coos) {
		enc.Encode(hloc.Coos)
		buf.WriteString("}")
	} else {
		buf.WriteString("null}")
	}
	return buf.Bytes(), nil
}

type Ship struct {
	Id       int
	Type     string
	Ident    string
	Name     string
	BerthLoc int64
}

type Header struct {
	Cmdr   string
	Ship   Ship
	System HdrSysLoc
	Home   *HdrSysLoc
}

const WsUpdCmd = "update"

type WsCmdUpdate struct {
	WsCommand
	Hdr *Header     `json:",omitempty"`
	Tpc interface{} `json:",omitempty"`
}

func NewWsCmdUpdate(header bool, reuse *WsCmdUpdate) *WsCmdUpdate {
	if reuse == nil {
		reuse = new(WsCmdUpdate)
	}
	reuse.Cmd = WsUpdCmd
	if header {
		reuse.Hdr = newHeader(nil)
	}
	return reuse
}

func newHeader(reuse *Header) *Header {
	if reuse == nil {
		reuse = new(Header)
	}
	if cmdr := theCmdr(); cmdr == nil {
		reuse.Cmdr = "–"
		// TODO
	} else {
		reuse.Cmdr = cmdr.Name
		reuse.Ship.Id = cmdr.InShip
		if cmdr.InShip >= 0 {
			inShip := cmdr.Ships[cmdr.InShip]
			reuse.Ship.Type, _ = nmap.ShipType.Map(inShip.Type)
			reuse.Ship.Ident = inShip.Ident
			reuse.Ship.Name = inShip.Name
			reuse.Ship.BerthLoc = inShip.BerthLoc
		}
		ssys, _ := theGalaxy.GetSystem(cmdr.Loc.SysId)
		if ssys != nil {
			reuse.System.Name = ssys.Name
			reuse.System.Coos = ssys.Coos
		}
		if cmdr.Home.SysId > 0 {
			ssys, _ = theGalaxy.GetSystem(cmdr.Home.SysId)
			if ssys != nil {
				reuse.Home = &HdrSysLoc{
					Name: ssys.Name,
					Coos: ssys.Coos,
				}
			} else {
				reuse.Home = nil
			}
		} else {
			reuse.Home = nil
		}
	}
	return reuse
}
