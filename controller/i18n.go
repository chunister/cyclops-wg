package main

import (
	"fmt"
	"sync/atomic"
)

// idioma é o idioma ativo do dashboard e das mensagens de log do
// controlador. Guardado como atomic.Int32 porque é lido por goroutines em
// segundo plano (health check, rotação) e escrito pelo atalho "l" do
// dashboard, que roda em outra goroutine.
type idioma int32

const (
	idiomaEN idioma = iota
	idiomaPT
	idiomaES
)

func idiomaDeString(s string) idioma {
	switch s {
	case "pt", "pt-br", "pt_BR", "pt-BR":
		return idiomaPT
	case "es", "es-ES", "es_ES":
		return idiomaES
	default:
		return idiomaEN
	}
}

var idiomaAtivo atomic.Int32 // valores da const idioma; padrão idiomaEN (zero)

func DefineIdioma(i idioma) {
	idiomaAtivo.Store(int32(i))
}

func IdiomaAtual() idioma {
	return idioma(idiomaAtivo.Load())
}

// ProximoIdioma avança o ciclo en -> pt-br -> es -> en, usado pelo atalho
// "l" do dashboard, e devolve o novo idioma ativo.
func ProximoIdioma() idioma {
	novo := (IdiomaAtual() + 1) % 3
	DefineIdioma(novo)
	return novo
}

// T busca o texto de chave no idioma ativo e formata com args, como
// fmt.Sprintf. Cai pro inglês se a chave não existir no idioma ativo.
func T(chave string, args ...interface{}) string {
	modelo, ok := textos[IdiomaAtual()][chave]
	if !ok {
		if modelo, ok = textos[idiomaEN][chave]; !ok {
			return chave
		}
	}
	if len(args) == 0 {
		return modelo
	}
	return fmt.Sprintf(modelo, args...)
}

