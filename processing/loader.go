package processing

import (
	"bufio"
	"log"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/paulmach/orb"
)

const MAX_LEVEL = 15
const EARTH_RADIUS_M = 6371000.0

func haversineMeters(lon1, lat1, lon2, lat2 float64) float64 {
	lat1Rad := lat1 * math.Pi / 180.0
	lat2Rad := lat2 * math.Pi / 180.0
	dLat := (lat2 - lat1) * math.Pi / 180.0
	dLon := (lon2 - lon1) * math.Pi / 180.0

	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1Rad)*math.Cos(lat2Rad)*math.Sin(dLon/2)*math.Sin(dLon/2)
	c := 2.0 * math.Atan2(math.Sqrt(a), math.Sqrt(1.0-a))
	return EARTH_RADIUS_M * c
}

func filterHighSpeedPoints(points []DataPoint, maxKmh float64, maxDeltaSeconds int64) []DataPoint {
	if len(points) < 2 {
		return points
	}
	maxMs := maxKmh / 3.6 // km/h to m/s
	filtered := make([]DataPoint, 0, len(points))
	isHighSpeed := make([]bool, len(points))

	for i := 1; i < len(points); i++ {
		dt := points[i].timestamp - points[i-1].timestamp
		if dt > 0 && dt <= maxDeltaSeconds {
			dist := haversineMeters(points[i-1].x, points[i-1].y, points[i].x, points[i].y)
			if dist/float64(dt) >= maxMs {
				isHighSpeed[i-1] = true
				isHighSpeed[i] = true
			}
		}
	}

	for i, d := range points {
		if !isHighSpeed[i] {
			filtered = append(filtered, d)
		}
	}
	return filtered
}

const PERCENT_YELLOW = 60
const PERCENT_ORANGE = 80
const PERCENT_RED = 98

const MAIN_SESSION_ID = -1

type Session struct {
	Expiry       int64
	LevelCounter [MAX_LEVEL]map[[2]float64]map[int64]bool
	Threshold    [MAX_LEVEL][3]int
	Tiles        map[TileKey]RenderingPoints
}

var sessions map[int64]Session

type DataPoint struct {
	timestamp int64
	x         float64
	y         float64
}

var data []DataPoint

func cleanUp() {
	for k, s := range sessions {
		if s.Expiry > 0 && s.Expiry < time.Now().Unix() {
			delete(sessions, k)
		}
	}
}

func LoadSession(t1, t2, sessionId int64, isMain bool) {
	delete(sessions, sessionId)
	cleanUp()

	counter := map[[2]float64]int{}

	for _, d := range data {
		if d.timestamp >= t1 && d.timestamp <= t2 {
			key := [2]float64{d.x, d.y}
			counter[key]++
		}
	}

	var expiry int64 = -1
	if !isMain {
		expiry = time.Now().Add(time.Hour).Unix()
	}
	lcounter := [MAX_LEVEL]map[[2]float64]map[int64]bool{}
	threshold := [MAX_LEVEL][3]int{}
	tiles := map[TileKey]RenderingPoints{}

	for i := 0; i < MAX_LEVEL; i++ {
		lcounter[i] = map[[2]float64]map[int64]bool{}
		size := gridSizeAtLevel(i)
		levelCounts := map[[2]float64]int{}
		for coord, cnt := range counter {
			key := [2]float64{snapToGrid(coord[0], size), snapToGrid(coord[1], size)}
			levelCounts[key] += cnt
		}

		counts := make([]int, 0, len(levelCounts)+1)
		counts = append(counts, 0)
		for _, cnt := range levelCounts {
			counts = append(counts, cnt)
		}
		sort.Ints(counts)

		n := len(counts)
		p := []int{
			counts[n*30/100],
			counts[n*45/100],
			counts[n*60/100],
			counts[n*70/100],
			counts[n*80/100],
			counts[n*88/100],
			counts[n*94/100],
			counts[n*98/100],
			counts[n*995/1000],
		}
		for j := 1; j < len(p); j++ {
			if p[j] <= p[j-1] {
				p[j] = p[j-1] + 1
			}
		}

		for coord, cnt := range levelCounts {
			tier := 1
			for j := len(p) - 1; j >= 0; j-- {
				if cnt >= p[j] {
					tier = j + 2
					break
				}
			}
			tx, ty := LonLatToTileXY(coord[0], coord[1], i)
			tKey := TileKey{Z: i, X: tx, Y: ty}
			pts := tiles[tKey]
			point := orb.Point{coord[0], coord[1]}
			pts.Points = append(pts.Points, GridPoint{
				Point: point,
				Count: cnt,
				Tier:  tier,
			})
			tiles[tKey] = pts
		}
	}
	sessions[sessionId] = Session{expiry, lcounter, threshold, tiles}
}

func timestampToDate(timestamp int64) int64 {
	return timestamp / 86400
}

func init() {
	start := time.Now()
	log.Println("[rfiel] Initializing data loader...")
	sessions = map[int64]Session{}

	data = []DataPoint{}

	paths := []string{
		"data/db/full.csv",
		"../data/db/full.csv",
	}
	if exe, err := os.Executable(); err == nil {
		dir := filepath.Dir(exe)
		paths = append(paths, filepath.Join(dir, "data/db/full.csv"), filepath.Join(dir, "../data/db/full.csv"))
	}

	var f *os.File
	var err error
	var loadedPath string
	for _, p := range paths {
		f, err = os.Open(p)
		if err == nil {
			loadedPath = p
			break
		}
	}
	if err != nil {
		cwd, _ := os.Getwd()
		log.Fatalf("[rfiel] FATAL ERROR: could not open data/db/full.csv in CWD (%s) or executable directory: %v", cwd, err)
		return
	}
	defer f.Close()

	log.Printf("[rfiel] Successfully opened dataset from: %s", loadedPath)

	scanner := bufio.NewScanner(f)
	scanner.Scan()

	for scanner.Scan() {
		s := strings.Split(scanner.Text(), ",")
		t, _ := strconv.ParseInt(s[0], 10, 64)
		x, _ := strconv.ParseFloat(s[1], 64)
		y, _ := strconv.ParseFloat(s[2], 64)
		data = append(data, DataPoint{t, x, y})
	}

	rawCount := len(data)
	log.Printf("[rfiel] Loaded %d raw GPS records from full.csv", rawCount)

	// Filter out airplane tracks and GPS glitches (avg speed >= 400 km/h within 5 mins)
	data = filterHighSpeedPoints(data, 400.0, 300)
	log.Printf("[rfiel] After high-speed flight filtering: %d clean GPS records remain (removed %d flight/glitch points)", len(data), rawCount-len(data))

	log.Println("[rfiel] Indexing main session across 15 zoom levels...")
	LoadSession(0, time.Now().Unix(), MAIN_SESSION_ID, true)
	log.Printf("[rfiel] Main session ready! Total initialization time: %v", time.Since(start))
}
