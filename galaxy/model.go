package galaxy

import (
	"database/sql"
	"math"
	"os"
	"strconv"
	"strings"

	l "git.fractalqb.de/fractalqb/qbsllm"
	_ "github.com/mattn/go-sqlite3"
)

var (
	log       = l.New(l.Lnormal, "bc+gxy", nil, nil)
	LogConfig = l.Config(log)
)

var NaN32 = float32(math.NaN())

func LocalName(system, planet string) string {
	if len(planet) <= len(system) {
		return planet
	}
	system = strings.ToLower(system)
	plo := strings.ToLower(planet)
	if strings.HasPrefix(plo, system) {
		planet = strings.TrimSpace(planet[len(system):])
	}
	return planet
}

type Repo struct {
	db *sql.DB
	tx *sql.Tx
	// cache grows unboundend: no concurrent use; size limited by gaming session
	chSysId  map[int64]*System
	chSysNm  map[string]*System
	chPartId map[int64]*SysPart
	chResId  map[int64]*Resource
}

const dbDriver = "sqlite3"

// Returns os.IsNotExists(err) == true if it is a new db
func NewRepo(dbFile string) (res *Repo, err error) {
	defer func() {
		if p := recover(); p != nil {
			err = p.(error)
		}
	}()
	if len(dbFile) == 0 {
		log.Fatals("galaxy repo created with empty dbConnect string")
	}
	log.Infoa("galaxy repo connecting to `DB`", dbFile)
	_, notExists := os.Stat(dbFile)
	if !os.IsNotExist(notExists) {
		notExists = nil
	}
	db, err := sql.Open(dbDriver, dbFile)
	if err != nil {
		return nil, err
	}
	res = &Repo{
		db:       db,
		chSysId:  make(map[int64]*System),
		chSysNm:  make(map[string]*System),
		chPartId: make(map[int64]*SysPart),
		chResId:  make(map[int64]*Resource),
	}
	return res, notExists
}

func (rpo *Repo) chSys(s *System) {
	rpo.chSysId[s.Id] = s
	rpo.chSysNm[s.Name] = s
}

func (rpo *Repo) chPart(p *SysPart) {
	rpo.chPartId[p.Id] = p
}

func (rpo *Repo) chRes(r *Resource) {
	rpo.chResId[r.Id] = r
}

func (rpo *Repo) ClearCache() {
	rpo.chSysId = make(map[int64]*System)
	rpo.chSysNm = make(map[string]*System)
	rpo.chPartId = make(map[int64]*SysPart)
	rpo.chResId = make(map[int64]*Resource)
}

func (rpo *Repo) Close() {
	if rpo.db != nil {
		err := rpo.db.Close()
		if err != nil {
			log.Errora("`err`", err)
		}
		rpo.db = nil
	}
}

func (rpo *Repo) Version() (v int, err error) {
	var vStr string
	err = rpo.db.QueryRow("select val from meta where key='version'").Scan(&vStr)
	if err != nil {
		return -1, err
	}
	v, err = strconv.Atoi(vStr)
	return v, err
}

func (rpo *Repo) XaBegin() *Xa {
	if rpo.tx == nil {
		var err error
		rpo.tx, err = rpo.db.Begin()
		if err != nil {
			panic(err)
		}
		return (*Xa)(rpo)
	} else {
		return nil
	}
}

type Xa Repo

func (xa *Xa) Commit() error {
	if xa != nil && xa.tx != nil {
		err := xa.tx.Commit()
		xa.tx = nil
		return err
	}
	return nil
}

func (xa *Xa) Rollback() error {
	if xa != nil && xa.tx != nil {
		err := xa.tx.Rollback()
		xa.tx = nil
		return err
	}
	return nil
}

type Entity struct {
	r  *Repo
	Id int64
}

type System struct {
	Entity
	Name   string
	Coos   Vec3D
	BodyNo int
	parts  []*SysPart
}

func (s *System) LocalName(name string) string {
	return LocalName(s.Name, name)
}

func (rpo *Repo) PutSystem(sys *System) (*System, error) {
	var err error
	xa := rpo.XaBegin()
	defer xa.Rollback()
	if sys.r == nil {
		var x, y, z sql.NullFloat64
		if V3dValid(sys.Coos) {
			x.Valid, x.Float64 = true, sys.Coos[Xk]
			y.Valid, y.Float64 = true, sys.Coos[Yk]
			z.Valid, z.Float64 = true, sys.Coos[Zk]
		}
		var res sql.Result
		res, err = rpo.tx.Exec(`insert into system (name, x, y, z, bno)
				                values ($1, $2, $3, $4, $5)`,
			sys.Name, x, y, z, sys.BodyNo)
		if err == nil {
			if sys.Id, err = res.LastInsertId(); err == nil {
				sys.r = rpo
				xa.Commit()
				rpo.chSys(sys)
			}
		}
	} else {
		_, err = rpo.tx.Exec(`update system
		                       set name=$1, x=$2, y=$3, z=$4, bno=$5
		                       where id=$6`,
			sys.Name,
			sys.Coos[Xk], sys.Coos[Yk], sys.Coos[Zk],
			sys.BodyNo,
			sys.Id)
		if err == nil {
			xa.Commit()
			rpo.chSys(sys)
		}
	}
	return sys, err
}

