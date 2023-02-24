package processing

import (
	"bufio"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
)

const MAX_LEVEL = 15

const PERCENT_YELLOW = 60
const PERCENT_ORANGE = 80
const PERCENT_RED = 98

const MAIN_SESSION_ID = -1

type Session struct {
	Expiry       int64
	LevelCounter [MAX_LEVEL]map[[2]float64]int
	Threshold    [MAX_LEVEL][3]int
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
			counter[[2]float64{d.x, d.y}]++
		}
	}

	var expiry int64 = -1
	if !isMain {
		expiry = time.Now().Add(time.Hour).Unix()
	}
	lcounter := [MAX_LEVEL]map[[2]float64]int{}
	threshold := [MAX_LEVEL][3]int{}

	for i := 0; i < MAX_LEVEL; i++ {
		lcounter[i] = map[[2]float64]int{}
		size := gridSizeAtLevel(i)
		for coord, cnt := range counter {
			lcounter[i][[2]float64{snapToGrid(coord[0], size), snapToGrid(coord[1], size)}] += cnt
		}

		counts := []int{0}
		for _, count := range lcounter[i] {
			counts = append(counts, count)
		}
		sort.Ints(counts)
		greenThreshold := counts[0]
		yellowThreshold := counts[len(counts)*PERCENT_YELLOW/100]
		if yellowThreshold == greenThreshold {
			yellowThreshold += 1
		}
		orangeThreshold := counts[len(counts)*PERCENT_ORANGE/100]
		if orangeThreshold == yellowThreshold {
			orangeThreshold += 1
		}
		redThreshold := counts[len(counts)*PERCENT_RED/100]
		if redThreshold == orangeThreshold {
			redThreshold += 1
		}
		threshold[i] = [3]int{redThreshold, orangeThreshold, yellowThreshold}
	}
	sessions[sessionId] = Session{expiry, lcounter, threshold}
}

func init() {
	sessions = map[int64]Session{}

	data = []DataPoint{}
	f, _ := os.Open("data/db/full.csv")
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Scan()

	for scanner.Scan() {
		s := strings.Split(scanner.Text(), ",")
		t, _ := strconv.ParseInt(s[0], 10, 64)
		x, _ := strconv.ParseFloat(s[1], 64)
		y, _ := strconv.ParseFloat(s[2], 64)
		data = append(data, DataPoint{t, x, y})
	}

	LoadSession(0, time.Now().Unix(), MAIN_SESSION_ID, true)
}
