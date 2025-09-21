// jogo.go - Funções para manipular os elementos do jogo, como carregar o mapa e mover o personagem
package main

import (
	"bufio"
	"fmt"
	"math"
	"math/rand"
	"os"
	"time"
)

// estado inimigo individual
type InimigoStruct struct {
	x, y           int
	stop           chan bool
	UltimoVisitado Elemento
	estado         string
	chBotao        chan bool
	chMoeda        chan bool
}

// estado moeda individual
type MoedaStruct struct {
	x, y int
	stop chan bool
}

// estado botao individual
type BotaoStruct struct {
	x, y   int
	ligado bool
}

// atualização do mapa
type MapaUpdate struct {
	tipo           string
	fx, fy, tx, ty int
	UltimoVisitado Elemento
}

// Elemento representa qualquer objeto do mapa (parede, personagem, vegetação, etc)
type Elemento struct {
	simbolo  rune
	cor      Cor
	corFundo Cor
	tangivel bool // Indica se o elemento bloqueia passagem
}

// Jogo contém o estado atual do jogo
type Jogo struct {
	Mapa           [][]Elemento    // grade 2D representando o mapa
	PosX, PosY     int             // posição atual do personagem
	UltimoVisitado Elemento        // elemento que estava na posição do personagem antes de mover
	StatusMsg      string          // mensagem para a barra de status
	NumMoedas      int             // num de moedas coletadas
	encerrar       bool            // dita quando encerrar
	chMapa         chan MapaUpdate // controla o mapa
	botoes         []*BotaoStruct
	chInimigoBotao []chan bool
	chInimigoMoeda []chan bool
}

// Elementos visuais do jogo
var (
	Personagem     = Elemento{'☺', CorCinzaEscuro, CorPadrao, false}
	Parede         = Elemento{'▤', CorParede, CorFundoParede, true}
	Vegetacao      = Elemento{'♣', CorVerde, CorPadrao, false}
	Vazio          = Elemento{' ', CorPadrao, CorPadrao, false}
	Inimigo        = Elemento{'☠', CorVermelho, CorPadrao, false}
	Moeda          = Elemento{'$', CorAmarelo, CorPadrao, false}
	BotaoLigado    = Elemento{'●', CorMagenta, CorPadrao, false}
	BotaoDesligado = Elemento{'○', CorVermelho, CorPadrao, false}
	moedas         []*MoedaStruct
	Inimigos       []*InimigoStruct
)

func (j *Jogo) Run() {
	interfaceDesenharJogo(j)

	for {
		update := <-j.chMapa
		switch update.tipo {
		case "Personagem":
			jogoMoverPersonagem(j, update.fx, update.fy, update.tx-update.fx, update.ty-update.fy)
			interfaceDesenharJogo(j)
		case "Moeda":
			j.Mapa[update.fy][update.fx] = Vazio
			j.Mapa[update.ty][update.tx] = Moeda
			interfaceDesenharJogo(j)

		case "Botao":
			for _, botao := range j.botoes {
				if botao.ligado {
					j.Mapa[botao.y][botao.x] = BotaoLigado
				} else {
					j.Mapa[botao.y][botao.x] = BotaoDesligado
				}
			}
			interfaceDesenharJogo(j)
		case "Inimigo":
			// Restaura e move
			if update.UltimoVisitado == BotaoDesligado || update.UltimoVisitado == BotaoLigado {
				if j.botoes[0].ligado {
					update.UltimoVisitado = BotaoLigado
				} else {
					update.UltimoVisitado = BotaoDesligado
				}
			}
			j.Mapa[update.fy][update.fx] = update.UltimoVisitado
			j.Mapa[update.ty][update.tx] = Inimigo
			interfaceDesenharJogo(j)
		case "Perdeu":
			interfacePerdeu(j) // para evitar deadlock
		}
	}
}

// Cria e retorna uma nova instância do jogo
func jogoNovo() Jogo {
	// O ultimo elemento visitado é inicializado como vazio
	// pois o jogo começa com o personagem em uma posição vazia
	return Jogo{
		UltimoVisitado: Vazio,
		chMapa:         make(chan MapaUpdate),
		encerrar:       false,
		NumMoedas:      0,
	}
}

