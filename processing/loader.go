package processing

import (
	"bufio"
	"os"
	"strconv"
	"strings"
)

const MAX_LEVEL = 18

var counter map[[2]float64]int
var lcounter [MAX_LEVEL]map[[2]float64]int

func init() {
	counter = map[[2]float64]int{}
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
	}
}