var textos = map[idioma]map[string]string{
	idiomaEN: {
		"panel_resources":               " resources (host) ",
		"panel_active":                  " active ",
		"panel_spare":                   " spare ",
		"panel_connections":             " recent connections ",
		"panel_events":                  " events ",
		"panel_shortcuts":               " shortcuts ",
		"shortcuts_text":                "[yellow]q[white]  quit\n[yellow]r[white]  renew all tunnels\n[yellow]p[white]  pause/resume screen\n[yellow]c[white]  clear events\n[yellow]l[white]  change language",
		"col_gateway":                   "GATEWAY",
		"col_server":                    "SERVER",
		"col_internal_ip":               "INTERNAL IP",
		"col_exit_ip":                   "EXIT IP",
		"col_last_switch":               "LAST SWITCH",
		"col_failures":                  "FAILURES",
		"col_state":                     "STATE",
		"col_time":                      "TIME",
		"col_source":                    "SOURCE",
		"col_destination":               "DESTINATION",
		"state_healthy":                 "healthy",
		"state_repairing":               "repairing",
		"state_failing":                 "failing",
		"time_ago":                      "%s ago",
		"summary_line":                  "active healthy: [green]%d[white]/%d    spare healthy: [green]%d[white]/%d    up for %s%s",
		"paused_suffix":                 "    [yellow]PAUSED (press p to resume)[white]",
		"no_reading":                    "(no reading: %v)",
		"err_wg_key_required":           "set WG_PRIVATE_KEY in the environment",
		"err_docker":                    "docker: %v",
		"err_network":                   "network: %v",
		"err_build":                     "build: %v",
		"err_catalog":                   "server catalog: %v",
		"err_catalog_too_small":         "catalog has %d servers, need %d (GATEWAY_COUNT + SPARE_COUNT)",
		"err_initial_boot":              "initial boot: %v",
		"log_bringing_up_gateway":       "bringing up %s with %s",
		"log_waiting_tunnels":           "waiting for tunnels to come up...",
		"log_no_ip_yet":                 "no IP yet for %s: %v",
		"log_exit_ip_lookup_failed":     "couldn't find the exit IP for %s: %v",
		"log_health_check_failed":       "%s (%s) failed the health check (%d): %v",
		"log_sick_no_spare":             "%s is sick, but no healthy spare is available; restarting the container",
		"log_restart_failed":            "restarting %s failed: %v",
		"log_promoting":                 "promoting %s in place of %s",
		"log_balancer_update_failed":    "updating the balancer: %v",
		"log_no_free_server":            "no free server for %s right now: %v",
		"log_running_check_failed":      "couldn't check whether %s is running: %v",
		"log_not_running_restarting":    "%s isn't running (probably crashed); restarting before switching servers",
		"log_not_running_in_time":       "%s didn't come back up in time",
		"log_switch_server_failed":      "switch-server on %s failed: %v",
		"log_now_points_to":             "%s now points to %s",
		"log_new_exit_ip_lookup_failed": "couldn't find the new exit IP for %s: %v",
		"log_rotation_already_running":  "a full rotation is already running, ignoring the new request",
		"log_rotation_start":            "full rotation: switching the server of %d gateways",
		"log_rotation_done":             "full rotation done",
		"log_status_server_error":       "controller status server: %v",
		"log_status_server_listening":   "controller status at :%s/status",
		"log_invalid_env_int":           "invalid value in %s=%q, using %d",
		"log_invalid_env_bool":          "invalid value in %s=%q, using %v",
	},
	idiomaPT: {
		"panel_resources":               " recursos (host) ",
		"panel_active":                  " ativos ",
		"panel_spare":                   " reservas ",
		"panel_connections":             " conexões recentes ",
		"panel_events":                  " eventos ",
		"panel_shortcuts":               " atalhos ",
		"shortcuts_text":                "[yellow]q[white]  sair\n[yellow]r[white]  renovar todos os túneis\n[yellow]p[white]  pausar/retomar a tela\n[yellow]c[white]  limpar eventos\n[yellow]l[white]  mudar idioma",
		"col_gateway":                   "GATEWAY",
		"col_server":                    "SERVIDOR",
		"col_internal_ip":               "IP INTERNO",
		"col_exit_ip":                   "IP SAÍDA",
		"col_last_switch":               "ÚLT. TROCA",
		"col_failures":                  "FALHAS",
		"col_state":                     "ESTADO",
		"col_time":                      "HORA",
		"col_source":                    "ORIGEM",
		"col_destination":               "DESTINO",
		"state_healthy":                 "saudável",
		"state_repairing":               "reparando",
		"state_failing":                 "falhando",
		"time_ago":                      "há %s",
		"summary_line":                  "ativos saudáveis: [green]%d[white]/%d    reservas saudáveis: [green]%d[white]/%d    no ar há %s%s",
		"paused_suffix":                 "    [yellow]PAUSADO (aperte p pra retomar)[white]",
		"no_reading":                    "(sem leitura: %v)",
		"err_wg_key_required":           "defina WG_PRIVATE_KEY no ambiente",
		"err_docker":                    "docker: %v",
		"err_network":                   "rede: %v",
		"err_build":                     "build: %v",
		"err_catalog":                   "catálogo de servidores: %v",
		"err_catalog_too_small":         "catálogo tem %d servidores, preciso de %d (GATEWAY_COUNT + SPARE_COUNT)",
		"err_initial_boot":              "subida inicial: %v",
		"log_bringing_up_gateway":       "subindo %s com %s",
		"log_waiting_tunnels":           "esperando os túneis subirem...",
		"log_no_ip_yet":                 "sem IP ainda para %s: %v",
		"log_exit_ip_lookup_failed":     "não consegui descobrir o IP de saída de %s: %v",
		"log_health_check_failed":       "%s (%s) falhou a verificação (%d): %v",
		"log_sick_no_spare":             "%s doente, mas sem reserva saudável; reiniciando o container",
		"log_restart_failed":            "reinício de %s falhou: %v",
		"log_promoting":                 "promovendo %s no lugar de %s",
		"log_balancer_update_failed":    "atualizar balanceador: %v",
		"log_no_free_server":            "sem servidor livre para %s agora: %v",
		"log_running_check_failed":      "não consegui checar se %s está rodando: %v",
		"log_not_running_restarting":    "%s não está rodando (provavelmente caiu); reiniciando antes de trocar o servidor",
		"log_not_running_in_time":       "%s não voltou a rodar a tempo",
		"log_switch_server_failed":      "switch-server em %s falhou: %v",
		"log_now_points_to":             "%s agora aponta para %s",
		"log_new_exit_ip_lookup_failed": "não consegui descobrir o novo IP de saída de %s: %v",
		"log_rotation_already_running":  "rotação completa já em andamento, ignorando novo pedido",
		"log_rotation_start":            "rotação completa: trocando o servidor dos %d gateways",
		"log_rotation_done":             "rotação completa concluída",
		"log_status_server_error":       "status do controlador: %v",
		"log_status_server_listening":   "status do controlador em :%s/status",
		"log_invalid_env_int":           "valor inválido em %s=%q, usando %d",
		"log_invalid_env_bool":          "valor inválido em %s=%q, usando %v",
	},
	idiomaES: {
		"panel_resources":               " recursos (host) ",
		"panel_active":                  " activos ",
		"panel_spare":                   " reserva ",
		"panel_connections":             " conexiones recientes ",
		"panel_events":                  " eventos ",
		"panel_shortcuts":               " atajos ",
		"shortcuts_text":                "[yellow]q[white]  salir\n[yellow]r[white]  renovar todos los túneles\n[yellow]p[white]  pausar/reanudar la pantalla\n[yellow]c[white]  limpiar eventos\n[yellow]l[white]  cambiar idioma",
		"col_gateway":                   "GATEWAY",
		"col_server":                    "SERVIDOR",
		"col_internal_ip":               "IP INTERNO",
		"col_exit_ip":                   "IP DE SALIDA",
		"col_last_switch":               "ÚLT. CAMBIO",
		"col_failures":                  "FALLOS",
		"col_state":                     "ESTADO",
		"col_time":                      "HORA",
		"col_source":                    "ORIGEN",
		"col_destination":               "DESTINO",
		"state_healthy":                 "sano",
		"state_repairing":               "reparando",
		"state_failing":                 "fallando",
		"time_ago":                      "hace %s",
		"summary_line":                  "activos sanos: [green]%d[white]/%d    reserva sana: [green]%d[white]/%d    activo hace %s%s",
		"paused_suffix":                 "    [yellow]PAUSADO (pulsa p para reanudar)[white]",
		"no_reading":                    "(sin lectura: %v)",
		"err_wg_key_required":           "define WG_PRIVATE_KEY en el entorno",
		"err_docker":                    "docker: %v",
		"err_network":                   "red: %v",
		"err_build":                     "build: %v",
		"err_catalog":                   "catálogo de servidores: %v",
		"err_catalog_too_small":         "el catálogo tiene %d servidores, se necesitan %d (GATEWAY_COUNT + SPARE_COUNT)",
		"err_initial_boot":              "arranque inicial: %v",
		"log_bringing_up_gateway":       "levantando %s con %s",
		"log_waiting_tunnels":           "esperando a que los túneles suban...",
		"log_no_ip_yet":                 "todavía sin IP para %s: %v",
		"log_exit_ip_lookup_failed":     "no pude averiguar la IP de salida de %s: %v",
		"log_health_check_failed":       "%s (%s) falló la verificación (%d): %v",
		"log_sick_no_spare":             "%s está mal, pero no hay reserva sana disponible; reiniciando el contenedor",
		"log_restart_failed":            "el reinicio de %s falló: %v",
		"log_promoting":                 "promoviendo %s en lugar de %s",
		"log_balancer_update_failed":    "actualizar el balanceador: %v",
		"log_no_free_server":            "sin servidor libre para %s ahora mismo: %v",
		"log_running_check_failed":      "no pude comprobar si %s está corriendo: %v",
		"log_not_running_restarting":    "%s no está corriendo (probablemente se cayó); reiniciando antes de cambiar de servidor",
		"log_not_running_in_time":       "%s no volvió a tiempo",
		"log_switch_server_failed":      "switch-server en %s falló: %v",
		"log_now_points_to":             "%s ahora apunta a %s",
		"log_new_exit_ip_lookup_failed": "no pude averiguar la nueva IP de salida de %s: %v",
		"log_rotation_already_running":  "ya hay una rotación completa en curso, ignorando el nuevo pedido",
		"log_rotation_start":            "rotación completa: cambiando el servidor de %d gateways",
		"log_rotation_done":             "rotación completa terminada",
		"log_status_server_error":       "estado del controlador: %v",
		"log_status_server_listening":   "estado del controlador en :%s/status",
		"log_invalid_env_int":           "valor inválido en %s=%q, usando %d",
		"log_invalid_env_bool":          "valor inválido en %s=%q, usando %v",
	},
}
