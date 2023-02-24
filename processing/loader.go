package processing

import (
	"bufio"
	"os"
	"sort"
	"strconv"
	"strings"
)

const MAX_LEVEL = 16

const PERCENT_YELLOW = 60
const PERCENT_ORANGE = 80
const PERCENT_RED = 98

var lcounter [MAX_LEVEL]map[[2]float64]int
var threshold [MAX_LEVEL][3]int

func init() {
	counter := map[[2]float64]int{}
	f, _ := os.Open("data/db/full.csv")
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Scan()

	for scanner.Scan() {
		s := strings.Split(scanner.Text(), ",")
		x, _ := strconv.ParseFloat(s[1], 64)
		y, _ := strconv.ParseFloat(s[2], 64)
		counter[[2]float64{x, y}]++
	}

	for i := 0; i < MAX_LEVEL; i++ {
		lcounter[i] = map[[2]float64]int{}
		size := gridSizeAtLevel(i)
		for coord, cnt := range counter {
			lcounter[i][[2]float64{snapToGrid(coord[0], size), snapToGrid(coord[1], size)}] += cnt
		}

		counts := []int{}
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
}
