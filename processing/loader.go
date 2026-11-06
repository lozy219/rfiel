package processing

import (
	"bufio"
	"bytes"
	"log"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/paulmach/orb"
)

const MAX_LEVEL = 15
const NUM_LEVELS = MAX_LEVEL + 1
const EARTH_RADIUS_M = 6371000.0
const MAIN_SESSION_ID = -1
const degE7MaxMeters = (EARTH_RADIUS_M * math.Pi / 180.0) * 1e-7 // ~0.01111949m per 1e-7 deg

type Session struct {
	Expiry   int64
	Tiles    [NUM_LEVELS]map[uint64]*RenderingPoints
	mvtMutex sync.RWMutex
	mvtCache map[TileKey][]byte
}

var (
	sessionsMutex sync.RWMutex
	sessions      map[int64]*Session
)

func IsSessionValid(sessionId int64) bool {
	sessionsMutex.RLock()
	s, ok := sessions[sessionId]
	sessionsMutex.RUnlock()
	if !ok {
		return false
	}
	return s.Expiry < 0 || s.Expiry > time.Now().Unix()
}

func RefreshSession(sessionId int64) bool {
	now := time.Now().Unix()
	sessionsMutex.Lock()
	defer sessionsMutex.Unlock()
	s, ok := sessions[sessionId]
	if !ok {
		return false
	}
	if s.Expiry > 0 {
		if s.Expiry <= now {
			delete(sessions, sessionId)
			return false
		}
		s.Expiry = time.Now().Add(time.Hour).Unix()
	}
	return true
}

// DataPoint uses int32 fixed-point (1e-7 deg, ~1.1cm precision) to save 33% RAM (16B vs 24B).
type DataPoint struct {
	timestamp int64
	lonE7     int32
	latE7     int32
}

type coordCount struct {
	lon   float64
	lat   float64
	count int
}

var data []DataPoint

func cleanUp() {
	now := time.Now().Unix()
	sessionsMutex.Lock()
	defer sessionsMutex.Unlock()
	for k, s := range sessions {
		if s.Expiry > 0 && s.Expiry < now {
			delete(sessions, k)
		}
	}
}

func haversineMeters(lon1, lat1, lon2, lat2 float64) float64 {
	lat1Rad := lat1 * (math.Pi / 180.0)
	lat2Rad := lat2 * (math.Pi / 180.0)
	dLat := (lat2 - lat1) * (math.Pi / 180.0)
	dLon := (lon2 - lon1) * (math.Pi / 180.0)

	sinLat := math.Sin(dLat / 2)
	sinLon := math.Sin(dLon / 2)
	a := sinLat*sinLat + math.Cos(lat1Rad)*math.Cos(lat2Rad)*sinLon*sinLon
	c := 2.0 * math.Atan2(math.Sqrt(a), math.Sqrt(1.0-a))
	return EARTH_RADIUS_M * c
}

func absInt64(x int64) int64 {
	if x < 0 {
		return -x
	}
	return x
}

func filterHighSpeedPoints(points []DataPoint, maxKmh float64, maxDeltaSeconds int64) []DataPoint {
	if len(points) < 2 {
		return points
	}
	maxMs := maxKmh / 3.6 // km/h to m/s
	isHighSpeed := make([]bool, len(points))

	for i := 1; i < len(points); i++ {
		dt := points[i].timestamp - points[i-1].timestamp
		if dt > 0 && dt <= maxDeltaSeconds {
			maxDist := maxMs * float64(dt)
			dLonE7 := absInt64(int64(points[i].lonE7) - int64(points[i-1].lonE7))
			dLatE7 := absInt64(int64(points[i].latE7) - int64(points[i-1].latE7))
			// Fast spherical triangle-inequality upper bound skips trig on >99% of points.
			if dLonE7 < 1_000_000_000 && float64(dLonE7+dLatE7)*degE7MaxMeters < maxDist {
				continue
			}
			lon1 := float64(points[i-1].lonE7) * 1e-7
			lat1 := float64(points[i-1].latE7) * 1e-7
			lon2 := float64(points[i].lonE7) * 1e-7
			lat2 := float64(points[i].latE7) * 1e-7
			if haversineMeters(lon1, lat1, lon2, lat2) >= maxDist {
				isHighSpeed[i-1] = true
				isHighSpeed[i] = true
			}
		}
	}

	// Compact in-place to avoid allocating a second 72MB slice.
	writeIdx := 0
	for i, d := range points {
		if !isHighSpeed[i] {
			points[writeIdx] = d
			writeIdx++
		}
	}
	return points[:writeIdx]
}

