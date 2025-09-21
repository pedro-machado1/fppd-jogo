// main.go - Loop principal do jogo
package main

import "os"

func main() {
	// Inicializa a interface (termbox)
	interfaceIniciar()
	defer interfaceFinalizar()

	// Usa "mapa.txt" como arquivo padrão ou lê o primeiro argumento
	mapaFile := "mapa.txt"
	if len(os.Args) > 1 {
		mapaFile = os.Args[1]
	}

	// Inicializa o jogo
	jogo := jogoNovo()
	if err := jogoCarregarMapa(mapaFile, &jogo); err != nil {
		panic(err)
	}

	// moeda
	// Desenha o estado inicial do jogo

	go jogo.Run()

	go MoedaController(&jogo)
	go InimigoController(&jogo)
	go BotaoController(&jogo)

	// Loop principal de entrada
	for !jogo.encerrar {
		evento := interfaceLerEventoTeclado()
		if evento.Tipo == "sair" {
			jogo.encerrar = true
			break
		}
		if continuar := personagemExecutarAcao(evento, &jogo); !continuar {
			break
		}
	}

	interfaceLimparTela()
}
