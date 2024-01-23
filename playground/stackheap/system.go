package main

import (
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/mem"
)

// getSystemResourceUsage returns CPU usage for all cores and memory usage.
func getSystemResourceUsage() ([]float64, float64, error) {

	// Get CPU usage for each core
	cpuPercents, err := cpu.Percent(0, true)
	if err != nil {
		return nil, 0, err
	}

	// Get Memory usage
	memStats, err := mem.VirtualMemory()
	if err != nil {
		return nil, 0, err
	}

	memUsage := memStats.UsedPercent

	return cpuPercents, memUsage, nil
}

// check if one of the core is reaching overload
func checkCPUOverload(cpuPercents []float64) bool {
	const cpuOverloadThreshold = 90.0
	for _, usage := range cpuPercents {
		if usage > cpuOverloadThreshold {
			return true
		}
	}
	return false
}