// Lê um arquivo texto linha por linha e constrói o mapa do jogo
func jogoCarregarMapa(nome string, jogo *Jogo) error {
	arq, err := os.Open(nome)
	if err != nil {
		return err
	}
	defer arq.Close()

	scanner := bufio.NewScanner(arq)
	y := 0
	for scanner.Scan() {
		linha := scanner.Text()
		var linhaElems []Elemento
		for x, ch := range linha {
			e := Vazio
			switch ch {
			case Parede.simbolo:
				e = Parede
			case Inimigo.simbolo:
				e = Inimigo
			case Vegetacao.simbolo:
				e = Vegetacao
			case Moeda.simbolo:
				e = Moeda
			case Personagem.simbolo:
				jogo.PosX, jogo.PosY = x, y // registra a posição inicial do personagem
			case BotaoDesligado.simbolo:
				e = BotaoDesligado
			}

			linhaElems = append(linhaElems, e)
		}
		jogo.Mapa = append(jogo.Mapa, linhaElems)
		y++
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	return nil
}

// Verifica se o personagem pode se mover para a posição (x, y)
func jogoPodeMoverPara(jogo *Jogo, x, y int) bool {
	// Verifica se a coordenada Y está dentro dos limites verticais do mapa
	if y < 0 || y >= len(jogo.Mapa) {
		return false
	}

	// Verifica se a coordenada X está dentro dos limites horizontais do mapa
	if x < 0 || x >= len(jogo.Mapa[y]) {
		return false
	}

	// Verifica se o elemento de destino é tangível (bloqueia passagem)
	if jogo.Mapa[y][x].tangivel {
		return false
	}

	// Pode mover para a posição
	return true
}

// Move um elemento para a nova posição
func jogoMoverPersonagem(jogo *Jogo, x, y, dx, dy int) {
	nx, ny := x+dx, y+dy

	for _, botao := range jogo.botoes {
		if nx == botao.x && ny == botao.y {
			if botao.ligado {
				for _, ch := range jogo.chInimigoBotao {
					select {
					case ch <- true:
					default:
					}
				}
			}
			break
		}
	}

	if jogo.Mapa[ny][nx].simbolo == Moeda.simbolo {
		for i, m := range moedas {
			if m.x == nx && m.y == ny {
				m.stop <- true
				moedas = append(moedas[:i], moedas[i+1:]...)
				jogo.Mapa[m.y][m.x] = Vazio
				jogo.NumMoedas++
				jogo.StatusMsg = fmt.Sprintf("Moedas coletadas: %d/5", jogo.NumMoedas)

				for _, ch := range jogo.chInimigoMoeda {
					select {
					case ch <- true:
					default:
						// nao bloquear
					}
				}

				if jogo.NumMoedas == 5 {
					interfaceGanhou(jogo)
				}
				break
			}
		}
	}

	if jogo.Mapa[ny][nx].simbolo == Inimigo.simbolo {
		interfacePerdeu(jogo)
	}

	elemento := jogo.Mapa[y][x] // guarda o conteúdo atual da posição
	if jogo.UltimoVisitado == BotaoDesligado || jogo.UltimoVisitado == BotaoLigado {
		if jogo.botoes[0].ligado {
			jogo.UltimoVisitado = BotaoLigado
		} else {
			jogo.UltimoVisitado = BotaoDesligado
		}
	}
	if jogo.UltimoVisitado == Moeda { // se chegar a moeda no mesmo momento que ela mudar de local
		num := 0
		for _, m := range moedas {
			if m.x == x && m.y == y {
				break
			}
			num++
			if num == len(moedas) {
				jogo.UltimoVisitado = Vazio
			}
		}
	}
	jogo.Mapa[y][x] = jogo.UltimoVisitado   // restaura o conteúdo anterior
	jogo.UltimoVisitado = jogo.Mapa[ny][nx] // guarda o conteúdo atual da nova posição
	jogo.Mapa[ny][nx] = elemento            // move o elemento
	jogo.PosX, jogo.PosY = nx, ny
}

// ve botoes do mapa
func BotaoController(jogo *Jogo) {
	for y, linha := range jogo.Mapa {
		for x, elem := range linha {
			if elem.simbolo == BotaoDesligado.simbolo {
				b := &BotaoStruct{x: x, y: y, ligado: false}
				jogo.botoes = append(jogo.botoes, b)
			}
		}
	}

	// Inicia o controle automático dos botões
	if len(jogo.botoes) > 0 {
		go ServiceBotoes(jogo)
	}
}

// logica botoes
func ServiceBotoes(jogo *Jogo) {
	for {
		time.Sleep(15 * time.Second)
		for _, botao := range jogo.botoes {
			botao.ligado = !botao.ligado
		}

		jogo.chMapa <- MapaUpdate{
			tipo: "Botao",
		}

	}
}

// ve moeda
func MoedaController(jogo *Jogo) {
	for y, linha := range jogo.Mapa {
		for x, elem := range linha {
			if elem.simbolo == '$' {
				m := &MoedaStruct{x: x, y: y, stop: make(chan bool)}
				moedas = append(moedas, m)
				go ServiceMoeda(jogo, m)
			}
		}
	}
}

// logica moeda
func ServiceMoeda(jogo *Jogo, m *MoedaStruct) {
	for {
		select {
		case <-m.stop:
			return
		case <-time.After((time.Duration(rand.Intn(10) + 5)) * time.Second):
			num1 := rand.Intn(len(jogo.Mapa))
			num2 := rand.Intn(len(jogo.Mapa[num1]))
			if jogo.Mapa[num1][num2].simbolo == ' ' {
				jogo.chMapa <- MapaUpdate{
					tipo: "Moeda",
					fx:   m.x, fy: m.y,
					tx: num2, ty: num1,
				}
				m.x, m.y = num2, num1
			}
		}
	}
}

// ve inimigos
func InimigoController(jogo *Jogo) {
	for y, linha := range jogo.Mapa {
		for x, elem := range linha {
			if elem.simbolo == '☠' {
				i := &InimigoStruct{
					x: x, y: y,
					stop:           make(chan bool),
					UltimoVisitado: Vazio,
					estado:         "Patrulha",
					chBotao:        make(chan bool, 1), // evitar bloqueio
					chMoeda:        make(chan bool, 1), // evitar bloqueio
				}
				Inimigos = append(Inimigos, i)
				jogo.chInimigoBotao = append(jogo.chInimigoBotao, i.chBotao)
				jogo.chInimigoMoeda = append(jogo.chInimigoMoeda, i.chMoeda)
				go ServiceInimigo(jogo, i)
			}
		}
	}
}

// logica
func ServiceInimigo(jogo *Jogo, i *InimigoStruct) {
	for {
		select {
		case botaoEstado := <-i.chBotao:
			if botaoEstado {
				i.estado = "Patrulha"
				jogo.StatusMsg = "Inimigos pararam de perseguir devido ao botao!"
			}
		case <-i.chMoeda:
			i.estado = "Perseguicao"
			jogo.StatusMsg = "Moeda coletada! Inimigos iniciaram perseguicao!"

		case <-time.After(500 * time.Millisecond):
			var nx, ny int

			estadoAtual := i.estado

			if estadoAtual == "Perseguicao" {
				// Persegue o jogador
				dx := 0
				if i.x < jogo.PosX {
					dx = 1
				} else if i.x > jogo.PosX {
					dx = -1
				}

				dy := 0
				if i.y < jogo.PosY {
					dy = 1
				} else if i.y > jogo.PosY {
					dy = -1
				}

				// Verifica se o movimento n é ilegal
				if math.Abs(float64(i.x-jogo.PosX)) > math.Abs(float64(i.y-jogo.PosY)) {
					if jogoPodeMoverPara(jogo, i.x+dx, i.y) {
						nx, ny = i.x+dx, i.y
					} else if jogoPodeMoverPara(jogo, i.x, i.y+dy) {
						nx, ny = i.x, i.y+dy
					} else {
						nx, ny = i.x, i.y
					}
				} else {
					if jogoPodeMoverPara(jogo, i.x, i.y+dy) {
						nx, ny = i.x, i.y+dy
					} else if jogoPodeMoverPara(jogo, i.x+dx, i.y) {
						nx, ny = i.x+dx, i.y
					} else {
						nx, ny = i.x, i.y
					}
				}

			} else {
				// aleatorio
				for {
					n := rand.Intn(4)
					nx, ny = i.x, i.y
					switch n {
					case 0:
						nx++
					case 1:
						nx--
					case 2:
						ny++
					case 3:
						ny--
					}
					if jogoPodeMoverPara(jogo, nx, ny) {
						break
					}
				}
			}

			if nx != i.x || ny != i.y {
				// colisao
				if jogo.PosX == nx && jogo.PosY == ny {
					jogo.chMapa <- MapaUpdate{
						tipo: "Perdeu",
					}
				}

				i.UltimoVisitado = jogo.Mapa[ny][nx]
				// Envia update normal
				jogo.chMapa <- MapaUpdate{
					tipo: "Inimigo",
					fx:   i.x, fy: i.y,
					tx: nx, ty: ny,
					UltimoVisitado: i.UltimoVisitado,
				}

				// Atualiza estado
				i.x, i.y = nx, ny
			}
		}
	}
}
