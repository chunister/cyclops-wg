package main

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Catalogo lê os nomes de servidor disponíveis em servers/ (sem a extensão .conf).
func Catalogo(dir string) ([]string, error) {
	entradas, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var nomes []string
	for _, e := range entradas {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".conf") {
			continue
		}
		nomes = append(nomes, strings.TrimSuffix(e.Name(), ".conf"))
	}
	return nomes, nil
}

// Cooldown lembra quando cada servidor foi usado pela última vez, para não
// reaproveitar um servidor que acabou de falhar antes do tempo de espera passar.
// É salvo em disco para sobreviver a um reinício do controlador.
type Cooldown struct {
	caminho string
	mu      sync.Mutex
	usoEm   map[string]time.Time
}

func AbreCooldown(caminho string) *Cooldown {
	c := &Cooldown{caminho: caminho, usoEm: map[string]time.Time{}}
	dados, err := os.ReadFile(caminho)
	if err == nil {
		json.Unmarshal(dados, &c.usoEm)
	}
	return c
}

func (c *Cooldown) Marca(servidor string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.usoEm[servidor] = time.Now()
	dados, err := json.Marshal(c.usoEm)
	if err == nil {
		os.WriteFile(c.caminho, dados, 0o644)
	}
}

func (c *Cooldown) EmEspera(servidor string, duracao time.Duration) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	t, ok := c.usoEm[servidor]
	if !ok {
		return false
	}
	return time.Since(t) < duracao
}

// Escolhe sorteia um servidor do catálogo que não esteja em uso agora (em
// usados) nem em cooldown, e marca como usado antes de devolver.
func Escolhe(catalogo []string, usados map[string]bool, cd *Cooldown, espera time.Duration) (string, error) {
	candidatos := make([]string, 0, len(catalogo))
	for _, s := range catalogo {
		if usados[s] {
			continue
		}
		if cd.EmEspera(s, espera) {
			continue
		}
		candidatos = append(candidatos, s)
	}
	if len(candidatos) == 0 {
		return "", fmt.Errorf("nenhum servidor livre: todos em uso ou em cooldown")
	}
	escolhido := candidatos[rand.Intn(len(candidatos))]
	cd.Marca(escolhido)
	return escolhido, nil
}

func confPath(dir, servidor string) string {
	return filepath.Join(dir, servidor+".conf")
}