func computePercentiles(levelCounts map[uint64]int) [9]int {
	const histLimit = 4096
	var hist [histLimit]int
	var overflow []int

	for _, cnt := range levelCounts {
		if cnt >= 0 && cnt < histLimit {
			hist[cnt]++
		} else {
			overflow = append(overflow, cnt)
		}
	}
	if len(overflow) > 1 {
		sort.Ints(overflow)
	}

	n := len(levelCounts) + 1 // includes leading 0 from original percentile logic
	targets := [9]int{
		n * 30 / 100,
		n * 45 / 100,
		n * 60 / 100,
		n * 70 / 100,
		n * 80 / 100,
		n * 88 / 100,
		n * 94 / 100,
		n * 98 / 100,
		n * 995 / 1000,
	}

	var p [9]int
	cum := 1 + hist[0] // index 0 is the synthetic 0 element
	v := 1
	for idx, target := range targets {
		if target == 0 {
			p[idx] = 0
			continue
		}
		for v < histLimit && cum <= target {
			cum += hist[v]
			if cum > target {
				break
			}
			v++
		}
		if v < histLimit {
			p[idx] = v
		} else {
			p[idx] = overflow[target-cum]
		}
	}

	for j := 1; j < len(p); j++ {
		if p[j] <= p[j-1] {
			p[j] = p[j-1] + 1
		}
	}
	return p
}

func buildZoomLevel(level int, unique []coordCount) map[uint64]*RenderingPoints {
	size := gridSizeAtLevel(level)
	estCap := len(unique) / 2
	if maxCells := 1 << (2*level + 6); level < 10 && estCap > maxCells {
		estCap = maxCells
	}
	if estCap < 16 {
		estCap = 16
	}

	levelCounts := make(map[uint64]int, estCap)
	for _, c := range unique {
		gx := int32(math.Round(c.lon / size))
		gy := int32(math.Round(c.lat / size))
		levelCounts[packGrid(gx, gy)] += c.count
	}

	p := computePercentiles(levelCounts)
	levelTiles := make(map[uint64]*RenderingPoints, len(levelCounts)/16+8)

	for gridKey, cnt := range levelCounts {
		tier := 1
		for j := len(p) - 1; j >= 0; j-- {
			if cnt >= p[j] {
				tier = j + 2
				break
			}
		}
		gx, gy := unpackGrid(gridKey)
		lon := float64(gx) * size
		lat := float64(gy) * size
		tx, ty := LonLatToTileXY(lon, lat, level)
		tKey := packXY(tx, ty)
		rp := levelTiles[tKey]
		if rp == nil {
			rp = &RenderingPoints{}
			levelTiles[tKey] = rp
		}
		rp.Tiers[tier-1] = append(rp.Tiers[tier-1], orb.Point{lon, lat})
	}
	return levelTiles
}

func LoadSession(t1, t2, sessionId int64, isMain bool) {
	cleanUp()

	var expiry int64 = -1
	if !isMain {
		expiry = time.Now().Add(time.Hour).Unix()
	}

	// Binary search on timestamp-sorted data instead of scanning all 4.5M records.
	startIdx := sort.Search(len(data), func(i int) bool {
		return data[i].timestamp >= t1
	})
	endIdx := sort.Search(len(data), func(i int) bool {
		return data[i].timestamp > t2
	})
	sub := data[startIdx:endIdx]

	var tiles [NUM_LEVELS]map[uint64]*RenderingPoints
	if len(sub) > 0 {
		counterCap := len(sub) / 2
		if counterCap > 2_500_000 {
			counterCap = 2_500_000
		}
		counter := make(map[uint64]int, counterCap)
		for _, d := range sub {
			key := (uint64(uint32(d.lonE7)) << 32) | uint64(uint32(d.latE7))
			counter[key]++
		}

		unique := make([]coordCount, 0, len(counter))
		for k, cnt := range counter {
			lon := float64(int32(k>>32)) * 1e-7
			lat := float64(int32(k)) * 1e-7
			unique = append(unique, coordCount{lon: lon, lat: lat, count: cnt})
		}
		counter = nil

		// Index all 16 zoom levels (0..15) concurrently, heaviest levels first.
		var wg sync.WaitGroup
		wg.Add(NUM_LEVELS)
		for lvl := MAX_LEVEL; lvl >= 0; lvl-- {
			go func(level int) {
				defer wg.Done()
				tiles[level] = buildZoomLevel(level, unique)
			}(lvl)
		}
		wg.Wait()
	}

	sess := &Session{
		Expiry:   expiry,
		Tiles:    tiles,
		mvtCache: make(map[TileKey][]byte),
	}
	sessionsMutex.Lock()
	sessions[sessionId] = sess
	sessionsMutex.Unlock()
}

