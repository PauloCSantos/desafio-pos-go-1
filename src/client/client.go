package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"
)

type Cotacao struct {
	Bid float64 `json:"bid"`
}

func main() {
	// Define as configurações
	baseURL := "http://localhost:8080"
	cotacaoPath := "cotacao.txt"
	timeout := 300 * time.Millisecond

	// Cria o contexto com timeout
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// Faz a requisição para a cotação
	req, err := http.NewRequestWithContext(ctx, "GET", baseURL+"/cotacao", nil)
	if err != nil {
		log.Fatal(err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()

	// Lê o corpo da resposta
	cotacaoJSON, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatal(err)
	}

	// Decodifica o JSON para a estrutura Cotacao
	var cotacao Cotacao
	err = json.Unmarshal(cotacaoJSON, &cotacao)
	if err != nil {
		log.Fatal(err)
	}

	// Salva o valor "bid" no arquivo
	err = saveToFile(cotacaoPath, fmt.Sprintf("Dólar: %.2f", cotacao.Bid))
	if err != nil {
		log.Fatal(err)
	}

	// Imprime o valor "bid" no console
	fmt.Printf("Dólar: %.2f\n", cotacao.Bid)
}

func saveToFile(path string, content string) error {
	// Verifica se o arquivo existe
	_, err := os.Stat(path)

	// Cria o arquivo se não existir
	if os.IsNotExist(err) {
		f, err := os.Create(path)
		if err != nil {
			return err
		}
		defer f.Close()
	} else if err != nil {
		// Outro erro além de "não existe"
		return err
	}

	// Abre o arquivo para escrita
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	// Escreve a cotação no arquivo
	_, err = f.WriteString(content + "\n")
	if err != nil {
		return err
	}

	return nil
}
