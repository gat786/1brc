package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

type Results struct {
	minv  float32
	maxv  float32
	total float32
	count int
}

func main() {
	df := os.Getenv("MEASUREMENTS_FILE")

	fmt.Printf("Processing: %s\n", df)

	results := map[string]Results{}
	fp, err := os.Open(df)
	if err != nil {
		os.Exit(1)
	}
	defer fp.Close()

	reader := bufio.NewReader(fp)
	lineno := 0
	for {
		line, err := reader.ReadString('\n')
		if errors.Is(err, io.EOF) {
			fmt.Println("Measurements file read completely")
			break
		}

		city_and_val := strings.Split(strings.TrimSuffix(line, "\n"), ";")
		city_name := city_and_val[0]
		numv, err := strconv.ParseFloat(city_and_val[1], 32)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		if v, ok := results[city_name]; ok {
			results[city_name] = Results{
				minv:  min(v.minv, float32(numv)),
				maxv:  max(v.maxv, float32(numv)),
				total: v.total + float32(numv),
				count: v.count + 1,
			}
		} else {
			results[city_and_val[0]] = Results{
				minv:  float32(numv),
				maxv:  float32(numv),
				total: float32(numv),
				count: 1,
			}
		}
		// fmt.Printf("%d\r", lineno)
		lineno += 1
	}

	for key, value := range results {
		fmt.Printf("%s: %v/%v/%v\n", key, value.minv, (value.total / float32(value.count)), value.maxv)
	}
}