func parseInt64Bytes(b []byte) (int64, bool) {
	if len(b) == 0 {
		return 0, false
	}
	var v int64
	for _, c := range b {
		if c < '0' || c > '9' {
			return 0, false
		}
		v = v*10 + int64(c-'0')
	}
	return v, true
}

func parseFixedE7(b []byte) (int32, bool) {
	if len(b) == 0 {
		return 0, false
	}
	i := 0
	neg := false
	if b[0] == '-' {
		neg = true
		i++
	} else if b[0] == '+' {
		i++
	}
	if i >= len(b) {
		return 0, false
	}
	var intPart int64
	hasDigits := false
	for i < len(b) && b[i] >= '0' && b[i] <= '9' {
		intPart = intPart*10 + int64(b[i]-'0')
		if intPart > 180 {
			return 0, false
		}
		hasDigits = true
		i++
	}
	var fracPart int64
	fracDigits := 0
	if i < len(b) && b[i] == '.' {
		i++
		for i < len(b) && b[i] >= '0' && b[i] <= '9' {
			hasDigits = true
			if fracDigits < 7 {
				fracPart = fracPart*10 + int64(b[i]-'0')
				fracDigits++
			} else if fracDigits == 7 {
				if b[i] >= '5' {
					fracPart++
				}
				fracDigits++
			}
			i++
		}
	}
	if !hasDigits || i != len(b) {
		return 0, false
	}
	for fracDigits < 7 {
		fracPart *= 10
		fracDigits++
	}
	val := intPart*10_000_000 + fracPart
	if neg {
		val = -val
	}
	return int32(val), true
}

func parseCoordE7(b []byte) int32 {
	if v, ok := parseFixedE7(b); ok {
		return v
	}
	f, _ := strconv.ParseFloat(string(b), 64)
	return int32(math.Round(f * 1e7))
}

func init() {
	start := time.Now()
	log.Println("[rfiel] Initializing data loader...")
	sessions = make(map[int64]*Session)

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

	data = make([]DataPoint, 0, 4_600_000)
	scanner := bufio.NewScanner(f)
	buf := make([]byte, 1024*1024)
	scanner.Buffer(buf, 1024*1024)
	scanner.Scan() // skip header

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) > 0 && line[len(line)-1] == '\r' {
			line = line[:len(line)-1]
		}
		c1 := bytes.IndexByte(line, ',')
		if c1 < 0 {
			continue
		}
		rest := line[c1+1:]
		c2 := bytes.IndexByte(rest, ',')
		if c2 < 0 {
			continue
		}
		tBytes := line[:c1]
		xBytes := rest[:c2]
		yBytes := rest[c2+1:]

		t, ok := parseInt64Bytes(tBytes)
		if !ok {
			t, _ = strconv.ParseInt(string(tBytes), 10, 64)
		}
		lonE7 := parseCoordE7(xBytes)
		latE7 := parseCoordE7(yBytes)
		data = append(data, DataPoint{timestamp: t, lonE7: lonE7, latE7: latE7})
	}

	rawCount := len(data)
	log.Printf("[rfiel] Loaded %d raw GPS records from full.csv (in %v)", rawCount, time.Since(start))

	// Filter out airplane tracks and GPS glitches (avg speed >= 310 km/h within 10s)
	data = filterHighSpeedPoints(data, 310.0, 10)
	log.Printf("[rfiel] After high-speed flight filtering: %d clean GPS records remain (removed %d flight/glitch points)", len(data), rawCount-len(data))

	// Ensure sorted by timestamp so year sessions can binary-search directly.
	if !sort.SliceIsSorted(data, func(i, j int) bool {
		return data[i].timestamp < data[j].timestamp
	}) {
		sort.Slice(data, func(i, j int) bool {
			return data[i].timestamp < data[j].timestamp
		})
	}

	log.Printf("[rfiel] Indexing main session across %d zoom levels (0..%d)...", NUM_LEVELS, MAX_LEVEL)
	LoadSession(0, time.Now().Unix(), MAIN_SESSION_ID, true)
	log.Printf("[rfiel] Main session ready! Total initialization time: %v", time.Since(start))
}
