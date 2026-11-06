package processing

import (
	"math"
	"time"

	"github.com/paulmach/orb"
)

const NUM_TIERS = 10

func gridSizeAtLevel(level int) float64 {
	return math.Ldexp(270.0, -(level + 7))
}

func packXY(x, y int) uint64 {
	return (uint64(uint32(x)) << 32) | uint64(uint32(y))
}

func packGrid(gx, gy int32) uint64 {
	return (uint64(uint32(gx)) << 32) | uint64(uint32(gy))
}

func unpackGrid(k uint64) (gx, gy int32) {
	return int32(k >> 32), int32(k)
}

type TileKey struct {
	Z, X, Y int
}

func LonLatToTileXY(lon, lat float64, z int) (x, y int) {
	n := math.Ldexp(1.0, z)
	x = int((lon + 180.0) / 360.0 * n)
	latRad := lat * (math.Pi / 180.0)
	cosLat := math.Cos(latRad)
	if cosLat == 0 {
		cosLat = 1e-10
	}
	t := math.Tan(latRad) + 1.0/cosLat
	if t <= 0 {
		t = 1e-10
	}
	y = int((1.0 - math.Log(t)/math.Pi) / 2.0 * n)
	return x, y
}

// RenderingPoints stores points pre-grouped by tier (index 0..9 -> tier 1..10).
type RenderingPoints struct {
	Tiers [NUM_TIERS]orb.MultiPoint
}

func (rp *RenderingPoints) TotalPoints() int {
	if rp == nil {
		return 0
	}
	total := 0
	for _, mp := range rp.Tiers {
		total += len(mp)
	}
	return total
}

func GetSession(sessionId int64) (*Session, bool) {
	sessionsMutex.RLock()
	session, ok := sessions[sessionId]
	sessionsMutex.RUnlock()
	if !ok || (session.Expiry > 0 && session.Expiry <= time.Now().Unix()) {
		return nil, false
	}
	return session, true
}

func (s *Session) GetTile(x, y, z int) *RenderingPoints {
	if z < 0 {
		return nil
	}
	if z > MAX_LEVEL {
		z = MAX_LEVEL
	}
	levelMap := s.Tiles[z]
	if levelMap == nil {
		return nil
	}
	return levelMap[packXY(x, y)]
}

func (s *Session) GetCachedMVT(key TileKey) ([]byte, bool) {
	s.mvtMutex.RLock()
	data, ok := s.mvtCache[key]
	s.mvtMutex.RUnlock()
	return data, ok
}

func (s *Session) SetCachedMVT(key TileKey, data []byte) {
	s.mvtMutex.Lock()
	s.mvtCache[key] = data
	s.mvtMutex.Unlock()
}

func GetMultiPoint(sessionId int64, x, y, z int) (*RenderingPoints, bool) {
	if z < 0 {
		return nil, false
	}
	session, ok := GetSession(sessionId)
	if !ok {
		return nil, false
	}
	return session.GetTile(x, y, z), true
}