func (rpo *Repo) GetSystem(id int64) (*System, error) {
	if res, ok := rpo.chSysId[id]; ok {
		return res, nil
	}
	res := &System{Entity: Entity{r: rpo, Id: id}}
	row := rpo.db.QueryRow("select name, x, y, z, bno from system where id=$1", id)
	var tx, ty, tz sql.NullFloat64
	var bno sql.NullInt64
	err := row.Scan(&res.Name, &tx, &ty, &tz, &bno)
	if tx.Valid && ty.Valid && tz.Valid {
		res.Coos[Xk], res.Coos[Yk], res.Coos[Zk] = tx.Float64, ty.Float64, tz.Float64
	} else {
		res.Coos = NaV3D
	}
	if bno.Valid {
		res.BodyNo = int(bno.Int64)
	} else {
		res.BodyNo = 0
	}
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	rpo.chSys(res)
	return res, nil
}

func (rpo *Repo) FindSystem(name string) (*System, error) {
	if res, ok := rpo.chSysNm[name]; ok {
		return res, nil
	}
	res := &System{Name: name}
	row := rpo.db.QueryRow(`select id, x, y, z, bno from system
	                        where name=$1 collate nocase`, res.Name)
	var tx, ty, tz sql.NullFloat64
	var bno sql.NullInt64
	err := row.Scan(&res.Id, &tx, &ty, &tz, &bno)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	if tx.Valid && ty.Valid && tz.Valid {
		res.Coos[Xk], res.Coos[Yk], res.Coos[Zk] = tx.Float64, ty.Float64, tz.Float64
	} else {
		res.Coos = NaV3D
	}
	if bno.Valid {
		res.BodyNo = int(bno.Int64)
	} else {
		res.BodyNo = 0
	}
	res.r = rpo
	rpo.chSys(res)
	return res, nil
}

func (rpo *Repo) MustSystemCoos(name string, x, y, z float64) (*System, error) {
	res, err := rpo.FindSystem(name)
	if err != nil {
		return res, err
	}
	if res == nil {
		res = &System{
			Name: name,
			Coos: Vec3D{x, y, z},
		}
		_, err = rpo.PutSystem(res)
		if err != nil {
			return res, err
		}
	} else {
		res.Coos[Xk], res.Coos[Yk], res.Coos[Zk] = x, y, z
		_, err = rpo.PutSystem(res)
		if err != nil {
			return res, err
		}
	}
	return res, nil
}

func (rpo *Repo) MustSystem(name string) (*System, error) {
	reuse, err := rpo.FindSystem(name)
	if err != nil {
		return reuse, err
	}
	if reuse == nil {
		reuse = &System{
			Name: name,
			Coos: NaV3D,
		}
		_, err = rpo.PutSystem(reuse)
	}
	return reuse, err
}

func nvlInt(nv sql.NullInt64, v int) int {
	if nv.Valid {
		return int(nv.Int64)
	} else {
		return v
	}
}

func nvlInt64(nv sql.NullInt64, v int64) int64 {
	if nv.Valid {
		return nv.Int64
	} else {
		return v
	}
}

func nvlFloat32(nv sql.NullFloat64, v float32) float32 {
	if nv.Valid {
		return float32(nv.Float64)
	} else {
		return v
	}
}

func (sys *System) Parts() ([]*SysPart, error) {
	if sys.parts == nil {
		rows, err := sys.r.db.Query(`select id, typ, name, dfc, tto, hgt, lat, lon
	        from syspart where sys=$1`,
			sys.Id)
		if err != nil {
			return nil, err
		}
		var tto sql.NullInt64
		var dfc, hgt, lat, lon sql.NullFloat64
		for rows.Next() {
			p := &SysPart{Entity: Entity{r: sys.r}, sys: sys, SysId: sys.Id}
			err = rows.Scan(&p.Id, &p.Type, &p.Name,
				&dfc, &tto,
				&hgt, &lat, &lon)
			p.FromCenter = nvlFloat32(dfc, -1)
			p.TiedTo = nvlInt64(tto, 0)
			p.Height = nvlFloat32(hgt, float32(math.NaN()))
			p.Lat = nvlFloat32(lat, float32(math.NaN()))
			p.Lon = nvlFloat32(lon, float32(math.NaN()))
			if err != nil {
				return nil, err
			}
			sys.parts = append(sys.parts, p)
		}
	}
	return sys.parts, nil
}

