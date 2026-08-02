package processing

import (
	"math"

	"github.com/paulmach/orb"
)

func xyzToLatlon(x, y, z int) (lat float64, lon float64) {
	n := math.Pi - 2.0*math.Pi*float64(y)/math.Exp2(float64(z))
	lat = 180.0 / math.Pi * math.Atan(0.5*(math.Exp(n)-math.Exp(-n)))
	lon = float64(x)/math.Exp2(float64(z))*360.0 - 180.0
	return lat, lon
}

func gridSizeAtLevel(level int) float64 {
	return 270.0 / math.Pow(2.0, float64(level+7))
}

func snapToGrid(f, size float64) float64 {
	return math.Round(f/size) * size
}

type TileKey struct {
	Z, X, Y int
}

func LonLatToTileXY(lon, lat float64, z int) (x, y int) {
	n := math.Exp2(float64(z))
	x = int((lon + 180.0) / 360.0 * n)
	latRad := lat * math.Pi / 180.0
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

type GridPoint struct {
	Point orb.Point
	Count int
	Tier  int
}

type RenderingPoints struct {
	Points []GridPoint
}

func GetMultiPoint(sessionId int64, x, y, z int) (points RenderingPoints, ok bool) {
	var session Session
	if session, ok = sessions[sessionId]; !ok {
		return
	}
	if z > MAX_LEVEL {
		z = MAX_LEVEL
	}
	points = session.Tiles[TileKey{Z: z, X: x, Y: y}]
	return points, true
}
