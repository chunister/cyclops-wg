package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// cpuAmostra guarda os contadores cumulativos de /proc/stat. O uso de CPU
// não é um valor absoluto, é a diferença entre duas leituras no tempo.
type cpuAmostra struct {
	ocioso int64
	total  int64
}

func lerCPUAmostra() (cpuAmostra, error) {
	f, err := os.Open("/proc/stat")
	if err != nil {
		return cpuAmostra{}, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	if !scanner.Scan() {
		return cpuAmostra{}, fmt.Errorf("/proc/stat vazio")
	}
	campos := strings.Fields(scanner.Text())
	if len(campos) < 8 || campos[0] != "cpu" {
		return cpuAmostra{}, fmt.Errorf("linha inesperada em /proc/stat: %q", scanner.Text())
	}

	var total, ocioso int64
	for i, campo := range campos[1:] {
		v, err := strconv.ParseInt(campo, 10, 64)
		if err != nil {
			return cpuAmostra{}, err
		}
		total += v
		if i == 3 || i == 4 { // idle, iowait
			ocioso += v
		}
	}
	return cpuAmostra{ocioso: ocioso, total: total}, nil
}

// percentualCPU calcula o uso de CPU entre duas leituras de /proc/stat.
func percentualCPU(anterior, atual cpuAmostra) float64 {
	deltaTotal := atual.total - anterior.total
	deltaOcioso := atual.ocioso - anterior.ocioso
	if deltaTotal <= 0 {
		return 0
	}
	uso := float64(deltaTotal-deltaOcioso) / float64(deltaTotal) * 100
	switch {
	case uso < 0:
		return 0
	case uso > 100:
		return 100
	default:
		return uso
	}
}

// usoRAM lê /proc/meminfo e devolve o percentual usado e o usado/total em GB.
func usoRAM() (percentual, usadoGB, totalGB float64, err error) {
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return 0, 0, 0, err
	}
	defer f.Close()

	valores := map[string]int64{}
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		campos := strings.Fields(scanner.Text())
		if len(campos) < 2 {
			continue
		}
		chave := strings.TrimSuffix(campos[0], ":")
		if chave != "MemTotal" && chave != "MemAvailable" {
			continue
		}
		if v, err := strconv.ParseInt(campos[1], 10, 64); err == nil {
			valores[chave] = v
		}
	}

	total, ok1 := valores["MemTotal"]
	disponivel, ok2 := valores["MemAvailable"]
	if !ok1 || !ok2 {
		return 0, 0, 0, fmt.Errorf("MemTotal ou MemAvailable ausente em /proc/meminfo")
	}

	usado := total - disponivel
	totalGB = float64(total) / 1024 / 1024
	usadoGB = float64(usado) / 1024 / 1024
	if total > 0 {
		percentual = float64(usado) / float64(total) * 100
	}
	return percentual, usadoGB, totalGB, nil
}