func (sys *System) FindPart(typ PartType, name string) (*SysPart, error) {
	parts, err := sys.Parts()
	if err != nil {
		return nil, err
	}
	for _, p := range parts {
		if p.Type == typ && p.Name == name {
			return p, nil
		}
	}
	return nil, nil
}

func (sys *System) MustPart(typ PartType, name string) (*SysPart, error) {
	res, err := sys.FindPart(typ, name)
	if res != nil || err != nil {
		return res, err
	}
	res, err = sys.AddPart(&SysPart{
		Type:       typ,
		Name:       name,
		FromCenter: -1,
		Height:     NaN32,
		Lat:        NaN32,
		Lon:        NaN32,
	})
	return res, err
}

type PartType int

//go:generate stringer -type PartType
const (
	Star PartType = iota
	Planet
	Belt
	Ring
	Port
)

type SysPart struct {
	Entity
	SysId      int64
	Type       PartType
	Name       string
	FromCenter float32
	TiedTo     int64
	Height     float32
	Lat, Lon   float32
	sys        *System
	rscs       []*Resource
}

func (sys *System) AddPart(part *SysPart) (*SysPart, error) {
	part.sys = sys
	part.SysId = sys.Id
	if sys.r != nil {
		return sys.r.PutSysPart(part)
	}
	sys.parts = append(sys.parts, part)
	return part, nil
}

func (rpo *Repo) PutSysPart(part *SysPart) (*SysPart, error) {
	var err error
	xa := rpo.XaBegin()
	defer xa.Rollback()
	if part.r == nil {
		var res sql.Result
		res, err = rpo.tx.Exec(`insert into syspart
			(sys, typ, name, dfc, tto, hgt, lat, lon)
			values ($1, $2, $3, $4, $5, $6, $7, $8)`,
			part.SysId, part.Type, part.Name, part.FromCenter,
			part.TiedTo, part.Height, part.Lat, part.Lon)
		if err == nil {
			if part.Id, err = res.LastInsertId(); err == nil {
				part.r = rpo
				xa.Commit()
				rpo.chPart(part)
			}
		}
	} else {
		_, err := rpo.tx.Exec(`update syspart set
			sys=$1, typ=$2, name=$3, dfc=$4, tto=$5, hgt=$6, lat=$7, lon=$8
		    where id=$9`,
			part.SysId, part.Type, part.Name, part.FromCenter,
			part.TiedTo, part.Height, part.Lat, part.Lon,
			part.Id)
		if err == nil {
			xa.Commit()
			rpo.chPart(part)
		}
	}
	return part, err
}

func (rpo *Repo) GetSysPart(id int64) (*SysPart, error) {
	if res, ok := rpo.chPartId[id]; ok {
		return res, nil
	}
	res := &SysPart{Entity: Entity{r: rpo, Id: id}}
	var tto sql.NullInt64
	var dfc, hgt, lat, lon sql.NullFloat64
	row := rpo.db.QueryRow(`select sys, typ, name, dfc, tto, hgt, lat, lon
		                    from syspart where id=$1`, id)
	err := row.Scan(&res.SysId, &res.Type, &res.Name,
		&dfc, &tto,
		&hgt, &lat, &lon)
	res.FromCenter = nvlFloat32(dfc, -1)
	res.TiedTo = nvlInt64(tto, 0)
	res.Height = nvlFloat32(hgt, NaN32)
	res.Lat = nvlFloat32(lat, NaN32)
	res.Lon = nvlFloat32(lon, NaN32)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	rpo.chPart(res)
	return res, nil
}

type Resource struct {
	Entity
	LocId   int64
	Name    string
	OccFreq float32
	loc     *SysPart
}

func (rpo *Repo) PutResource(rsc *Resource) (*Resource, error) {
	var err error
	xa := rpo.XaBegin()
	defer xa.Rollback()
	if rsc.r == nil {
		var res sql.Result
		res, err = rpo.tx.Exec(`insert into resource
			(loc, name, freq)
			values ($1, $2, $3)`,
			rsc.LocId, rsc.Name, rsc.OccFreq)
		if err == nil {
			if rsc.Id, err = res.LastInsertId(); err == nil {
				rsc.r = rpo
				xa.Commit()
				rpo.chRes(rsc)
			}
		}
	} else {
		_, err := rpo.tx.Exec(`update resource set
			loc=$1, name=$2, freq=$2
		    where id=$3`,
			rsc.LocId, rsc.Name, rsc.OccFreq,
			rsc.Id)
		if err == nil {
			xa.Commit()
			rpo.chRes(rsc)
		}
	}
	return rsc, err
}

func (loc *SysPart) AddResource(rsc *Resource) (*Resource, error) {
	rsc.loc = loc
	rsc.LocId = loc.Id
	if loc.r != nil {
		return loc.r.PutResource(rsc)
	}
	loc.rscs = append(loc.rscs, rsc)
	return rsc, nil
}
