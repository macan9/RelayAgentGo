package collector

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type cpuSample struct {
	idle  uint64
	total uint64
}

func readCPUSample(procRoot string) (cpuSample, error) {
	content, err := os.ReadFile(filepath.Join(procRoot, "stat"))
	if err != nil {
		return cpuSample{}, err
	}

	for _, line := range strings.Split(string(content), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 5 || fields[0] != "cpu" {
			continue
		}

		var values []uint64
		for _, field := range fields[1:] {
			value, err := strconv.ParseUint(field, 10, 64)
			if err != nil {
				return cpuSample{}, fmt.Errorf("parse cpu stat: %w", err)
			}
			values = append(values, value)
		}

		var total uint64
		for _, value := range values {
			total += value
		}

		idle := values[3]
		if len(values) > 4 {
			idle += values[4]
		}

		return cpuSample{idle: idle, total: total}, nil
	}

	return cpuSample{}, fmt.Errorf("cpu stat is missing")
}

func (sample cpuSample) percentSince(previous cpuSample) float64 {
	totalDelta := sample.total - previous.total
	if totalDelta == 0 {
		return 0
	}

	idleDelta := sample.idle - previous.idle
	if idleDelta > totalDelta {
		return 0
	}

	return float64(totalDelta-idleDelta) * 100 / float64(totalDelta)
}

func readLoad1(procRoot string) (float64, error) {
	content, err := os.ReadFile(filepath.Join(procRoot, "loadavg"))
	if err != nil {
		return 0, err
	}

	fields := strings.Fields(string(content))
	if len(fields) == 0 {
		return 0, fmt.Errorf("loadavg is empty")
	}

	load1, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return 0, fmt.Errorf("parse load1: %w", err)
	}

	return load1, nil
}

func readMemoryPercent(procRoot string) (float64, error) {
	content, err := os.ReadFile(filepath.Join(procRoot, "meminfo"))
	if err != nil {
		return 0, err
	}

	values := map[string]uint64{}
	for _, line := range strings.Split(string(content), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}

		key := strings.TrimSuffix(fields[0], ":")
		value, err := strconv.ParseUint(fields[1], 10, 64)
		if err != nil {
			continue
		}
		values[key] = value
	}

	total := values["MemTotal"]
	available := values["MemAvailable"]
	if total == 0 {
		return 0, fmt.Errorf("MemTotal is missing")
	}
	if available == 0 {
		available = values["MemFree"] + values["Buffers"] + values["Cached"]
	}

	used := total - available
	return float64(used) * 100 / float64(total), nil
}

func readNetworkBytes(sysfsRoot string, interfaces []string) (uint64, uint64, error) {
	var rxTotal uint64
	var txTotal uint64
	var found bool

	for _, name := range interfaces {
		rx, err := readUint(filepath.Join(sysfsRoot, "class", "net", name, "statistics", "rx_bytes"))
		if err != nil {
			continue
		}
		tx, err := readUint(filepath.Join(sysfsRoot, "class", "net", name, "statistics", "tx_bytes"))
		if err != nil {
			continue
		}

		rxTotal += rx
		txTotal += tx
		found = true
	}

	if !found {
		return 0, 0, fmt.Errorf("no network statistics found")
	}

	return rxTotal, txTotal, nil
}

func readUint(path string) (uint64, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}

	value, err := strconv.ParseUint(strings.TrimSpace(string(content)), 10, 64)
	if err != nil {
		return 0, err
	}

	return value, nil
}
